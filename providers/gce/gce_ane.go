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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	computepreview "google.golang.org/api/compute/v1.preview"
	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Lifecycle state constants for AliasNetworkEndpointStatus.State.
const (
	// ANEStateUnspecified indicates an unspecified endpoint state.
	ANEStateUnspecified = "STATE_UNSPECIFIED"

	// ANEStateCreating indicates the endpoint is being created and is not
	// yet ready to process traffic.
	ANEStateCreating = "CREATING"

	// ANEStateActive indicates the endpoint is active and programmed in the
	// dataplane to process traffic.
	ANEStateActive = "ACTIVE"

	// ANEStateDeleting indicates the endpoint is being deleted.
	ANEStateDeleting = "DELETING"

	// ANEStateFrozen indicates the endpoint is frozen and deprogrammed from
	// the dataplane.
	ANEStateFrozen = "FROZEN"
)

// Configuration and GCE API enum constants for AliasNetworkEndpoint.
const (
	// ANEIPVersionIPv4 is the IPv4 ipVersion value for
	// AliasNetworkEndpointAlias.IpVersion.
	ANEIPVersionIPv4 = "IPV4"

	// ANEIPVersionIPv6 is the IPv6 ipVersion value for
	// AliasNetworkEndpointAlias.IpVersion.
	ANEIPVersionIPv6 = "IPV6"

	// ANESecurityTagInheritanceInherited configures the endpoint to inherit
	// security tags directly from its host VM instance so firewall rules
	// targeting the host instance's tags apply to this endpoint.
	ANESecurityTagInheritanceInherited = "INHERITED"

	// ANESecurityTagInheritanceNotInherited configures the endpoint not to
	// inherit security tags from its host VM instance.
	ANESecurityTagInheritanceNotInherited = "NOT_INHERITED"

	// DefaultANEOperationTimeout is the maximum time to wait for a zonal
	// ANE operation to reach DONE. Healthy operations complete in 6-10s. If
	// an operation stalls (e.g. during CLH rollback), we fail fast after
	// 20s to allow the controller workqueue to self-heal.
	DefaultANEOperationTimeout = 20 * time.Second

	anePreviewAPIVersion          = "2026-10-01-preview"
	defaultANEMaxTransientRetries = 3
	defaultANEInitialRetryBackoff = 250 * time.Millisecond
)

func newANEMetricContext(request, zone string) *metricContext {
	return newGenericMetricContext("ane", request, unusedMetricLabel, zone, anePreviewAPIVersion)
}

type aneServiceManager interface {
	CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *computepreview.AliasNetworkEndpoint) error
	DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error
	ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*computepreview.AliasNetworkEndpoint, error)
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
func (g *Cloud) CreateAliasNetworkEndpoint(ctx context.Context, providerID string, endpoint *computepreview.AliasNetworkEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}
	project, zone, _, instanceURL, err := parseInstanceProviderID(providerID)
	if err != nil {
		return err
	}
	if endpoint.Host == nil {
		endpoint.Host = &computepreview.AliasNetworkEndpointHost{Instance: instanceURL}
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

// ListAliasNetworkEndpoints lists all AliasNetworkEndpoints bound to the
// instance identified by providerID.
func (g *Cloud) ListAliasNetworkEndpoints(ctx context.Context, providerID string) ([]*computepreview.AliasNetworkEndpoint, error) {
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

func (s *gceANEService) acceptRateLimit() {
	if s.gce != nil && s.gce.operationPollRateLimiter != nil {
		s.gce.operationPollRateLimiter.Accept()
	}
}

func isTransientHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusRequestTimeout
}

func isUnallowlistedOrMethodNotFound(apiErr *googleapi.Error) bool {
	errStr := apiErr.Error()
	for _, token := range []string{"Method not found", "METHOD_NOT_FOUND", "API_VERSION_NOT_SUPPORTED"} {
		if strings.Contains(errStr, token) || strings.Contains(apiErr.Body, token) {
			return true
		}
	}
	return false
}

func sleepRetryAfter(ctx context.Context, header http.Header) error {
	seconds, err := strconv.Atoi(header.Get("Retry-After"))
	if err != nil || seconds <= 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *gceANEService) doWithRetry(ctx context.Context, call func() error) error {
	var lastErr error
	backoff := wait.Backoff{
		Duration: defaultANEInitialRetryBackoff,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    defaultANEMaxTransientRetries,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		lastErr = call()
		if lastErr == nil {
			return true, nil
		}

		var apiErr *googleapi.Error
		if errors.As(lastErr, &apiErr) && isTransientHTTPStatus(apiErr.Code) {
			if err := sleepRetryAfter(ctx, apiErr.Header); err != nil {
				return false, err
			}
			return false, nil
		}

		return false, lastErr
	})

	if ctx.Err() != nil {
		if lastErr != nil && !errors.Is(lastErr, ctx.Err()) {
			return fmt.Errorf("%w (last error: %w)", ctx.Err(), lastErr)
		}
		return ctx.Err()
	}
	if wait.Interrupted(err) && lastErr != nil {
		return lastErr
	}
	return err
}

func (s *gceANEService) CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *computepreview.AliasNetworkEndpoint) error {
	var op *computepreview.Operation
	err := s.doWithRetry(ctx, func() error {
		var callErr error
		op, callErr = s.gce.servicePreview.AliasNetworkEndpoints.Insert(project, zone, endpoint).Context(ctx).Do()
		return callErr
	})
	if err != nil {
		return fmt.Errorf("GCE ANE insert request failed: %w", err)
	}
	if op != nil && op.Name != "" {
		if err := s.waitForOperation(ctx, project, zone, op.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *gceANEService) DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error {
	var op *computepreview.Operation
	err := s.doWithRetry(ctx, func() error {
		var callErr error
		op, callErr = s.gce.servicePreview.AliasNetworkEndpoints.Delete(project, zone, name).Context(ctx).Do()
		return callErr
	})
	if err != nil {
		var apiErr *googleapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			if isUnallowlistedOrMethodNotFound(apiErr) {
				return fmt.Errorf("GCE ANE preview API endpoint not found or unallowlisted (HTTP 404): %w", err)
			}
			// Idempotent deletion: resource already deleted or not
			// found.
			return nil
		}
		return fmt.Errorf("GCE ANE delete request failed: %w", err)
	}
	if op != nil && op.Name != "" {
		if err := s.waitForOperation(ctx, project, zone, op.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *gceANEService) ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*computepreview.AliasNetworkEndpoint, error) {
	var result []*computepreview.AliasNetworkEndpoint
	// Use a regex suffix filter rather than an exact `host.instance = instanceURL`
	// match because `instanceURL` contains the alphanumeric project ID, whereas
	// GCE normalizes `host.instance` to the numeric project number, causing an
	// exact filter with the project ID to return 0 items.
	//
	// TODO(#1399): Once the GCE ANE API returns project IDs in `host.instance`
	// per AIP-122 (https://github.com/kubernetes/cloud-provider-gcp/issues/1399),
	// replace this regex suffix filter with an exact match:
	// `fmt.Sprintf("host.instance = %q", instanceURL)`.
	instanceFilter := ""
	if instanceName := lastComponent(instanceURL); zone != "" && instanceName != "" {
		instanceFilter = fmt.Sprintf(`host.instance eq ".*zones/%s/instances/%s$"`, zone, instanceName)
	}
	pageToken := ""
	for {
		var listResp *computepreview.AliasNetworkEndpointList
		err := s.doWithRetry(ctx, func() error {
			call := s.gce.servicePreview.AliasNetworkEndpoints.List(project, zone).Context(ctx)
			if instanceFilter != "" {
				call = call.Filter(instanceFilter)
			}
			if pageToken != "" {
				call = call.PageToken(pageToken)
			}
			var callErr error
			listResp, callErr = call.Do()
			return callErr
		})
		if err != nil {
			return nil, fmt.Errorf("GCE ANE list request failed: %w", err)
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
	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		s.acceptRateLimit()

		op, err := s.gce.servicePreview.ZoneOperations.Wait(project, zone, opName).Context(ctx).Do()
		if err != nil {
			return false, fmt.Errorf("operation %q wait failed: %w", opName, err)
		}

		if op.Status != "DONE" {
			return false, nil
		}

		if op.HttpErrorStatusCode >= 400 || (op.Error != nil && len(op.Error.Errors) > 0) {
			var detailStrs []string
			if op.Error != nil {
				for _, e := range op.Error.Errors {
					if e == nil {
						continue
					}
					if e.Message != "" {
						detailStrs = append(detailStrs, e.Message)
					} else if e.Code != "" {
						detailStrs = append(detailStrs, e.Code)
					}
					for _, d := range e.ErrorDetails {
						if d != nil && d.ErrorInfo != nil && d.ErrorInfo.Reason != "" {
							detailStrs = append(detailStrs, fmt.Sprintf("reason=%s domain=%s", d.ErrorInfo.Reason, d.ErrorInfo.Domain))
						}
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
