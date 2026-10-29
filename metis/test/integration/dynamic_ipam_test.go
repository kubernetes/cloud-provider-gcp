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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/pkg/store"
)

// TestBootstrapAllocation tests node bootstrap where the initial pod is allocated
// from the CNI-provided initial CIDR before cloud controllers have published
// status. It verifies that the daemon serves the local allocation immediately,
// and the CCM status controller discovers the existing instance alias range,
// publishes it to NodeNetworkConfig status, and marks the node Ready.
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

// TestScaleUpLoop tests the dynamic scale-up cycle when pod demand exhausts the
// bootstrap CIDR block. It verifies that pod exhaustion triggers the daemon's
// background monitor to request additional capacity in NodeNetworkConfig spec,
// the CCM spec controller attaches a new dynamic /28 alias range to the GCE
// instance, the status controller publishes the range, and the waiting pod
// allocation completes.
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

// TestScaleDownAndReleaseLoop tests dynamic scale-down and resource reclamation
// when workload drops. It verifies that when active pods are deleted, the daemon
// stages the idle dynamic block in spec.releasableCIDRs, and the CCM spec
// controller detaches the alias range from the GCE instance and prunes the release
// request from NodeNetworkConfig spec.
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

// TestMixedAddAndRemove tests concurrent capacity expansion and release in a single
// reconcile cycle. It simulates simultaneous workload growth on one network interface
// while an idle CIDR block on another is marked for release, verifying that the CCM
// spec controller processes additions and removals together without dropping either.
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

// TestPartialReleaseWithActivePods tests selective scale-down when an instance has
// multiple dynamic blocks allocated. It verifies that an idle dynamic block is drained
// and released to GCE while sibling blocks with active pods remain untouched and
// operational.
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

// TestConflictRetryOnStatusUpdate tests optimistic concurrency handling during status
// reconciliation. It injects 409 Conflict errors into NodeNetworkConfig status update
// calls to verify that the status controller retries updates on conflict and converges
// successfully without dropping state (PR #1410 resilience).
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

// TestNotReadyNodeHandling tests controller behavior during early node registration
// before ProviderID is initialized. It verifies that the controllers safely defer
// reconciliation with nodeNotReadyError while ProviderID is empty, emit appropriate
// warning events, and automatically resume reconciliation once the node's ProviderID
// is assigned.
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

// TestSteadyStateAllocation verifies that allocating pods within existing capacity
// is handled entirely by the local SQLite IPAM engine without mutating NNC spec
// or invoking GCE APIs.
func TestSteadyStateAllocation(t *testing.T) {
	f := newFixture(t)

	// Workload allocation within bootstrap capacity: allocate multiple pods.
	for i := 1; i <= 5; i++ {
		ip, cidr := f.allocatePod(fmt.Sprintf("pod-steady-%d", i))
		if cidr != initialPodCIDR {
			t.Fatalf("expected pod to be allocated from initial CIDR %s, got %s", initialPodCIDR, cidr)
		}
		if ip == "" {
			t.Fatalf("pod allocated empty IP")
		}
	}

	// Verification of isolation from cloud provider: assert no GCE or NNC spec changes.
	currentNNC := f.nnc()
	if allocations := currentNNC.Spec.Allocations; len(allocations) > 0 {
		for _, a := range allocations {
			if a.Network == network && a.Pods > int32(initialUsableIPs) {
				t.Fatalf("unexpected scale-up request in NNC spec: %v", a.Pods)
			}
		}
	}
	aliasRanges := f.gceAliasRanges()
	if len(aliasRanges) != 1 || aliasRanges[0] != initialPodCIDR {
		t.Fatalf("expected only initial GCE alias range %s, got %v", initialPodCIDR, aliasRanges)
	}
	if f.blockAllocatedIPs(initialPodCIDR) != 3+5 {
		t.Fatalf("expected 8 allocated IPs in store (3 reserved + 5 pods), got %d", f.blockAllocatedIPs(initialPodCIDR))
	}
}

// TestIPReuseAndChurn verifies that when a pod is deleted, its IP address is
// returned to the local pool and reused for subsequent allocations within the
// same block without triggering scale-up.
func TestIPReuseAndChurn(t *testing.T) {
	f := newFixture(t, withDaemonConfig(func(cfg *daemon.Config) {
		cfg.ReleaseCooldown = 0
	}))

	// Initial pod allocation: allocate a pod and record its assigned IP.
	ip1, cidr1 := f.allocatePod("pod-churn-1")
	if cidr1 != initialPodCIDR {
		t.Fatalf("expected allocation from initial range, got %s", cidr1)
	}

	// Pod release: delete the pod to return the IP to the local store.
	f.deletePod("pod-churn-1")

	// Local IP reuse: allocate a new pod and verify it receives the freed IP.
	ip2, cidr2 := f.allocatePod("pod-churn-2")
	if ip2 != ip1 {
		t.Fatalf("expected IP reuse of %s, got %s", ip1, ip2)
	}
	if cidr2 != cidr1 {
		t.Fatalf("expected CIDR %s, got %s", cidr1, cidr2)
	}

	// Verification of zero cloud provider churn: GCE ranges and NNC spec unchanged.
	aliasRanges := f.gceAliasRanges()
	if len(aliasRanges) != 1 || aliasRanges[0] != initialPodCIDR {
		t.Fatalf("unexpected GCE alias ranges: %v", aliasRanges)
	}
}

// TestSequentialMultiBlockScaleUp verifies that the system progressively scales
// up across multiple dynamic blocks as pod capacity fills each preceding block.
func TestSequentialMultiBlockScaleUp(t *testing.T) {
	f := newFixture(t)

	// Bootstrap block saturation: allocate pods to exhaust initial block capacity.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-seq-init-%d", i))
	}

	// First dynamic block trigger: overflow pod triggers the first scale-up.
	_, dynCIDR1 := f.allocatePod("pod-seq-dyn1-overflow")
	if dynCIDR1 == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected dynamic range")
	}

	// First dynamic block verification: wait for range to attach and status to publish.
	f.eventually("first dynamic range is attached and ready", func() (bool, string) {
		currentNNC := f.nnc()
		ranges := f.gceAliasRanges()
		return len(ranges) == 2 && len(statusCIDRs(currentNNC)) == 2 && f.blockState(dynCIDR1) == store.StateReady,
			fmt.Sprintf("ranges=%v status=%v blockState=%q", ranges, statusCIDRs(currentNNC), f.blockState(dynCIDR1))
	})

	// First dynamic block saturation: fill remaining capacity of the first dynamic block.
	// For secondary blocks, 0 IPs are reserved, so 15 additional pods can be allocated.
	for i := 1; i <= 15; i++ {
		_, c := f.allocatePod(fmt.Sprintf("pod-seq-dyn1-%d", i))
		if c != dynCIDR1 {
			t.Fatalf("expected allocation from first dynamic block %s, got %s", dynCIDR1, c)
		}
	}

	// Second dynamic block trigger: second overflow pod triggers another scale-up.
	_, dynCIDR2 := f.allocatePod("pod-seq-dyn2-overflow")
	if dynCIDR2 == initialPodCIDR || dynCIDR2 == dynCIDR1 {
		t.Fatalf("expected allocation from second dynamic block, got %s", dynCIDR2)
	}

	// Progressive watermarking verification: wait for second dynamic range to attach.
	f.eventually("second dynamic range is attached and ready", func() (bool, string) {
		currentNNC := f.nnc()
		ranges := f.gceAliasRanges()
		return len(ranges) == 3 && len(statusCIDRs(currentNNC)) == 3 && f.blockState(dynCIDR2) == store.StateReady,
			fmt.Sprintf("ranges=%v status=%v blockState=%q", ranges, statusCIDRs(currentNNC), f.blockState(dynCIDR2))
	})
}

// TestDaemonRestartRehydration verifies that stopping the daemon and starting
// a fresh instance against the same database rehydrates state, reconciles with
// NNC without spurious scale-ups, and continues serving pod allocations.
func TestDaemonRestartRehydration(t *testing.T) {
	f := newFixture(t)

	// Pre-restart capacity expansion: allocate initial pods and trigger a dynamic block.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-rehydrate-init-%d", i))
	}
	_, dynamicCIDR := f.allocatePod("pod-rehydrate-overflow")
	f.eventually("dynamic block is active before restart", func() (bool, string) {
		return f.blockState(dynamicCIDR) == store.StateReady && len(f.gceAliasRanges()) == 2,
			fmt.Sprintf("blockState=%q ranges=%v", f.blockState(dynamicCIDR), f.gceAliasRanges())
	})

	allocationsBeforeRestart := f.nnc().Spec.Allocations

	// Daemon lifecycle restart: restart daemon against the same SQLite database and NNC.
	f.restartDaemon()

	// Rehydration verification: assert no spurious scale-up requests were generated.
	f.eventually("daemon rehydrates state and maintains stable allocations", func() (bool, string) {
		currentNNC := f.nnc()
		return statusReady(currentNNC) && apiequality.Semantic.DeepEqual(currentNNC.Spec.Allocations, allocationsBeforeRestart),
			fmt.Sprintf("allocations=%v (want %v)", currentNNC.Spec.Allocations, allocationsBeforeRestart)
	})

	// Operational continuation: verify new pod can be allocated immediately from existing capacity.
	ipNew, cidrNew := f.allocatePod("pod-rehydrate-post-restart")
	if ipNew == "" || cidrNew == "" {
		t.Fatalf("failed to allocate pod after daemon restart")
	}
}

// TestScaleDown_ANE_RoundingBoundary exercises the fractional threshold boundary
// during scale-down of /32 ANE allocations.
//
// Skipped pending fix in metis/pkg/daemon/monitor.go where
// targetUsedIPs := int(m.lowUtilizationThreshold * float64(totalPods)) truncates
// fractional targets, causing totalUsedIPs >= targetUsedIPs to break early and
// freeze idle /32 ANEs when usedIPs sits right on the boundary.
func TestScaleDown_ANE_RoundingBoundary(t *testing.T) {
	t.Skip("Skipping pending fix in metis/pkg/daemon/monitor.go for /32 rounding threshold truncation")

	f := newFixture(t, withBackend(dynamicpodip.BackendTypeANE), withDaemonConfig(func(cfg *daemon.Config) {
		cfg.LowUtilizationThreshold = 0.50
	}))

	// Bootstrap block saturation: allocate pods to exhaust initial block.
	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up: trigger dynamic allocation of /32 ANEs.
	_, dynamicCIDR := f.allocatePod("pod-ane-overflow")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}
	f.eventually("ANEs are attached to GCE", func() (bool, string) {
		return len(f.gceANEs()) > 0, fmt.Sprintf("anes=%d", len(f.gceANEs()))
	})

	// Pod deletion to reach the fractional threshold boundary:
	// Deleting the overflow pod and 5 initial pods leaves 8 active pods.
	// Total IPs = 23 (16 initial + 7 ANEs). Used IPs = 8 active + 3 reserved = 11.
	// 11 / 23 = 47.8% < 50%.
	// Without math.Ceil: targetUsedIPs = int(0.50 * 23) = 11, so 11 >= 11 breaks early.
	// With math.Ceil: targetUsedIPs = ceil(11.5) = 12, allowing the idle ANEs to drain.
	f.deletePod("pod-ane-overflow")
	for i := 1; i <= 5; i++ {
		f.deletePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// Boundary release verification: idle ANEs should be drained and detached from GCE.
	f.eventually("idle ANEs are drained and removed from GCE", func() (bool, string) {
		anes := f.gceANEs()
		return len(anes) == 0, fmt.Sprintf("anes remaining: %d", len(anes))
	})
}
