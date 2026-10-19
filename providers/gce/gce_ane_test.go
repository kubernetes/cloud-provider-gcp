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
	"net/http"
	"strings"
	"testing"
	"time"

	computepreview "google.golang.org/api/compute/v1.preview"
	"google.golang.org/api/googleapi"
)

func TestGCEANEService_CreateGetListDelete(t *testing.T) {
	cloud := NewFakeGCECloud(DefaultTestClusterValues())

	ctx := context.Background()
	testProviderID := "gce://p/z/test-node"
	const testAliasKey = "gke-pod-ipv4"
	ane := &computepreview.AliasNetworkEndpoint{
		Name: "test-ane-1",
		Aliases: map[string]computepreview.AliasNetworkEndpointAlias{
			testAliasKey: {
				IpVersion: ANEIPVersionIPv4,
				IpAddress: "10.128.0.25",
			},
		},
	}

	if err := cloud.CreateAliasNetworkEndpoint(ctx, testProviderID, ane); err != nil {
		t.Fatalf("CreateAliasNetworkEndpoint failed: %v", err)
	}
	if ane.Host == nil || ane.Host.Instance != "projects/p/zones/z/instances/test-node" {
		t.Errorf("Expected Host.Instance to be populated from providerID, got: %+v", ane.Host)
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

func TestGCEANEService_TransientRetry(t *testing.T) {
	svc := &gceANEService{
		gce: &Cloud{},
	}
	attempts := 0
	err := svc.doWithRetry(context.Background(), func() error {
		attempts++
		if attempts == 1 {
			return &googleapi.Error{
				Code:    http.StatusTooManyRequests,
				Message: "RESOURCE_EXHAUSTED: rate limited",
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestGCEANEService_UnallowlistedMethodNotFound(t *testing.T) {
	apiErr := &googleapi.Error{
		Code:    http.StatusNotFound,
		Message: "Method not found.",
		Body:    "Method not found.",
	}
	if !isUnallowlistedOrMethodNotFound(apiErr) {
		t.Error("expected isUnallowlistedOrMethodNotFound to return true for ESF Method not found 404")
	}

	normalNotFound := &googleapi.Error{
		Code:    http.StatusNotFound,
		Message: "The resource 'projects/p/zones/z/aliasNetworkEndpoints/test-ane' was not found",
	}
	if isUnallowlistedOrMethodNotFound(normalNotFound) {
		t.Error("expected isUnallowlistedOrMethodNotFound to return false for normal resource 404")
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
	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("unexpected error message: %v", err)
	}
}
