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

package ane

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
	"time"

	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

// TODO(upstream): Migrate from raw REST HTTP client to the official generated
// GCE Go compute client once AliasNetworkEndpoints reaches GA in google.golang.org/api/compute/v1.
// See tracking issue: https://github.com/kubernetes/cloud-provider-gcp/issues/1369

const (
	// DefaultOperationTimeout is the maximum time to wait for a zonal operation to reach DONE.
	// Healthy operations complete in 6-10s. If an operation stalls (e.g. during CLH rollback),
	// we fail fast after 20s to allow the controller workqueue to self-heal.
	DefaultOperationTimeout = 20 * time.Second

	defaultMaxTransientRetries = 3
	defaultInitialRetryBackoff = 250 * time.Millisecond
)

// OperationWaiter defines a callback to wait for an operation to complete.
type OperationWaiter func(ctx context.Context, project, zone, opName string) error

// RateLimiter defines the interface for token bucket or QPS rate limiting.
type RateLimiter interface {
	Accept()
}

// MetricObserver defines a callback function to record request latency and errors.
type MetricObserver func(request, zone string, start time.Time, err error)

// ClientOptions provides configuration options for the ANE REST Client.
type ClientOptions struct {
	APIVersion     string
	RateLimiter    RateLimiter
	MetricObserver MetricObserver
}

// Client is the interface for performing GCE AliasNetworkEndpoint operations.
type Client interface {
	// Insert creates a new AliasNetworkEndpoint and returns the operation name.
	Insert(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) (string, error)
	// Delete deletes an AliasNetworkEndpoint and returns the operation name.
	Delete(ctx context.Context, project, zone, name string) (string, error)
	// Get retrieves an AliasNetworkEndpoint by name.
	Get(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error)
	// ListByInstance lists all AliasNetworkEndpoints in the zone bound to the given instance.
	ListByInstance(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error)
	// WaitForOperation waits for a zonal operation to complete.
	WaitForOperation(ctx context.Context, project, zone, opName string) error
}

type httpClient struct {
	client           *http.Client
	projectsBasePath string
	apiVersion       string
	waiter           OperationWaiter
	rateLimiter      RateLimiter
	metricObserver   MetricObserver
}

// NewHTTPClient creates a new Client using raw HTTP calls against the GCE REST API with default options.
func NewHTTPClient(client *http.Client, projectsBasePath string, waiter OperationWaiter) Client {
	return NewHTTPClientWithOptions(client, projectsBasePath, waiter, ClientOptions{})
}

// NewHTTPClientWithOptions creates a new Client with custom options (API version, rate limiter, metrics).
func NewHTTPClientWithOptions(client *http.Client, projectsBasePath string, waiter OperationWaiter, opts ClientOptions) Client {
	if client == nil {
		client = http.DefaultClient
	}
	if !strings.HasSuffix(projectsBasePath, "/") {
		projectsBasePath += "/"
	}
	apiVersion := opts.APIVersion
	if apiVersion == "" {
		apiVersion = PreviewAPIVersion
	}
	return &httpClient{
		client:           client,
		projectsBasePath: projectsBasePath,
		apiVersion:       apiVersion,
		waiter:           waiter,
		rateLimiter:      opts.RateLimiter,
		metricObserver:   opts.MetricObserver,
	}
}

func (c *httpClient) buildURL(path string, queryParams ...string) string {
	versionParam := "%24apiVersion=" + c.apiVersion
	allParams := append([]string{versionParam}, queryParams...)
	return fmt.Sprintf("%s%s?%s", c.projectsBasePath, strings.TrimPrefix(path, "/"), strings.Join(allParams, "&"))
}

func (c *httpClient) observe(request, zone string, start time.Time, err error) {
	if c.metricObserver != nil {
		c.metricObserver(request, zone, start, err)
	}
}

func isTransientHTTPStatus(statusCode int) bool {
	return statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout ||
		statusCode == http.StatusTooManyRequests
}

func parseRESTError(statusCode int, body []byte) error {
	var apiErr APIErrorResponse
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
	var apiErr APIErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil {
		for _, d := range apiErr.Error.Details {
			if d.Reason == "METHOD_NOT_FOUND" || d.Reason == "API_VERSION_NOT_SUPPORTED" {
				return true
			}
		}
	}
	return false
}

func (c *httpClient) doWithRetry(ctx context.Context, newReq func() (*http.Request, error)) (*http.Response, error) {
	var resp *http.Response
	var lastErr error
	backoff := defaultInitialRetryBackoff

	for attempt := 0; attempt < defaultMaxTransientRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		if c.rateLimiter != nil {
			c.rateLimiter.Accept()
		}

		req, err := newReq()
		if err != nil {
			return nil, err
		}

		resp, lastErr = c.client.Do(req)
		if lastErr != nil {
			continue
		}

		if isTransientHTTPStatus(resp.StatusCode) {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					backoff = time.Duration(seconds) * time.Second
				}
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = parseRESTError(resp.StatusCode, body)
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

func (c *httpClient) Insert(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) (string, error) {
	start := time.Now()
	var opName string
	var err error
	defer func() {
		c.observe("insert", zone, start, err)
	}()

	url := c.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone))
	data, err := json.Marshal(endpoint)
	if err != nil {
		err = fmt.Errorf("failed to marshal ANE payload: %w", err)
		return "", err
	}

	resp, err := c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create insert request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(PreviewAPIVersionHeader, c.apiVersion)
		return req, nil
	})
	if err != nil {
		return "", fmt.Errorf("GCE ANE insert request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err = parseRESTError(resp.StatusCode, body)
		return "", err
	}

	var op Operation
	if err = json.NewDecoder(resp.Body).Decode(&op); err != nil {
		err = fmt.Errorf("failed to decode insert operation: %w", err)
		return "", err
	}
	opName = op.Name
	return opName, nil
}

func (c *httpClient) Delete(ctx context.Context, project, zone, name string) (string, error) {
	start := time.Now()
	var opName string
	var err error
	defer func() {
		c.observe("delete", zone, start, err)
	}()

	url := c.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints/%s", project, zone, name))
	resp, err := c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create delete request: %w", err)
		}
		req.Header.Set(PreviewAPIVersionHeader, c.apiVersion)
		return req, nil
	})
	if err != nil {
		return "", fmt.Errorf("GCE ANE delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if isUnallowlistedOrMethodNotFound(body, bodyStr) {
			err = fmt.Errorf("GCE ANE preview API endpoint not found or unallowlisted (HTTP 404): %s", bodyStr)
			return "", err
		}
		// Idempotent deletion: resource already deleted or not found
		return "", nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err = parseRESTError(resp.StatusCode, body)
		return "", err
	}

	var op Operation
	if err = json.NewDecoder(resp.Body).Decode(&op); err != nil {
		err = fmt.Errorf("failed to decode delete operation: %w", err)
		return "", err
	}
	opName = op.Name
	return opName, nil
}

func (c *httpClient) Get(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error) {
	start := time.Now()
	var ane *AliasNetworkEndpoint
	var err error
	defer func() {
		c.observe("get", zone, start, err)
	}()

	url := c.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints/%s", project, zone, name))
	resp, err := c.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create get request: %w", err)
		}
		req.Header.Set(PreviewAPIVersionHeader, c.apiVersion)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("GCE ANE get request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err = parseRESTError(resp.StatusCode, body)
		return nil, err
	}

	ane = &AliasNetworkEndpoint{}
	if err = json.NewDecoder(resp.Body).Decode(ane); err != nil {
		err = fmt.Errorf("failed to decode ANE get response: %w", err)
		return nil, err
	}
	return ane, nil
}

func (c *httpClient) ListByInstance(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error) {
	start := time.Now()
	var result []*AliasNetworkEndpoint
	var err error
	defer func() {
		c.observe("list", zone, start, err)
	}()

	pageToken := ""
	for {
		var url string
		if pageToken != "" {
			url = c.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone), "pageToken="+pageToken)
		} else {
			url = c.buildURL(fmt.Sprintf("%s/zones/%s/aliasNetworkEndpoints", project, zone))
		}

		resp, err := c.doWithRetry(ctx, func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create list request: %w", err)
			}
			req.Header.Set(PreviewAPIVersionHeader, c.apiVersion)
			return req, nil
		})
		if err != nil {
			return nil, fmt.Errorf("GCE ANE list request failed: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			err = parseRESTError(resp.StatusCode, body)
			return nil, err
		}

		var listResp AliasNetworkEndpointList
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		resp.Body.Close()
		if err != nil {
			err = fmt.Errorf("failed to decode ANE list response: %w", err)
			return nil, err
		}

		for _, item := range listResp.Items {
			if item != nil && item.Host != nil && gce.EqualResourceURLs(item.Host.Instance, instanceURL) {
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

func (c *httpClient) WaitForOperation(ctx context.Context, project, zone, opName string) error {
	if c.waiter != nil {
		waitCtx, cancel := context.WithTimeout(ctx, DefaultOperationTimeout)
		defer cancel()
		err := c.waiter(waitCtx, project, zone, opName)
		if err != nil {
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("operation %q exceeded %v timeout: %w", opName, DefaultOperationTimeout, err)
			}
			return err
		}
	}
	return nil
}
