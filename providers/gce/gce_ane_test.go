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
)

func TestGCEANEService_CreateGetListDelete(t *testing.T) {
	endpoints := make(map[string]*AliasNetworkEndpoint)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify preview API version header is present.
		if v := r.Header.Get(anePreviewAPIVersionHeader); v != anePreviewAPIVersion {
			http.Error(w, "missing or invalid "+anePreviewAPIVersionHeader+": "+v, http.StatusBadRequest)
			return
		}
		// Verify $apiVersion query param is present on every request.
		if !strings.Contains(r.URL.RawQuery, "%24apiVersion="+anePreviewAPIVersion) && !strings.Contains(r.URL.RawQuery, "$apiVersion="+anePreviewAPIVersion) {
			http.Error(w, "missing or invalid $apiVersion in query: "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPost:
			if strings.HasSuffix(r.URL.Path, "/wait") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(aneOperation{Name: "op-wait", Status: "DONE"})
				return
			}
			var ane AliasNetworkEndpoint
			if err := json.NewDecoder(r.Body).Decode(&ane); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ane.Status = &ANEStatus{State: ANEStateActive}
			ane.ResourceMetadata = &ANEResourceMetadata{APIVersion: anePreviewAPIVersion}
			for _, alias := range ane.Aliases {
				if alias != nil {
					alias.EffectiveIPAddress = "10.128.0.25"
				}
			}
			endpoints[ane.Name] = &ane
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(aneOperation{Name: "op-insert-" + ane.Name, Status: "DONE"})

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
			var list []*AliasNetworkEndpoint
			for _, ep := range endpoints {
				list = append(list, ep)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(aneListResponse{Items: list})

		case http.MethodDelete:
			path := r.URL.Path
			name := path[strings.LastIndex(path, "/")+1:]
			if _, ok := endpoints[name]; !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			delete(endpoints, name)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(aneOperation{Name: "op-delete-" + name, Status: "DONE"})
		}
	}))
	defer server.Close()

	cloud := &Cloud{
		httpClient:       server.Client(),
		projectsBasePath: server.URL + "/projects/",
	}

	ctx := context.Background()
	testProviderID := "gce://p/z/test-node"
	const testAliasKey = "alias-ipv4"
	ane := &AliasNetworkEndpoint{
		Name: "test-ane-1",
		Aliases: map[string]*ANEAlias{
			testAliasKey: {
				IPVersion: ANEIPVersionIPv4,
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
	if got.Aliases[testAliasKey].EffectiveIPAddress != "10.128.0.25" {
		t.Errorf("Expected effective IP 10.128.0.25, got %q", got.Aliases[testAliasKey].EffectiveIPAddress)
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
			json.NewEncoder(w).Encode(aneOperation{Name: "op-insert-retry-success", Status: "DONE"})
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
		json.NewEncoder(w).Encode(aneOperation{Name: "op-insert-retry-success", Status: "DONE"})
	}))
	defer server.Close()

	cloud := &Cloud{
		httpClient:       server.Client(),
		projectsBasePath: server.URL + "/projects/",
	}
	ane := &AliasNetworkEndpoint{
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

	cloud := &Cloud{
		httpClient:       server.Client(),
		projectsBasePath: server.URL + "/projects/",
	}
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
		errResp := aneAPIErrorResponse{}
		errResp.Error.Code = 400
		errResp.Error.Message = "Invalid API version"
		errResp.Error.Details = []aneErrorInfo{
			{
				Type:   "type.googleapis.com/google.rpc.ErrorInfo",
				Reason: "API_VERSION_NOT_SUPPORTED",
				Domain: "compute.googleapis.com",
			},
		}
		json.NewEncoder(w).Encode(errResp)
	}))
	defer server.Close()

	cloud := &Cloud{
		httpClient:       server.Client(),
		projectsBasePath: server.URL + "/projects/",
	}
	err := cloud.CreateAliasNetworkEndpoint(context.Background(), "gce://p/z/test-node", &AliasNetworkEndpoint{Name: "test"})
	if err == nil {
		t.Fatal("expected AIP-193 structured error, got nil")
	}
	if !strings.Contains(err.Error(), "API_VERSION_NOT_SUPPORTED") {
		t.Errorf("expected error to contain structured reason API_VERSION_NOT_SUPPORTED, got: %v", err)
	}
}

func TestGCEANEService_WaitForOperationTimeout(t *testing.T) {
	// Use mock/parameterized test URL instead of hardcoded
	// compute.googleapis.com.
	svc := &gceANEService{
		gce: &Cloud{
			projectsBasePath: "http://127.0.0.1:8080/projects/",
		},
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
		json.NewEncoder(w).Encode(aneOperation{
			Name:   "op-123",
			Status: "DONE",
		})
	}))
	defer server.Close()

	svc := &gceANEService{
		gce: &Cloud{
			httpClient:       server.Client(),
			projectsBasePath: server.URL + "/projects/",
		},
	}
	if err := svc.waitForOperation(context.Background(), "p", "z", "op-123"); err != nil {
		t.Fatalf("expected waitForOperation to succeed, got error: %v", err)
	}
	if polls != 1 {
		t.Errorf("expected 1 poll, got %d", polls)
	}
}
