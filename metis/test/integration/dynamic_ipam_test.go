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

package integration

import (
	"fmt"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/pkg/store"
)

// TestBootstrapAllocation exercises Base Case 1: the first pod is served from
// the CNI-provided initial range before CCM has published anything, and CCM
// subsequently discovers and publishes the range with Ready=True.
func TestBootstrapAllocation(t *testing.T) {
	f := newFixture(t)

	// Bootstrap: allocate the first pod from the CNI-provided initial CIDR.
	_, cidr := f.allocatePod("pod-bootstrap")
	if cidr != initialPodCIDR {
		t.Fatalf("pod allocated from %s, want initial range %s", cidr, initialPodCIDR)
	}

	// Convergence: CCM discovers the initial range on the VM and publishes Ready status.
	f.eventually("CCM publishes initial range with Ready=True", func() (bool, string) {
		nnc := f.nnc()
		return equalSets(statusCIDRs(nnc), []string{initialPodCIDR}) && statusReady(nnc),
			fmt.Sprintf("status=%v ready=%v", statusCIDRs(nnc), statusReady(nnc))
	})

	// Store verification: daemon watcher sets the block state to Ready in SQLite.
	if got := f.blockState(initialPodCIDR); got != store.StateReady {
		t.Fatalf("initial block state after bootstrap = %q, want Ready", got)
	}
	if got := f.gceAliasRanges(); !equalSets(got, []string{initialPodCIDR}) {
		t.Fatalf("GCE alias ranges = %v, want exactly %v", got, []string{initialPodCIDR})
	}
}

// TestScaleUpLoop exercises Base Case 2: pod allocations exhaust the initial
// block, the daemon requests more capacity in NNC spec, the CCM spec controller
// attaches a new alias range in GCE, the CCM status controller publishes it,
// and the daemon watcher activates the new block to serve the overflow pod.
func TestScaleUpLoop(t *testing.T) {
	f := newFixture(t)

	// Initial saturation: allocate pods until the initial block is fully exhausted.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-init-%d", i))
	}

	// Scale-up demand: an overflow pod triggers the daemon to request additional capacity.
	_, dynamicCIDR := f.allocatePod("pod-overflow")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected a dynamically added range")
	}

	// Infrastructure expansion: GCE, status, and SQLite converge on the expanded capacity.
	f.eventually("GCE, status, and SQLite converge on two ranges", func() (bool, string) {
		nnc := f.nnc()
		want := []string{initialPodCIDR, dynamicCIDR}
		ok := equalSets(f.gceAliasRanges(), want) &&
			equalSets(statusCIDRs(nnc), want) && statusReady(nnc) &&
			f.blockState(dynamicCIDR) == store.StateReady
		return ok, fmt.Sprintf("gce=%v status=%v ready=%v block=%q", f.gceAliasRanges(), statusCIDRs(nnc), statusReady(nnc), f.blockState(dynamicCIDR))
	})

	if u := f.usage(); u.Total != 32 {
		t.Fatalf("usage.Total after scale-up = %d, want 32", u.Total)
	}
}

// TestScaleDownAndReleaseLoop exercises Base Case 3: pods terminate, an idle
// block is marked in spec.releasableCIDRs, the CCM spec controller detaches the
// alias range from GCE, prunes releasableCIDRs, and the status controller reflects
// only the remaining ranges.
func TestScaleDownAndReleaseLoop(t *testing.T) {
	f := newFixture(t)

	// Capacity expansion: scale up to two blocks so an expansion block exists.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-scale-%d", i))
	}
	_, dynamicCIDR := f.allocatePod("pod-drain-target")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected a dynamically added range")
	}

	f.eventually("dynamic block is active in GCE and status", func() (bool, string) {
		nnc := f.nnc()
		want := []string{initialPodCIDR, dynamicCIDR}
		return equalSets(f.gceAliasRanges(), want) && equalSets(statusCIDRs(nnc), want) && statusReady(nnc),
			fmt.Sprintf("gce=%v status=%v", f.gceAliasRanges(), statusCIDRs(nnc))
	})

	// Workload reduction: delete all pods so utilization drops and idle capacity can be released.
	for i := 1; i <= initialUsableIPs; i++ {
		f.deletePod(fmt.Sprintf("pod-scale-%d", i))
	}
	f.deletePod("pod-drain-target")

	// Drain and release initiation: mark the idle block for release and reduce desired pods.
	f.setAllocationsPods(16)
	f.addReleasableCIDR(dynamicCIDR)

	// Infrastructure release: CCM detaches the released alias range from GCE VM.
	f.eventually("GCE instance removes the released alias range", func() (bool, string) {
		ranges := f.gceAliasRanges()
		return equalSets(ranges, []string{initialPodCIDR}), fmt.Sprintf("gce=%v", ranges)
	})

	// Spec hygiene: CCM spec controller prunes the released range from spec.releasableCIDRs.
	f.eventually("CCM prunes releasableCIDRs from NNC spec", func() (bool, string) {
		rel := releasableCIDRs(f.nnc())
		return len(rel) == 0, fmt.Sprintf("releasableCIDRs=%v", rel)
	})

	// Status reconciliation: CCM status controller updates status to reflect only the retained block.
	f.eventually("status reflects only the retained initial range with Ready=True", func() (bool, string) {
		nnc := f.nnc()
		return equalSets(statusCIDRs(nnc), []string{initialPodCIDR}) && statusReady(nnc),
			fmt.Sprintf("status=%v ready=%v", statusCIDRs(nnc), statusReady(nnc))
	})
}

// TestMixedAddAndRemove exercises the Edge Case where a single reconcile pass
// handles both an allocation expansion and a CIDR release concurrently.
func TestMixedAddAndRemove(t *testing.T) {
	f := newFixture(t)

	// Baseline provisioning: establish two blocks via scale-up.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-mixed-%d", i))
	}
	_, dynamicCIDR := f.allocatePod("pod-to-release")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("expected dynamically added range")
	}

	f.eventually("baseline two blocks established in GCE", func() (bool, string) {
		return len(f.gceAliasRanges()) == 2, fmt.Sprintf("ranges=%v", f.gceAliasRanges())
	})

	// Pod terminates on the dynamic block.
	f.deletePod("pod-to-release")

	// Simultaneous scale-up and release: release dynamicCIDR while requesting 32 pods.
	// Since dynamicCIDR is marked releasable, its capacity does not satisfy demand,
	// prompting CCM to execute removal of dynamicCIDR and addition of a replacement block.
	nnc := f.nnc().DeepCopy()
	nnc.Spec.Allocations[0].Pods = 32
	nnc.Spec.ReleasableCIDRs = []nncv1.PodCIDR{
		{Network: network, CIDR: dynamicCIDR},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Update(f.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to update NNC for mixed add/remove: %v", err)
	}

	// Reconcile execution: CCM removes dynamicCIDR, adds a new block, and prunes releasableCIDRs.
	f.eventually("releasable CIDR is pruned and replacement range is attached", func() (bool, string) {
		currentNNC := f.nnc()
		ranges := f.gceAliasRanges()
		releasables := releasableCIDRs(currentNNC)
		hasOld := false
		for _, r := range ranges {
			if r == dynamicCIDR {
				hasOld = true
			}
		}
		ok := len(releasables) == 0 && len(ranges) == 2 && !hasOld && statusReady(currentNNC)
		return ok, fmt.Sprintf("releasables=%v ranges=%v statusReady=%v", releasables, ranges, statusReady(currentNNC))
	})

	// Status confirmation: status reflects two active ranges, excluding the released range.
	finalNNC := f.nnc()
	if equalSets(statusCIDRs(finalNNC), []string{initialPodCIDR, dynamicCIDR}) {
		t.Fatalf("status still contains released CIDR %s", dynamicCIDR)
	}
	if len(statusCIDRs(finalNNC)) != 2 {
		t.Fatalf("expected 2 CIDRs in status, got %d: %v", len(statusCIDRs(finalNNC)), statusCIDRs(finalNNC))
	}
}

// TestPartialReleaseWithActivePods exercises the Edge Case where multiple dynamic
// blocks exist, but only an idle block is released while blocks with running pods
// remain active and untouched.
func TestPartialReleaseWithActivePods(t *testing.T) {
	f := newFixture(t)

	// Multi-block expansion: exhaust initial block and expand into two dynamic blocks.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-partial-init-%d", i))
	}
	_, dynamicBlock1 := f.allocatePod("pod-on-block-1")
	for i := 1; i <= 15; i++ {
		f.allocatePod(fmt.Sprintf("pod-block1-filler-%d", i))
	}
	_, dynamicBlock2 := f.allocatePod("pod-on-block-2")

	if dynamicBlock1 == dynamicBlock2 || dynamicBlock1 == initialPodCIDR || dynamicBlock2 == initialPodCIDR {
		t.Fatalf("expected distinct blocks: init=%s b1=%s b2=%s", initialPodCIDR, dynamicBlock1, dynamicBlock2)
	}

	f.eventually("three distinct ranges active in GCE and status", func() (bool, string) {
		ranges := f.gceAliasRanges()
		return len(ranges) == 3, fmt.Sprintf("ranges=%v", ranges)
	})

	// Targeted pod deletion: delete only the pod on the second dynamic block, leaving the first active.
	f.deletePod("pod-on-block-2")

	// Partial release: release only dynamicBlock2.
	f.setAllocationsPods(32)
	f.addReleasableCIDR(dynamicBlock2)

	// Selective removal: CCM removes only dynamicBlock2 and preserves dynamicBlock1 and initial block.
	f.eventually("only dynamicBlock2 is removed; dynamicBlock1 and initial block are preserved", func() (bool, string) {
		ranges := f.gceAliasRanges()
		currentNNC := f.nnc()
		want := []string{initialPodCIDR, dynamicBlock1}
		ok := equalSets(ranges, want) && equalSets(statusCIDRs(currentNNC), want) && len(releasableCIDRs(currentNNC)) == 0
		return ok, fmt.Sprintf("gce=%v status=%v releasable=%v", ranges, statusCIDRs(currentNNC), releasableCIDRs(currentNNC))
	})
}

// TestConflictRetryOnStatusUpdate exercises the Edge Case where a 409 Conflict
// is returned during status synchronization (PR #1410). The status update retry
// must re-fetch live NNC and converge without dropping state.
func TestConflictRetryOnStatusUpdate(t *testing.T) {
	conflictInjected := false
	conflictResolved := false

	// Conflict injection: fail the first status write with 409 Conflict.
	f := newFixture(t, withPrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if !conflictInjected {
			conflictInjected = true
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"},
				nodeName, fmt.Errorf("simulated status conflict error"))
		}
		conflictResolved = true
		return false, nil, nil
	}))

	// Initial pod allocation triggers status reconciliation.
	f.allocatePod("pod-conflict-test")

	// Reconcile resilience: status controller retries on conflict and completes status publish.
	f.eventually("status publish succeeds despite initial 409 conflict", func() (bool, string) {
		nnc := f.nnc()
		return conflictResolved && statusReady(nnc) && len(statusCIDRs(nnc)) == 1,
			fmt.Sprintf("conflictResolved=%v ready=%v status=%v", conflictResolved, statusReady(nnc), statusCIDRs(nnc))
	})
}

// TestNotReadyNodeHandling exercises the Edge Case where a NodeNetworkConfig exists
// but the backing Node is uninitialized (empty ProviderID). The controllers must
// defer reconciliation with nodeNotReadyError until ProviderID is set.
func TestNotReadyNodeHandling(t *testing.T) {
	// Uninitialized node startup: launch controllers with a node lacking ProviderID.
	f := newFixture(t, withNodeProviderID(""))

	// Initial check: status is not yet ready because node has empty ProviderID.
	time.Sleep(500 * time.Millisecond)
	nnc := f.nnc()
	if statusReady(nnc) {
		t.Fatalf("expected status not ready while node is uninitialized")
	}

	// Node readiness: cloud-node-controller assigns ProviderID to the Node.
	f.updateNodeProviderID(providerID)

	// Successful reconciliation: controllers proceed with reconciliation once Node is ready.
	f.eventually("CCM publishes Ready status after ProviderID is set", func() (bool, string) {
		currentNNC := f.nnc()
		return statusReady(currentNNC) && equalSets(statusCIDRs(currentNNC), []string{initialPodCIDR}),
			fmt.Sprintf("ready=%v status=%v", statusReady(currentNNC), statusCIDRs(currentNNC))
	})
}

// TestScaleUp_ANE exercises dynamic scale-up using the Alias Network Endpoint
// (ANE) backend. The daemon does not know about ANEs specifically, but receives
// them as /32 CIDR allocations with a NonReusable policy.
func TestScaleUp_ANE(t *testing.T) {
	f := newFixture(t, withBackend(dynamicpodip.BackendTypeANE))

	// Bootstrap: allocate pods to exhaust the CNI-provided initial block.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up demand: an overflow pod triggers the daemon to request additional capacity.
	_, dynamicCIDR := f.allocatePod("pod-ane-overflow")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}

	// Verification of ANE allocation: GCE creates /32 ANE, status publishes /32 with NonReusable policy.
	f.eventually("GCE creates ANE and status publishes /32 with NonReusable policy", func() (bool, string) {
		currentNNC := f.nnc()
		anes := f.gceANEs()
		hasANE := len(anes) > 0
		hasStatus32 := false
		for _, pc := range currentNNC.Status.PodCIDRs {
			if pc.CIDR == dynamicCIDR && pc.ReusePolicy == nncv1.PodCIDRTypeNonReusable {
				hasStatus32 = true
				break
			}
		}
		return hasANE && hasStatus32 && statusReady(currentNNC) && f.blockState(dynamicCIDR) == store.StateReady,
			fmt.Sprintf("anes=%d hasStatus32=%v ready=%v blockState=%q", len(anes), hasStatus32, statusReady(currentNNC), f.blockState(dynamicCIDR))
	})
}

// TestScaleDown_ANE exercises dynamic scale-down and release of /32 NonReusable
// allocations. Skipped pending follow-up PR for /32 scale-down rounding and pruning stabilization.
func TestScaleDown_ANE(t *testing.T) {
	t.Skip("Skipping ANE /32 scale-down test pending follow-up PR for /32 rounding and pruning stabilization")

	f := newFixture(t, withBackend(dynamicpodip.BackendTypeANE))

	// Bootstrap: allocate pods to exhaust the CNI-provided initial block.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up demand: an overflow pod triggers the daemon to request additional capacity.
	_, dynamicCIDR := f.allocatePod("pod-ane-overflow")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}

	// Workload reduction: delete all pods to trigger scale-down of the idle /32 ANE.
	for i := 1; i <= initialUsableIPs; i++ {
		f.deletePod(fmt.Sprintf("pod-ane-init-%d", i))
	}
	f.deletePod("pod-ane-overflow")

	// Infrastructure release: CCM detaches the ANE from GCE and prunes releasableCIDRs.
	f.eventually("GCE removes ANE and CCM prunes releasableCIDRs", func() (bool, string) {
		anes := f.gceANEs()
		currentNNC := f.nnc()
		rel := releasableCIDRs(currentNNC)
		return len(anes) == 0 && len(rel) == 0 && equalSets(statusCIDRs(currentNNC), []string{initialPodCIDR}),
			fmt.Sprintf("anes=%d releasable=%v status=%v", len(anes), rel, statusCIDRs(currentNNC))
	})
}
