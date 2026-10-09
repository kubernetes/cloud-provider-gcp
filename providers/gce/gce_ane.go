//go:build !providerless
// +build !providerless

/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// TODO(upstream): Migrate from raw REST HTTP client to the official generated
// GCE Go compute client (google.golang.org/api/compute/v1.preview and v1).
// Tracking issue: https://github.com/kubernetes/cloud-provider-gcp/issues/1383

// State and configuration constants for AliasNetworkEndpoint lifecycle.
const (
	// ANEStateCreating indicates the endpoint is being created and is not
	// yet ready to process traffic.
	ANEStateCreating = "CREATING"

	// ANEStateActive indicates the endpoint is active and programmed in the
	// dataplane to process traffic.
	ANEStateActive = "ACTIVE"

	// ANEStateDeleting indicates the endpoint is being deleted.
	ANEStateDeleting = "DELETING"

	// DefaultANEAliasName is the client-chosen RFC 1035 map key used in
	// AliasNetworkEndpoint.Aliases when creating a Pod IP endpoint. The GCE
	// API requires a user-provided RFC 1035 identifier as the map key and
	// currently limits each endpoint to at most one entry (max_length: 1).
	//
	// WARNING: Do not change this value. Modifying this constant will
	// prevent the controller from recognizing existing ANE aliases across
	// upgrades, causing NodeNetworkConfig drift and Pod IP disruption.
	DefaultANEAliasName = "ccm-adaptive-ipam"

	// DefaultANEDescription is the description set on AliasNetworkEndpoint
	// resources created by the dynamic Pod IP controller.
	DefaultANEDescription = "Managed by cloud-controller-manager for GKE Adaptive Cluster IPAM"

	// DefaultANEIPVersion is the IPv4 ipVersion value for
	// ANEAlias.IPVersion.
	DefaultANEIPVersion = "IPV4"

	// ANESecurityTagInheritanceInherited configures the endpoint to inherit
	// security tags directly from its host VM instance so firewall rules
	// targeting the host instance's tags apply to this endpoint.
	ANESecurityTagInheritanceInherited = "INHERITED"

	// DefaultANEOperationTimeout is the maximum time to wait for a zonal
	// ANE operation to reach DONE. Healthy operations complete in 6-10s. If
	// an operation stalls (e.g. during CLH rollback), we fail fast after
	// 20s to allow the controller workqueue to self-heal.
	DefaultANEOperationTimeout = 20 * time.Second

	anePreviewAPIVersionHeader    = "X-Goog-Api-Version"
	anePreviewAPIVersion          = "2026-10-01-preview"
	defaultANEMaxTransientRetries = 3
	defaultANEInitialRetryBackoff = 250 * time.Millisecond
)

// ANEResourceMetadata contains metadata about the API resource, including the
// resolved API version (AIP-185).
type ANEResourceMetadata struct {
	// APIVersion is the output-only API version resolved by the server for
	// this resource (e.g. "2026-10-01-preview").
	APIVersion string `json:"apiVersion,omitempty"`
}

// AliasNetworkEndpoint represents a GCE zonal AliasNetworkEndpoint resource
// (`projects/{project}/zones/{zone}/aliasNetworkEndpoints/{name}`), used for
// GKE Pod-native IP endpoints in Andromeda.
type AliasNetworkEndpoint struct {
	// Kind is the output-only type of the resource
	// ("compute#aliasNetworkEndpoint").
	Kind string `json:"kind,omitempty"`

	// Id is the output-only unique numeric identifier for the resource,
	// defined by the server.
	Id uint64 `json:"id,string,omitempty"`

	// Name is the client-provided resource name (1-63 characters, RFC 1035
	// compliant).
	Name string `json:"name,omitempty"`

	// Description is an optional user-provided description of this
	// resource.
	Description string `json:"description,omitempty"`

	// Subnetwork is the URL of the subnetwork to which this alias network
	// endpoint belongs. Required on creation.
	Subnetwork string `json:"subnetwork,omitempty"`

	// Host specifies the GCE VM instance to which this alias network
	// endpoint is bound. Required on creation.
	Host *ANEHost `json:"host,omitempty"`

	// Aliases is the required map of IP aliases allocated for this
	// endpoint, keyed by a client-provided RFC 1035 alias name (such as
	// DefaultANEAliasName). The GCE API currently restricts this map to at
	// most 1 entry per endpoint.
	Aliases map[string]*ANEAlias `json:"aliases,omitempty"`

	// SecurityTagInheritance controls whether security tags are inherited
	// from the host VM instance (e.g. ANESecurityTagInheritanceInherited or
	// "NOT_INHERITED").
	SecurityTagInheritance string `json:"securityTagInheritance,omitempty"`

	// Status is the output-only current lifecycle status of the endpoint.
	Status *ANEStatus `json:"status,omitempty"`

	// ResourceMetadata contains output-only API version metadata (AIP-185).
	ResourceMetadata *ANEResourceMetadata `json:"resourceMetadata,omitempty"`

	// CreationTimestamp is the output-only creation timestamp in RFC 3339
	// text format.
	CreationTimestamp string `json:"creationTimestamp,omitempty"`

	// SelfLink is the output-only server-defined URL for this resource.
	SelfLink string `json:"selfLink,omitempty"`
}

// ANEHost represents the binding of an AliasNetworkEndpoint to a GCE VM
// instance.
type ANEHost struct {
	// Instance is the required URL of the GCE VM instance that this alias
	// network endpoint is bound to
	// (e.g. "projects/{project}/zones/{zone}/instances/{instance}").
	Instance string `json:"instance,omitempty"`
}

// ANEAlias represents an IP alias configuration and its allocated IP within
// AliasNetworkEndpoint.Aliases.
type ANEAlias struct {
	// IPAddress is an optional input-only IP address to request for the
	// endpoint. If omitted on creation, GCE automatically allocates an IP
	// and populates EffectiveIPAddress.
	IPAddress string `json:"ipAddress,omitempty"`

	// IPVersion is the required IP version of the alias IP ("IPV4" or
	// "IPV6").
	IPVersion string `json:"ipVersion,omitempty"`

	// SubnetworkRangeNames is an optional input-only list of candidate
	// secondary range names in the subnetwork from which to allocate the
	// alias IP. If empty, the IP is allocated from the subnetwork's primary
	// range.
	SubnetworkRangeNames []string `json:"subnetworkRangeNames,omitempty"`

	// EffectiveIPAddress is the output-only actual IP address allocated by
	// GCE for this alias.
	EffectiveIPAddress string `json:"effectiveIpAddress,omitempty"`

	// EffectiveSubnetworkRangeName is the output-only secondary subnetwork
	// range name from which the IP address was allocated.
	EffectiveSubnetworkRangeName string `json:"effectiveSubnetworkRangeName,omitempty"`
}

// ANEStatus represents the output-only lifecycle status of an
// AliasNetworkEndpoint.
type ANEStatus struct {
	// State is the output-only lifecycle state of the endpoint (e.g.
	// ANEStateCreating, ANEStateActive, ANEStateDeleting).
	State string `json:"state,omitempty"`
}

type aneListResponse struct {
	Kind          string                  `json:"kind,omitempty"`
	Items         []*AliasNetworkEndpoint `json:"items,omitempty"`
	NextPageToken string                  `json:"nextPageToken,omitempty"`
}

type aneErrorInfo struct {
	Type     string            `json:"@type,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	Domain   string            `json:"domain,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type aneOperationError struct {
	Errors []struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"errors,omitempty"`
	Details []aneErrorInfo `json:"details,omitempty"`
}

type aneOperation struct {
	Kind                string             `json:"kind,omitempty"`
	Id                  uint64             `json:"id,string,omitempty"`
	Name                string             `json:"name,omitempty"`
	Zone                string             `json:"zone,omitempty"`
	OperationType       string             `json:"operationType,omitempty"`
	Status              string             `json:"status,omitempty"`
	TargetLink          string             `json:"targetLink,omitempty"`
	Error               *aneOperationError `json:"error,omitempty"`
	HttpErrorMessage    string             `json:"httpErrorMessage,omitempty"`
	HttpErrorStatusCode int                `json:"httpErrorStatusCode,omitempty"`
}

type aneAPIErrorResponse struct {
	Error struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Errors  []struct {
			Code    string `json:"code,omitempty"`
			Message string `json:"message,omitempty"`
			Reason  string `json:"reason,omitempty"`
		} `json:"errors,omitempty"`
		Details []aneErrorInfo `json:"details,omitempty"`
	} `json:"error,omitempty"`
}

func newANEMetricContext(request, zone string) *metricContext {
	return newGenericMetricContext("ane", request, unusedMetricLabel, zone, anePreviewAPIVersion)
}

type aneServiceManager interface {
	CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) error
	DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error
	GetAliasNetworkEndpoint(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error)
	ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error)
}

func (g *Cloud) getANEService() aneServiceManager {
	if g.aneService != nil {
		return g.aneService
	}
	return &gceANEService{gce: g}
}

func parseInstanceProviderID(providerID string) (project, zone, name, instanceURL string, err error) {
	project, zone, name, err = splitProviderID(providerID)
	if err != nil {
		return "", "", "", "", err
	}
	name = canonicalizeInstanceName(name)
	instanceURL = fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, name)
	return project, zone, name, instanceURL, nil
}

// CreateAliasNetworkEndpoint creates an AliasNetworkEndpoint for the instance
// identified by providerID and waits for the operation to complete.
func (g *Cloud) CreateAliasNetworkEndpoint(ctx context.Context, providerID string, endpoint *AliasNetworkEndpoint) error {
	project, zone, _, instanceURL, err := parseInstanceProviderID(providerID)
	if err != nil {
		return err
	}
	if endpoint != nil && endpoint.Host == nil {
		endpoint.Host = &ANEHost{Instance: instanceURL}
	}
	mc := newANEMetricContext("create", zone)
	return mc.Observe(g.getANEService().CreateAliasNetworkEndpoint(ctx, project, zone, endpoint))
}

// DeleteAliasNetworkEndpoint deletes the specified AliasNetworkEndpoint for the
// instance identified by providerID and waits for the operation to complete.
func (g *Cloud) DeleteAliasNetworkEndpoint(ctx context.Context, providerID, name string) error {
	project, zone, _, _, err := parseInstanceProviderID(providerID)
	if err != nil {
		return err
	}
	mc := newANEMetricContext("delete", zone)
	return mc.Observe(g.getANEService().DeleteAliasNetworkEndpoint(ctx, project, zone, name))
}

// GetAliasNetworkEndpoint retrieves an AliasNetworkEndpoint by name in the
// project and zone of the instance identified by providerID.
func (g *Cloud) GetAliasNetworkEndpoint(ctx context.Context, providerID, name string) (*AliasNetworkEndpoint, error) {
	project, zone, _, _, err := parseInstanceProviderID(providerID)
	if err != nil {
		return nil, err
	}
	mc := newANEMetricContext("get", zone)
	ep, err := g.getANEService().GetAliasNetworkEndpoint(ctx, project, zone, name)
	return ep, mc.Observe(err)
}

// ListAliasNetworkEndpoints lists all AliasNetworkEndpoints bound to the
// instance identified by providerID.
func (g *Cloud) ListAliasNetworkEndpoints(ctx context.Context, providerID string) ([]*AliasNetworkEndpoint, error) {
	project, zone, _, instanceURL, err := parseInstanceProviderID(providerID)
	if err != nil {
		return nil, err
	}
	mc := newANEMetricContext("list", zone)
	eps, err := g.getANEService().ListAliasNetworkEndpoints(ctx, project, zone, instanceURL)
	return eps, mc.Observe(err)
}

type gceANEService struct {
	gce    *Cloud
	waiter func(ctx context.Context, project, zone, opName string) error
}

var _ aneServiceManager = (*gceANEService)(nil)

func (s *gceANEService) httpClient() *http.Client {
	if s.gce != nil && s.gce.httpClient != nil {
		return s.gce.httpClient
	}
	return http.DefaultClient
}

func (s *gceANEService) projectsBasePath() string {
	base := ""
	if s.gce != nil {
		base = s.gce.projectsBasePath
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

func (s *gceANEService) acceptRateLimit() {
	if s.gce != nil && s.gce.operationPollRateLimiter != nil {
		s.gce.operationPollRateLimiter.Accept()
	}
}

func (s *gceANEService) buildURL(path string, queryParams ...string) string {
	versionParam := "%24apiVersion=" + anePreviewAPIVersion
	allParams := append([]string{versionParam}, queryParams...)
	return fmt.Sprintf("%s%s?%s", s.projectsBasePath(), strings.TrimPrefix(path, "/"), strings.Join(allParams, "&"))
}

func isTransientHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout ||
		statusCode == http.StatusTooManyRequests
}

func parseANERESTError(statusCode int, body []byte) error {
	var apiErr aneAPIErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && (apiErr.Error.Message != "" || len(apiErr.Error.Details) > 0) {
		var details []string
		if apiErr.Error.Message != "" {
			details = append(details, apiErr.Error.Message)
		}
		for _, d := range apiErr.Error.Details {
			if d.Reason != "" {
				details = append(details, fmt.Sprintf("[reason=%s domain=%s]", d.Reason, d.Domain))
			}
		}
		return fmt.Errorf("GCE ANE request returned HTTP %d: %s", statusCode, strings.Join(details, " "))
	}
	return fmt.Errorf("GCE ANE request returned HTTP %d: %s", statusCode, string(body))
}

func isUnallowlistedOrMethodNotFound(body []byte, bodyStr string) bool {
	if strings.Contains(bodyStr, "Method not found") || strings.Contains(bodyStr, "METHOD_NOT_FOUND") {
		return true
	}
	var apiErr aneAPIErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil {
		for _, d := range apiErr.Error.Details {
			if d.Reason == "METHOD_NOT_FOUND" || d.Reason == "API_VERSION_NOT_SUPPORTED" {
				return true
			}
		}
	}
	return false
}

func (s *gceANEService) doWithRetry(ctx context.Context, newReq func() (*http.Request, error)) (*http.Response, error) {
	var resp *http.Response
	var lastErr error
	backoff := wait.Backoff{
		Duration: defaultANEInitialRetryBackoff,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    defaultANEMaxTransientRetries,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		s.acceptRateLimit()

		req, err := newReq()
		if err != nil {
			return false, err
		}

		resp, lastErr = s.httpClient().Do(req)
		if lastErr != nil {
			return false, nil
		}

		if isTransientHTTPStatus(resp.StatusCode) {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					select {
					case <-ctx.Done():
						resp.Body.Close()
						return false, ctx.Err()
					case <-time.After(time.Duration(seconds) * time.Second):
					}
				}
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = parseANERESTError(resp.StatusCode, body)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}
	return resp, nil
}

func (s *gceANEService) CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) error {
	url := s.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone))
	data, err := json.Marshal(endpoint)
	if err != nil {
		return fmt.Errorf("failed to marshal ANE payload: %w", err)
	}

	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create insert request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(anePreviewAPIVersionHeader, anePreviewAPIVersion)
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("GCE ANE insert request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return parseANERESTError(resp.StatusCode, body)
	}

	var op aneOperation
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return fmt.Errorf("failed to decode insert operation: %w", err)
	}
	if op.Name != "" {
		if err := s.waitForOperation(ctx, project, zone, op.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *gceANEService) DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error {
	url := s.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints/%s", project, zone, name))
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create delete request: %w", err)
		}
		req.Header.Set(anePreviewAPIVersionHeader, anePreviewAPIVersion)
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("GCE ANE delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if isUnallowlistedOrMethodNotFound(body, bodyStr) {
			return fmt.Errorf("GCE ANE preview API endpoint not found or unallowlisted (HTTP 404): %s", bodyStr)
		}
		// Idempotent deletion: resource already deleted or not found.
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return parseANERESTError(resp.StatusCode, body)
	}

	var op aneOperation
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return fmt.Errorf("failed to decode delete operation: %w", err)
	}
	if op.Name != "" {
		if err := s.waitForOperation(ctx, project, zone, op.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *gceANEService) GetAliasNetworkEndpoint(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error) {
	url := s.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints/%s", project, zone, name))
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create get request: %w", err)
		}
		req.Header.Set(anePreviewAPIVersionHeader, anePreviewAPIVersion)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("GCE ANE get request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseANERESTError(resp.StatusCode, body)
	}

	ane := &AliasNetworkEndpoint{}
	if err := json.NewDecoder(resp.Body).Decode(ane); err != nil {
		return nil, fmt.Errorf("failed to decode ANE get response: %w", err)
	}
	return ane, nil
}

func (s *gceANEService) ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error) {
	var result []*AliasNetworkEndpoint
	pageToken := ""
	for {
		var url string
		if pageToken != "" {
			url = s.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone), "pageToken="+pageToken)
		} else {
			url = s.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone))
		}

		resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create list request: %w", err)
			}
			req.Header.Set(anePreviewAPIVersionHeader, anePreviewAPIVersion)
			return req, nil
		})
		if err != nil {
			return nil, fmt.Errorf("GCE ANE list request failed: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, parseANERESTError(resp.StatusCode, body)
		}

		var listResp aneListResponse
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to decode ANE list response: %w", err)
		}

		for _, item := range listResp.Items {
			if item != nil && item.Host != nil && EqualResourceURLs(item.Host.Instance, instanceURL) {
				result = append(result, item)
			}
		}

		if listResp.NextPageToken == "" {
			break
		}
		pageToken = listResp.NextPageToken
	}
	return result, nil
}

func (s *gceANEService) waitForOperation(ctx context.Context, project, zone, opName string) error {
	waitCtx, cancel := context.WithTimeout(ctx, DefaultANEOperationTimeout)
	defer cancel()

	waitFn := s.waiter
	if waitFn == nil {
		waitFn = s.waitForZoneOperation
	}
	err := waitFn(waitCtx, project, zone, opName)
	if err != nil {
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("operation %q exceeded %v timeout: %w", opName, DefaultANEOperationTimeout, err)
		}
		return err
	}
	return nil
}

// waitForZoneOperation polls the GCE zonal operations.wait endpoint until the
// operation reaches DONE or the context expires.
func (s *gceANEService) waitForZoneOperation(ctx context.Context, project, zone, opName string) error {
	url := s.buildURL(fmt.Sprintf("%s/zones/%s/operations/%s/wait", project, zone, opName))

	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return false, fmt.Errorf("failed to create operation wait request: %w", err)
		}
		req.Header.Set(anePreviewAPIVersionHeader, anePreviewAPIVersion)
		s.acceptRateLimit()

		resp, err := s.httpClient().Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf("operation %q wait returned HTTP %d: %s", opName, resp.StatusCode, string(body))
		}

		var op aneOperation
		if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
			return false, fmt.Errorf("failed to decode operation %q: %w", opName, err)
		}

		if op.Status != "DONE" {
			return false, nil
		}

		if op.HttpErrorStatusCode >= 400 || (op.Error != nil && (len(op.Error.Errors) > 0 || len(op.Error.Details) > 0)) {
			var detailStrs []string
			if op.Error != nil {
				for _, e := range op.Error.Errors {
					if e.Message != "" {
						detailStrs = append(detailStrs, e.Message)
					}
				}
				for _, d := range op.Error.Details {
					if d.Reason != "" {
						detailStrs = append(detailStrs, fmt.Sprintf("reason=%s domain=%s", d.Reason, d.Domain))
					}
				}
			}
			if len(detailStrs) == 0 && op.HttpErrorMessage != "" {
				detailStrs = append(detailStrs, op.HttpErrorMessage)
			}
			return false, fmt.Errorf("zone operation %q failed (HTTP %d): %s", opName, op.HttpErrorStatusCode, strings.Join(detailStrs, "; "))
		}
		return true, nil
	})
}

type fakeANEService struct {
	mu        sync.RWMutex
	endpoints map[string]*AliasNetworkEndpoint
	nextIP    int
}

func newFakeANEService() *fakeANEService {
	return &fakeANEService{
		endpoints: make(map[string]*AliasNetworkEndpoint),
	}
}

func (f *fakeANEService) CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.endpoints[endpoint.Name]; exists {
		return fmt.Errorf("AliasNetworkEndpoint %q already exists", endpoint.Name)
	}

	epCopy := *endpoint
	if epCopy.Status == nil {
		epCopy.Status = &ANEStatus{State: ANEStateActive}
		if alias, ok := epCopy.Aliases[DefaultANEAliasName]; ok && alias != nil {
			if alias.EffectiveIPAddress == "" {
				if alias.IPAddress != "" {
					alias.EffectiveIPAddress = alias.IPAddress
				} else {
					f.nextIP++
					alias.EffectiveIPAddress = fmt.Sprintf("10.128.0.%d", f.nextIP)
				}
			}
		}
	}
	f.endpoints[endpoint.Name] = &epCopy
	return nil
}

func (f *fakeANEService) DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.endpoints, name)
	return nil
}

func (f *fakeANEService) GetAliasNetworkEndpoint(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	ep, exists := f.endpoints[name]
	if !exists {
		return nil, fmt.Errorf("AliasNetworkEndpoint %q not found", name)
	}
	epCopy := *ep
	return &epCopy, nil
}

func (f *fakeANEService) ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	var result []*AliasNetworkEndpoint
	for _, ep := range f.endpoints {
		if ep.Host != nil && EqualResourceURLs(ep.Host.Instance, instanceURL) {
			epCopy := *ep
			result = append(result, &epCopy)
		}
	}
	return result, nil
}
