/*
Copyright 2025 The Kubernetes Authors.

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
	"strings"
	"sync"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/mock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

const (
	fakeDenyFirewallName        = "k8s-fw-a-deny"
	fakeNodeFirewallName        = "k8s-fw-a"
	fakeHealthCheckFirewallName = "k8s-test-cluster-id-node-http-hc"
)

// firewallTracker is used to check if there are multiple firewalls
// on the same IP or IP range that have conflicting priority, which
// would result in blocking all traffic.
type firewallTracker struct {
	// firewalls contains all firewalls for IP specified in the key
	// for IPv6 ranges we store just the prefix
	firewalls map[ipPrefix]map[resourceName]*compute.Firewall

	mu sync.Mutex
}

type (
	ipPrefix     string
	resourceName string
	firewallMap  map[ipPrefix]map[resourceName]*compute.Firewall
)

// patch will return an error if there is a situation that modifying fw
// would cause blocking traffic - allow overruled by deny firewall.
func (f *firewallTracker) patch(fw *compute.Firewall) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.firewalls == nil {
		f.firewalls = make(firewallMap)
	}

	if len(fw.DestinationRanges) < 1 { // Does not concern us - likely for healthcheck
		return nil
	}

	if len(fw.DestinationRanges) > 1 {
		return fmt.Errorf("unexpected count of destination ranges, expected at most 1: %v", fw.DestinationRanges)
	}

	cidrOrIP := fw.DestinationRanges[0]
	key := ipPrefix(strings.TrimSuffix(cidrOrIP, "/96"))

	if f.firewalls[key] == nil {
		f.firewalls[key] = make(map[resourceName]*compute.Firewall)
	}
	rName := resourceName(fw.Name)
	f.firewalls[key][rName] = fw

	for _, other := range f.firewalls[key] {
		if fw.Name == other.Name {
			continue
		}
		if areBlocked(fw, other) {
			return fmt.Errorf(
				"two firewalls block each other on %q: %s (priority %d) and %s (priority %d)",
				key, fw.Name, fw.Priority, other.Name, other.Priority,
			)
		}
	}

	return nil
}

func (f *firewallTracker) delete(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.firewalls == nil {
		return
	}

	// this could be done a tad quicker with an additional map
	// but this should be fast enough for the test
	for _, fw := range f.firewalls {
		delete(fw, resourceName(name))
	}
}

func (f *firewallTracker) hookTo(mockGCE *cloud.MockGCE) {
	mockGCE.MockFirewalls.InsertHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) (bool, error) {
		if err := f.patch(obj); err != nil {
			return true, err
		}
		return false, nil
	}
	mockGCE.MockFirewalls.UpdateHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) error {
		if err := f.patch(obj); err != nil {
			return err
		}
		return mock.UpdateFirewallHook(ctx, key, obj, m, options...)
	}
	mockGCE.MockFirewalls.PatchHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) error {
		if err := f.patch(obj); err != nil {
			return err
		}
		return mock.UpdateFirewallHook(ctx, key, obj, m, options...)
	}
	mockGCE.MockFirewalls.DeleteHook = func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, error) {
		f.delete(key.Name)
		return false, nil
	}
}

// areBlocked only works if fw1 and fw2 are using the same
// destination range, direction, etc
func areBlocked(fw1, fw2 *compute.Firewall) bool {
	if fw1 == nil || fw2 == nil {
		return false
	}

	if len(fw2.Denied) > 0 {
		fw1, fw2 = fw2, fw1
	}

	// Both are deny or allow - won't block themselves
	if len(fw1.Denied) == 0 || len(fw2.Allowed) == 0 {
		return false
	}

	// deny takes precedence over allow if they have the same priority
	return fw1.Priority <= fw2.Priority
}

func TestDenyFirewall(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Fatalf("fakeGCECloud error: %v", err)
	}

	// Hook firewall tracker which will throw errors on mockGCE calls when firewalls are blocking each other
	tracker := &firewallTracker{}
	mockGCE := gce.Compute().(*cloud.MockGCE)
	tracker.hookTo(mockGCE)

	// Create service and nodes
	svc := fakeLoadbalancerService("")
	nodeName := "test-node-1"
	nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	if err != nil {
		t.Fatalf("createAndInsertNodes error: %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure with deny enabled
	gce.enableL4DenyFirewallRule = true
	gce.enableL4DenyFirewallRollbackCleanup = true

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	if err != nil {
		t.Fatalf("ensureExternalLoadBalancer(deny=false) error: %v", err)
	}

	// Verify health check firewall exists at 999
	fw, err := gce.GetFirewall(fakeHealthCheckFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeHealthCheckFirewallName, err)
	}
	if fw.Priority != 999 {
		t.Errorf("Allow firewall priority = %d, want 999", fw.Priority)
	}

	wantDestinationRange := []string{"1.2.3.0"}

	// Verify node firewall exists at 999
	fw, err = gce.GetFirewall(fakeNodeFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeNodeFirewallName, err)
	}
	if fw.Priority != 999 {
		t.Errorf("Allow firewall priority = %d, want 999", fw.Priority)
	}
	if diff := cmp.Diff(wantDestinationRange, fw.DestinationRanges); diff != "" {
		t.Errorf("allow destination range got != want (-want, +got)\n%s", diff)
	}

	// Verify deny firewall
	fw, err = gce.GetFirewall(fakeDenyFirewallName)
	if err != nil {
		t.Errorf("GetFirewall(%q) error: %v", fakeDenyFirewallName, err)
	}
	want := &compute.Firewall{
		Name:              fakeDenyFirewallName,
		Denied:            []*compute.FirewallDenied{{IPProtocol: "all"}},
		Description:       `{"kubernetes.io/service-name":"/fakesvc", "kubernetes.io/service-ip":"1.2.3.0"}`,
		DestinationRanges: wantDestinationRange,
		SourceRanges:      []string{"0.0.0.0/0"},
		TargetTags:        []string{nodeName},
		Priority:          1000,
	}
	fwCmpOpt := cmpopts.IgnoreFields(compute.Firewall{}, "SelfLink")
	if diff := cmp.Diff(want, fw, fwCmpOpt); diff != "" {
		t.Errorf("deny firewalls got != want (-want, +got)\n%s", diff)
	}
}

func TestDenyRollforwardDoesNotBlockTraffic(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Fatalf("fakeGCECloud error: %v", err)
	}

	// Hook firewall tracker which will throw errors on mockGCE calls when firewalls are blocking each other
	tracker := &firewallTracker{}
	mockGCE := gce.Compute().(*cloud.MockGCE)
	tracker.hookTo(mockGCE)

	// Create service and nodes
	svc := fakeLoadbalancerService("")
	nodeName := "test-node-1"
	nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	if err != nil {
		t.Fatalf("createAndInsertNodes error: %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Ensure with Deny Disabled
	gce.enableL4DenyFirewallRule = false
	gce.enableL4DenyFirewallRollbackCleanup = true

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	if err != nil {
		t.Fatalf("ensureExternalLoadBalancer(deny=false) error: %v", err)
	}

	// Verify Allow exists at 1000
	fw, err := gce.GetFirewall(fakeNodeFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeNodeFirewallName, err)
	}
	if fw.Priority != 1000 {
		t.Errorf("Allow firewall priority = %d, want 1000", fw.Priority)
	}

	// Verify Health Check firewall exists at 1000
	fw, err = gce.GetFirewall(fakeHealthCheckFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeHealthCheckFirewallName, err)
	}
	if fw.Priority != 1000 {
		t.Errorf("Allow firewall priority = %d, want 1000", fw.Priority)
	}

	// Verify Deny does not exist
	_, err = gce.GetFirewall(fakeDenyFirewallName)
	if !isNotFound(err) {
		t.Errorf("Deny firewall %q should not exist, err: %v", fakeDenyFirewallName, err)
	}

	// 2. Ensure with Deny Enabled (Rollforward)
	gce.enableL4DenyFirewallRule = true

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	if err != nil {
		t.Fatalf("ensureExternalLoadBalancer(deny=true) error: %v", err)
	}

	// Verify Allow exists at 999
	fw, err = gce.GetFirewall(fakeNodeFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeNodeFirewallName, err)
	}
	if fw.Priority != 999 {
		t.Errorf("Allow firewall priority = %d, want 999", fw.Priority)
	}

	// Verify Healthcheck Firewall exists at 999
	fwHealthcheck, err := gce.GetFirewall(fakeHealthCheckFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeHealthCheckFirewallName, err)
	}
	if fwHealthcheck.Priority != 999 {
		t.Errorf("Healthcheck firewall priority = %d, want 999", fwHealthcheck.Priority)
	}

	// Verify Deny exists at 1000
	fwDeny, err := gce.GetFirewall(fakeDenyFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeDenyFirewallName, err)
	}
	if fwDeny.Priority != 1000 {
		t.Errorf("Deny firewall priority = %d, want 1000", fwDeny.Priority)
	}

	// 3. Delete service
	err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	if err != nil {
		t.Fatal(err)
	}

	// Verify firewalls are cleaned up
	for _, fwName := range []string{fakeNodeFirewallName, fakeHealthCheckFirewallName, fakeDenyFirewallName} {
		got, err := gce.GetFirewall(fwName)
		if got != nil {
			t.Errorf("firewall %v wasn't deleted after delete service", fwName)
		}
		if !isNotFound(err) {
			t.Errorf("got unexpected err %v when checking for deleted %v firewall", err, fwName)
		}
	}
}

func TestDenyRollback(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Fatalf("fakeGCECloud error: %v", err)
	}
	gce.eventRecorder = record.NewFakeRecorder(1024)

	// Hook firewall tracker which will throw errors on mockGCE calls when firewalls are blocking each other
	tracker := &firewallTracker{}
	mockGCE := gce.Compute().(*cloud.MockGCE)
	tracker.hookTo(mockGCE)

	// Create service and nodes
	svc := fakeLoadbalancerService("")
	nodeName := "test-node-1"
	nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	if err != nil {
		t.Fatalf("createAndInsertNodes error: %v", err)
	}
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Ensure with Deny Enabled
	gce.enableL4DenyFirewallRule = true
	gce.enableL4DenyFirewallRollbackCleanup = true

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	if err != nil {
		t.Fatalf("ensureExternalLoadBalancer(deny=true) error: %v", err)
	}

	// 2. Ensure with Deny Disabled (Rollback)
	gce.enableL4DenyFirewallRule = false

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	if err != nil {
		t.Fatalf("ensureExternalLoadBalancer(deny=false) error: %v", err)
	}

	// Verify Allow exists at 1000
	fw, err := gce.GetFirewall(fakeNodeFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeNodeFirewallName, err)
	}
	if fw.Priority != 1000 {
		t.Errorf("Allow firewall priority = %d, want 1000", fw.Priority)
	}

	// Verify Health Check firewall exists at 1000
	fw, err = gce.GetFirewall(fakeHealthCheckFirewallName)
	if err != nil {
		t.Fatalf("GetFirewall(%q) error: %v", fakeNodeFirewallName, err)
	}
	if fw.Priority != 1000 {
		t.Errorf("Health check firewall priority = %d, want 1000", fw.Priority)
	}

	// Verify Deny does not exist
	_, err = gce.GetFirewall(fakeDenyFirewallName)
	if !isNotFound(err) {
		t.Errorf("Deny firewall %q should not exist, err: %v", fakeDenyFirewallName, err)
	}
}

func TestDenyIsNotCreatedWhenPriorityUpdateFails(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		firewallNameToFail string
	}{
		{
			name:               "node_firewall",
			firewallNameToFail: fakeNodeFirewallName,
		},
		{
			name:               "healthcheck_firewall",
			firewallNameToFail: fakeHealthCheckFirewallName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			// Setup
			vals := DefaultTestClusterValues()
			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud error: %v", err)
			}

			svc := fakeLoadbalancerService("")
			nodeName := "test-node-1"
			nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes error: %v", err)
			}
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}

			// 1. Ensure with Deny Disabled just to provision firewalls with 1000 priority
			gce.enableL4DenyFirewallRule = false
			gce.enableL4DenyFirewallRollbackCleanup = false

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
			if err != nil {
				t.Fatalf("ensureExternalLoadBalancer(deny=false) error: %v", err)
			}

			// 2. Inject error on Allow update
			mockGCE := gce.Compute().(*cloud.MockGCE)
			injectedError := errors.New("injected error on allow patch")

			mockGCE.MockFirewalls.PatchHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) error {
				if key.Name == tc.firewallNameToFail {
					return injectedError
				}
				return mock.UpdateFirewallHook(ctx, key, obj, m, options...)
			}
			mockGCE.MockFirewalls.UpdateHook = mockGCE.MockFirewalls.PatchHook

			// 3. Ensure with Deny Enabled to force priority decrease
			gce.enableL4DenyFirewallRule = true
			gce.enableL4DenyFirewallRollbackCleanup = true

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)

			// Assert error returned
			if err == nil || !strings.Contains(err.Error(), injectedError.Error()) {
				t.Errorf("got unexpected err %q, wanted %q", err, injectedError)
			}

			// Assert Deny rule NOT created
			_, err = gce.GetFirewall(fakeDenyFirewallName)
			if !isNotFound(err) {
				t.Errorf("Deny firewall %q should not exist after failure to update %q rule", fakeDenyFirewallName, tc.firewallNameToFail)
			}
		})
	}
}

// TestContinueOnXPN403s verifies that we don't error out on XPN (shared VPC) clusters that don't have permissions to create firewalls
func TestContinueOnXPN403s(t *testing.T) {
	testCases := []struct {
		name                       string
		denyFirewallEnabled        bool
		denyFirewallCleanupEnabled bool
	}{
		{
			name: "disabled",
		},
		{
			name:                "rolled_back",
			denyFirewallEnabled: true,
		},
		{
			name:                       "enabled",
			denyFirewallEnabled:        true,
			denyFirewallCleanupEnabled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			vals := DefaultTestClusterValues()
			vals.OnXPN = true
			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud error: %v", err)
			}

			gce.enableL4DenyFirewallRule = tc.denyFirewallEnabled
			gce.enableL4DenyFirewallRollbackCleanup = tc.denyFirewallCleanupEnabled

			svc := fakeLoadbalancerService("")
			nodeName := "test-node-1"
			nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes error: %v", err)
			}
			ctx := t.Context()
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}

			xpnErr := func(call, name string) *googleapi.Error {
				return &googleapi.Error{
					Code:    http.StatusForbidden,
					Message: fmt.Sprintf("Required 'compute.firewalls.%s' permission for 'projects/something/global/firewalls/%s'.", call, name),
				}
			}

			mockGCE := gce.Compute().(*cloud.MockGCE)
			mockGCE.MockFirewalls.InsertHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) (bool, error) {
				return true, xpnErr("insert", key.Name)
			}
			mockGCE.MockFirewalls.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, *compute.Firewall, error) {
				return false, nil, nil // For some reason get doesn't return 403
			}
			mockGCE.MockFirewalls.DeleteHook = func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, error) {
				return true, xpnErr("delete", key.Name)
			}
			mockGCE.MockFirewalls.PatchHook = func(ctx context.Context, key *meta.Key, obj *compute.Firewall, m *cloud.MockFirewalls, options ...cloud.Option) error {
				return xpnErr("patch", key.Name)
			}
			mockGCE.MockFirewalls.UpdateHook = mockGCE.MockFirewalls.PatchHook

			// Act: create load balancer
			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)

			// Assert: we don't expect any errors to be returned
			if err != nil {
				t.Fatal(err)
			}

			// Assert: no firewalls were created
			fwNames := []string{"k8s-fw-a", "k8s-fw-a-deny"}
			for _, name := range fwNames {
				fw, _ := gce.GetFirewall(name)
				if fw != nil {
					t.Errorf("something is wrong with the test logic, the firewall %v should not have been created", name)
				}
			}

			// Assert: forwarding rule exists either way
			forwardingRuleName := "a"
			_, err = gce.GetRegionForwardingRule(forwardingRuleName, gce.Region())
			if err != nil {
				t.Errorf("something is wrong with the test logic, the forwarding rule %v should have been created, but got %v", forwardingRuleName, err)
			}

			// Act: delete the service
			err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
			if err != nil {
				t.Fatal(err)
			}

			// Assert: forwarding rules were cleaned up
			fr, err := gce.GetRegionForwardingRule(forwardingRuleName, gce.Region())
			if !isNotFound(err) || fr != nil {
				t.Errorf("something is wrong with the test logic, the forwarding rule %v should have been cleaned up, but got err: %v and fw: %v", forwardingRuleName, err, fr)
			}
		})
	}
}
