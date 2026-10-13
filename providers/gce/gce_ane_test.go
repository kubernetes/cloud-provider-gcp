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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	computepreview "google.golang.org/api/compute/v1.preview"
	"google.golang.org/api/option"
)

func newTestCloudWithServer(t *testing.T, server *httptest.Server) *Cloud {
	t.Helper()
	svc, err := computepreview.NewService(
		context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(server.Client()),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("computepreview.NewService failed: %v", err)
	}
	return &Cloud{
		servicePreview: svc,
	}
}

func TestGCEANEService_CreateGetListDelete(t *testing.T) {
	endpoints := make(map[string]*computepreview.AliasNetworkEndpoint)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify preview API version header is present.
		if v := r.Header.Get("X-Goog-Api-Version"); v != anePreviewAPIVersion {
			http.Error(w, "missing or invalid X-Goog-Api-Version: "+v, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPost:
			if strings.HasSuffix(r.URL.Path, "/wait") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(computepreview.Operation{Name: "op-wait", Status: "DONE"})
				return
			}
			var ane computepreview.AliasNetworkEndpoint
			if err := json.NewDecoder(r.Body).Decode(&ane); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ane.Status = &computepreview.AliasNetworkEndpointStatus{State: ANEStateActive}
			for k, alias := range ane.Aliases {
				alias.EffectiveIpAddress = "10.128.0.25"
				ane.Aliases[k] = alias
			}
			endpoints[ane.Name] = &ane
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(computepreview.Operation{Name: "op-insert-" + ane.Name, Status: "DONE"})

		case http.MethodGet:
			path := r.URL.Path
			if strings.Contains(path, "aliasNetworkEndpoints/") {
				name := path[strings.LastIndex(path, "/")+1:]
				ane, ok := endpoints[name]
				if !ok {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ane)
				return
			}
			// List
			var list []*computepreview.AliasNetworkEndpoint
			for _, ep := range endpoints {
				list = append(list, ep)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(computepreview.AliasNetworkEndpointList{Items: list})

		case http.MethodDelete:
			path := r.URL.Path
			name := path[strings.LastIndex(path, "/")+1:]
			if _, ok := endpoints[name]; !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			delete(endpoints, name)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(computepreview.Operation{Name: "op-delete-" + name, Status: "DONE"})
		}
	}))
	defer server.Close()

	cloud := newTestCloudWithServer(t, server)

	ctx := context.Background()
	testProviderID := "gce://p/z/test-node"
	const testAliasKey = "alias-ipv4"
	ane := &computepreview.AliasNetworkEndpoint{
		Name: "test-ane-1",
		Aliases: map[string]computepreview.AliasNetworkEndpointAlias{
			testAliasKey: {
				IpVersion: ANEIPVersionIPv4,
			},
		},
	}

	if err := cloud.CreateAliasNetworkEndpoint(ctx, testProviderID, ane); err != nil {
		t.Fatalf("CreateAliasNetworkEndpoint failed: %v", err)
	}

	got, err := cloud.GetAliasNetworkEndpoint(ctx, testProviderID, "test-ane-1")
	if err != nil {
		t.Fatalf("GetAliasNetworkEndpoint failed: %v", err)
	}
	if got.Aliases[testAliasKey].EffectiveIpAddress != "10.128.0.25" {
		t.Errorf("Expected effective IP 10.128.0.25, got %q", got.Aliases[testAliasKey].EffectiveIpAddress)
	}

	list, err := cloud.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 item in list, got %d", len(list))
	}

	if err := cloud.DeleteAliasNetworkEndpoint(ctx, testProviderID, "test-ane-1"); err != nil {
		t.Fatalf("DeleteAliasNetworkEndpoint failed: %v", err)
	}

	// Idempotent delete on non-existent resource should succeed without
	// error.
	if err := cloud.DeleteAliasNetworkEndpoint(ctx, testProviderID, "test-ane-nonexistent"); err != nil {
		t.Fatalf("DeleteAliasNetworkEndpoint non-existent ANE returned error: %v", err)
	}

	listAfter, err := cloud.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints after delete failed: %v", err)
	}
	if len(listAfter) != 0 {
		t.Fatalf("Expected 0 items after delete, got %d", len(listAfter))
	}
}

func TestGCEANEService_TransientRetryAndRetryAfter(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/wait") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(computepreview.Operation{Name: "op-insert-retry-success", Status: "DONE"})
			return
		}
		attempts++
		if attempts == 1 {
			// First attempt simulates 429 with Retry-After header.
			w.Header().Set("Retry-After", "1")
			http.Error(w, "RESOURCE_EXHAUSTED: rate limited", http.StatusTooManyRequests)
			return
		}
		// Second attempt succeeds.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(computepreview.Operation{Name: "op-insert-retry-success", Status: "DONE"})
	}))
	defer server.Close()

	cloud := newTestCloudWithServer(t, server)
	ane := &computepreview.AliasNetworkEndpoint{
		Name: "test-ane-retry",
	}

	if err := cloud.CreateAliasNetworkEndpoint(context.Background(), "gce://p/z/test-node", ane); err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestGCEANEService_Delete_UnallowlistedMethodNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate ESF visibility gate returning HTTP 404.
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Method not found."))
	}))
	defer server.Close()

	cloud := newTestCloudWithServer(t, server)
	err := cloud.DeleteAliasNetworkEndpoint(context.Background(), "gce://p/z/test-node", "test-ane")
	if err == nil {
		t.Fatal("expected error on ESF Method not found 404, got nil")
	}
	if !strings.Contains(err.Error(), "Method not found") && !strings.Contains(err.Error(), "unallowlisted") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGCEANEService_AIP193StructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{
			"error": {
				"code": 400,
				"message": "Invalid API version",
				"details": [
					{
						"@type": "type.googleapis.com/google.rpc.ErrorInfo",
						"reason": "API_VERSION_NOT_SUPPORTED",
						"domain": "compute.googleapis.com"
					}
				]
			}
		}`))
	}))
	defer server.Close()

	cloud := newTestCloudWithServer(t, server)
	err := cloud.CreateAliasNetworkEndpoint(context.Background(), "gce://p/z/test-node", &computepreview.AliasNetworkEndpoint{Name: "test"})
	if err == nil {
		t.Fatal("expected AIP-193 structured error, got nil")
	}
	if !strings.Contains(err.Error(), "API_VERSION_NOT_SUPPORTED") {
		t.Errorf("expected error to contain structured reason API_VERSION_NOT_SUPPORTED, got: %v", err)
	}
}

func TestGCEANEService_WaitForOperationTimeout(t *testing.T) {
	svc := &gceANEService{
		gce: &Cloud{},
		waiter: func(ctx context.Context, p, z, op string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	// When using a context with 50ms deadline, waiter should fail fast with
	// context error.
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := svc.waitForOperation(shortCtx, "p", "z", "op-stalled")
	if err == nil {
		t.Fatal("expected error waiting on stalled operation, got nil")
	}
}

func TestGCEANEService_WaitForOperationDefaultPolling(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls++
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/projects/p/zones/z/operations/op-123/wait" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(computepreview.Operation{
			Name:   "op-123",
			Status: "DONE",
		})
	}))
	defer server.Close()

	svc := &gceANEService{
		gce: newTestCloudWithServer(t, server),
	}
	if err := svc.waitForOperation(context.Background(), "p", "z", "op-123"); err != nil {
		t.Fatalf("expected waitForOperation to succeed, got error: %v", err)
	}
	if polls != 1 {
		t.Errorf("expected 1 poll, got %d", polls)
	}
}
