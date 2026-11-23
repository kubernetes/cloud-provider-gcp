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
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computealpha "google.golang.org/api/compute/v0.alpha"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
)

func TestForwardingRuleInsertTimeout(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Fatalf("fakeGCECloud() error: %v", err)
	}

	mockGCE, ok := gce.c.(*cloud.MockGCE)
	if !ok {
		t.Fatalf("could not cast cloud to MockGCE")
	}

	// 1. Test CreateRegionForwardingRule (GA Regional)
	var regionalGAInvoked bool
	mockGCE.MockForwardingRules.InsertHook = func(ctx context.Context, key *meta.Key, obj *compute.ForwardingRule, m *cloud.MockForwardingRules, options ...cloud.Option) (bool, error) {
		regionalGAInvoked = true
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("CreateRegionForwardingRule context has no deadline")
			return true, nil
		}
		duration := time.Until(deadline)
		if duration < 9*time.Minute || duration > 10*time.Minute {
			t.Errorf("CreateRegionForwardingRule timeout expected to be ~10m, got %v", duration)
		}
		return true, nil
	}

	regionalGARule := &compute.ForwardingRule{Name: "regional-ga-rule"}
	err = gce.CreateRegionForwardingRule(regionalGARule, vals.Region)
	if err != nil {
		t.Errorf("CreateRegionForwardingRule failed: %v", err)
	}
	if !regionalGAInvoked {
		t.Error("CreateRegionForwardingRule hook was not invoked")
	}

	// 2. Test CreateGlobalForwardingRule (GA Global)
	var globalGAInvoked bool
	mockGCE.MockGlobalForwardingRules.InsertHook = func(ctx context.Context, key *meta.Key, obj *compute.ForwardingRule, m *cloud.MockGlobalForwardingRules, options ...cloud.Option) (bool, error) {
		globalGAInvoked = true
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("CreateGlobalForwardingRule context has no deadline")
			return true, nil
		}
		duration := time.Until(deadline)
		if duration < 9*time.Minute || duration > 10*time.Minute {
			t.Errorf("CreateGlobalForwardingRule timeout expected to be ~10m, got %v", duration)
		}
		return true, nil
	}

	globalGARule := &compute.ForwardingRule{Name: "global-ga-rule"}
	err = gce.CreateGlobalForwardingRule(globalGARule)
	if err != nil {
		t.Errorf("CreateGlobalForwardingRule failed: %v", err)
	}
	if !globalGAInvoked {
		t.Error("CreateGlobalForwardingRule hook was not invoked")
	}

	// 3. Test CreateAlphaRegionForwardingRule (Alpha Regional)
	var regionalAlphaInvoked bool
	mockGCE.MockAlphaForwardingRules.InsertHook = func(ctx context.Context, key *meta.Key, obj *computealpha.ForwardingRule, m *cloud.MockAlphaForwardingRules, options ...cloud.Option) (bool, error) {
		regionalAlphaInvoked = true
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("CreateAlphaRegionForwardingRule context has no deadline")
			return true, nil
		}
		duration := time.Until(deadline)
		if duration < 9*time.Minute || duration > 10*time.Minute {
			t.Errorf("CreateAlphaRegionForwardingRule timeout expected to be ~10m, got %v", duration)
		}
		return true, nil
	}

	regionalAlphaRule := &computealpha.ForwardingRule{Name: "regional-alpha-rule"}
	err = gce.CreateAlphaRegionForwardingRule(regionalAlphaRule, vals.Region)
	if err != nil {
		t.Errorf("CreateAlphaRegionForwardingRule failed: %v", err)
	}
	if !regionalAlphaInvoked {
		t.Error("CreateAlphaRegionForwardingRule hook was not invoked")
	}

	// 4. Test CreateBetaRegionForwardingRule (Beta Regional)
	var regionalBetaInvoked bool
	mockGCE.MockBetaForwardingRules.InsertHook = func(ctx context.Context, key *meta.Key, obj *computebeta.ForwardingRule, m *cloud.MockBetaForwardingRules, options ...cloud.Option) (bool, error) {
		regionalBetaInvoked = true
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("CreateBetaRegionForwardingRule context has no deadline")
			return true, nil
		}
		duration := time.Until(deadline)
		if duration < 9*time.Minute || duration > 10*time.Minute {
			t.Errorf("CreateBetaRegionForwardingRule timeout expected to be ~10m, got %v", duration)
		}
		return true, nil
	}

	regionalBetaRule := &computebeta.ForwardingRule{Name: "regional-beta-rule"}
	err = gce.CreateBetaRegionForwardingRule(regionalBetaRule, vals.Region)
	if err != nil {
		t.Errorf("CreateBetaRegionForwardingRule failed: %v", err)
	}
	if !regionalBetaInvoked {
		t.Error("CreateBetaRegionForwardingRule hook was not invoked")
	}
}
