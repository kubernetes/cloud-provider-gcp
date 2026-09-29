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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPClient_InsertGetListDelete(t *testing.T) {
	endpoints := make(map[string]*AliasNetworkEndpoint)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify preview API version header is present on every request
		if v := r.Header.Get(PreviewAPIVersionHeader); v != PreviewAPIVersion {
			http.Error(w, "missing or invalid "+PreviewAPIVersionHeader+": "+v, http.StatusBadRequest)
			return
		}
		// Verify $apiVersion query param is present on every request (Finding 4)
		if !strings.Contains(r.URL.RawQuery, "%24apiVersion="+PreviewAPIVersion) && !strings.Contains(r.URL.RawQuery, "$apiVersion="+PreviewAPIVersion) {
			http.Error(w, "missing or invalid $apiVersion in query: "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPost:
			var ane AliasNetworkEndpoint
			if err := json.NewDecoder(r.Body).Decode(&ane); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ane.Status = &Status{State: StateActive}
			ane.ResourceMetadata = &ResourceMetadata{APIVersion: PreviewAPIVersion}
			if alias, ok := ane.Aliases[DefaultAliasName]; ok && alias != nil {
				alias.EffectiveIPAddress = "10.128.0.25"
			}
			endpoints[ane.Name] = &ane
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Operation{Name: "op-insert-" + ane.Name, Status: "DONE"})

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
			json.NewEncoder(w).Encode(AliasNetworkEndpointList{Items: list})

		case http.MethodDelete:
			path := r.URL.Path
			name := path[strings.LastIndex(path, "/")+1:]
			if _, ok := endpoints[name]; !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			delete(endpoints, name)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Operation{Name: "op-delete-" + name, Status: "DONE"})
		}
	}))
	defer server.Close()

	var metricCalls int32
	observer := func(request, zone string, start time.Time, err error) {
		atomic.AddInt32(&metricCalls, 1)
	}

	client := NewHTTPClientWithOptions(
		server.Client(),
		server.URL+"/projects/",
		func(ctx context.Context, p, z, op string) error {
			return nil
		},
		ClientOptions{
			APIVersion:     PreviewAPIVersion,
			MetricObserver: observer,
		},
	)

	ctx := context.Background()
	ane := &AliasNetworkEndpoint{
		Name: "test-ane-1",
		Host: &Host{
			Instance: "projects/p/zones/z/instances/test-node",
		},
		Aliases: map[string]*Alias{
			DefaultAliasName: {
				IPVersion: DefaultIPVersion,
			},
		},
	}

	opName, err := client.Insert(ctx, "p", "z", ane)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if opName != "op-insert-test-ane-1" {
		t.Errorf("Unexpected opName: %q", opName)
	}

	got, err := client.Get(ctx, "p", "z", "test-ane-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Aliases[DefaultAliasName].EffectiveIPAddress != "10.128.0.25" {
		t.Errorf("Expected effective IP 10.128.0.25, got %q", got.Aliases[DefaultAliasName].EffectiveIPAddress)
	}

	list, err := client.ListByInstance(ctx, "p", "z", "projects/p/zones/z/instances/test-node")
	if err != nil {
		t.Fatalf("ListByInstance failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Expected 1 item in list, got %d", len(list))
	}

	delOp, err := client.Delete(ctx, "p", "z", "test-ane-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if delOp != "op-delete-test-ane-1" {
		t.Errorf("Unexpected delOp: %q", delOp)
	}

	// Idempotent delete on non-existent resource should return ("", nil) without error
	delOp404, err := client.Delete(ctx, "p", "z", "test-ane-nonexistent")
	if err != nil {
		t.Fatalf("Delete non-existent ANE returned error: %v", err)
	}
	if delOp404 != "" {
		t.Errorf("Expected empty op name for 404 delete, got %q", delOp404)
	}

	listAfter, err := client.ListByInstance(ctx, "p", "z", "projects/p/zones/z/instances/test-node")
	if err != nil {
		t.Fatalf("ListByInstance after delete failed: %v", err)
	}
	if len(listAfter) != 0 {
		t.Fatalf("Expected 0 items after delete, got %d", len(listAfter))
	}

	if atomic.LoadInt32(&metricCalls) == 0 {
		t.Errorf("Expected metric observer to be called for API operations")
	}
}

func TestHTTPClient_TransientRetryAndRetryAfter(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// First attempt simulates 429 with Retry-After header (Finding 6)
			w.Header().Set("Retry-After", "1")
			http.Error(w, "RESOURCE_EXHAUSTED: rate limited", http.StatusTooManyRequests)
			return
		}
		// Second attempt succeeds
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Operation{Name: "op-insert-retry-success", Status: "DONE"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL+"/projects/", nil)
	ane := &AliasNetworkEndpoint{
		Name: "test-ane-retry",
	}

	opName, err := client.Insert(context.Background(), "p", "z", ane)
	if err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got error: %v", err)
	}
	if opName != "op-insert-retry-success" {
		t.Errorf("unexpected opName: %q", opName)
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestHTTPClient_Delete_UnallowlistedMethodNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate ESF visibility gate returning HTTP 404 Method not found (Finding 8)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Method not found."))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL+"/projects/", nil)
	_, err := client.Delete(context.Background(), "p", "z", "test-ane")
	if err == nil {
		t.Fatal("expected error on ESF Method not found 404, got nil")
	}
	if !strings.Contains(err.Error(), "Method not found") && !strings.Contains(err.Error(), "unallowlisted") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPClient_AIP193StructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		errResp := APIErrorResponse{}
		errResp.Error.Code = 400
		errResp.Error.Message = "Invalid API version"
		errResp.Error.Details = []ErrorInfo{
			{
				Type:   "type.googleapis.com/google.rpc.ErrorInfo",
				Reason: "API_VERSION_NOT_SUPPORTED",
				Domain: "compute.googleapis.com",
			},
		}
		json.NewEncoder(w).Encode(errResp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), server.URL+"/projects/", nil)
	_, err := client.Insert(context.Background(), "p", "z", &AliasNetworkEndpoint{Name: "test"})
	if err == nil {
		t.Fatal("expected AIP-193 structured error, got nil")
	}
	if !strings.Contains(err.Error(), "API_VERSION_NOT_SUPPORTED") {
		t.Errorf("expected error to contain structured reason API_VERSION_NOT_SUPPORTED, got: %v", err)
	}
}

func TestHTTPClient_WaitForOperationTimeout(t *testing.T) {
	// Finding 11: Use mock/parameterized test URL instead of hardcoded compute.googleapis.com
	client := NewHTTPClient(nil, "http://127.0.0.1:8080/projects/", func(ctx context.Context, p, z, op string) error {
		<-ctx.Done()
		return ctx.Err()
	})

	// When using a context with 50ms deadline, waiter should fail fast with context error
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.WaitForOperation(shortCtx, "p", "z", "op-stalled")
	if err == nil {
		t.Fatal("expected error waiting on stalled operation, got nil")
	}
}
