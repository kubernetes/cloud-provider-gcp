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
	"strings"
	"testing"
	"time"

	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/pkg/store"
)

// TestOutOfBandCIDRDeletion walks a node from bootstrap through a dynamic
// scale-up, then deletes the dynamically added range straight from GCE and
// verifies that the daemon notices via NodeNetworkConfig status, stops using
// the range, does not enter a scale-up feedback loop, and cleans the block up
// once the pod on it exits -- all without ever touching spec.releasableCIDRs.
func TestOutOfBandCIDRDeletion(t *testing.T) {
	f := newFixture(t)

	// Bootstrap: the first pod is served from the CNI-provided initial range
	// before CCM has published anything; the reconciler must not treat that as
	// out of sync.
	_, cidr := f.allocatePod("pod-1")
	if cidr != initialPodCIDR {
		t.Fatalf("pod-1 allocated from %s, want initial range %s", cidr, initialPodCIDR)
	}
	f.eventually("CCM publishes the initial range with Ready=True", func() (bool, string) {
		nnc := f.nnc()
		return equalSets(statusCIDRs(nnc), []string{initialPodCIDR}) && statusReady(nnc),
			fmt.Sprintf("status=%v ready=%v", statusCIDRs(nnc), statusReady(nnc))
	})
	if got := f.blockState(initialPodCIDR); got != store.StateReady {
		t.Fatalf("initial block state after bootstrap = %q, want Ready", got)
	}

	// Dynamic scale-up: exhaust the initial block. The next allocation blocks
	// inside the daemon until the full loop (spec -> CCM -> GCE -> status ->
	// watcher) delivers a new range.
	for i := 2; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-%d", i))
	}
	_, dynamicCIDR := f.allocatePod("pod-overflow")
	if dynamicCIDR == initialPodCIDR {
		t.Fatalf("overflow pod was allocated from the initial range; expected a dynamically added range")
	}
	f.eventually("GCE, status and SQLite converge on two ranges", func() (bool, string) {
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
	podsAfterScaleUp := f.settledSpecPods()

	// Out-of-band deletion.
	f.removeAliasRangeOutOfBand(dynamicCIDR)

	// The daemon learns of the loss from NodeNetworkConfig status. Whether or
	// not the test happens to observe the intermediate single-range status, the
	// lost range is never published again, so the block must end up Unknown.
	f.eventually("daemon marks the block Unknown", func() (bool, string) {
		st := f.blockState(dynamicCIDR)
		return st == store.StateUnknown, fmt.Sprintf("state=%q", st)
	})

	// CCM publishes the shrunken status, which re-runs its spec controller:
	// spec.allocations.pods still exceeds what the VM holds, so a fresh range is
	// attached -- never the deleted one -- and everything converges on it.
	var restoredCIDR string
	f.eventually("CCM restores capacity with a fresh range", func() (bool, string) {
		gceRanges := f.gceAliasRanges()
		nnc := f.nnc()
		if len(gceRanges) != 2 || !statusReady(nnc) || !equalSets(statusCIDRs(nnc), gceRanges) {
			return false, fmt.Sprintf("gce=%v status=%v ready=%v", gceRanges, statusCIDRs(nnc), statusReady(nnc))
		}
		for _, cidr := range gceRanges {
			if cidr != initialPodCIDR {
				restoredCIDR = cidr
			}
		}
		return restoredCIDR != "" && restoredCIDR != dynamicCIDR && f.blockState(restoredCIDR) == store.StateReady,
			fmt.Sprintf("gce=%v restored=%q block=%q", gceRanges, restoredCIDR, f.blockState(restoredCIDR))
	})
	f.eventually("the loss is reported as a Warning event on the Node", func() (bool, string) {
		events := f.nodeEvents(dynamicpodip.PodCIDRRemovedOutOfBandReason)
		return len(events) == 1 && strings.Contains(events[0], dynamicCIDR), fmt.Sprintf("events=%v", events)
	})

	// The lost range never counts towards capacity (only the initial and the
	// restored block do)...
	if u := f.usage(); u.Total != 32 {
		t.Errorf("usage.Total after restoration = %d, want 32 (initial + restored block, never the lost one)", u.Total)
	}
	// ...the daemon does not spin asking CCM for more capacity because of it...
	f.consistently("spec.allocations.pods is stable after the loss", 6*monitorInterval, func() (bool, string) {
		pods := specPods(f.nnc())
		return pods == podsAfterScaleUp, fmt.Sprintf("pods=%d, was %d after scale-up", pods, podsAfterScaleUp)
	})
	// ...and the range was never offered back to CCM as releasable.
	if rel := releasableCIDRs(f.nnc()); len(rel) != 0 {
		t.Errorf("spec.releasableCIDRs = %v, want empty: an out-of-band deletion must not be double-released", rel)
	}
	if got := f.gceAliasRanges(); !equalSets(got, []string{initialPodCIDR, restoredCIDR}) {
		t.Errorf("GCE alias ranges = %v, want exactly %v", got, []string{initialPodCIDR, restoredCIDR})
	}

	// The pod on the lost range exits.
	f.deletePod("pod-overflow")
	f.eventually("Unknown block is removed from SQLite once empty", func() (bool, string) {
		return !f.blockExists(dynamicCIDR), fmt.Sprintf("exists=%v state=%q", f.blockExists(dynamicCIDR), f.blockState(dynamicCIDR))
	})
	if rel := releasableCIDRs(f.nnc()); len(rel) != 0 {
		t.Errorf("spec.releasableCIDRs = %v after cleanup, want empty", rel)
	}
	for _, cidr := range []string{initialPodCIDR, restoredCIDR} {
		if got := f.blockState(cidr); got != store.StateReady {
			t.Errorf("block %s state = %q, want Ready: the loss must not disturb healthy blocks", cidr, got)
		}
	}
}

// TestOutOfBandCIDRDeletion_CapacityIsRestored checks what happens to the
// *next* pod after a range is lost out of band: the node is under-provisioned
// relative to spec.allocations.pods, and a new pod must get an IP from a
// freshly added range rather than failing.
//
// This is the case the status->spec trigger in CCM exists for. Without it, a
// daemon whose ask already exceeds the VM's capacity has no reason to change
// spec.allocations, the spec controller (which reconciles on generation changes
// only) never re-runs, and the allocation times out.
func TestOutOfBandCIDRDeletion_CapacityIsRestored(t *testing.T) {
	f := newFixture(t)

	for i := 1; i <= initialUsableIPs; i++ {
		f.allocatePod(fmt.Sprintf("pod-%d", i))
	}
	_, dynamicCIDR := f.allocatePod("pod-overflow")
	f.eventually("status lists both ranges", func() (bool, string) {
		nnc := f.nnc()
		return equalSets(statusCIDRs(nnc), []string{initialPodCIDR, dynamicCIDR}), fmt.Sprintf("status=%v", statusCIDRs(nnc))
	})

	f.removeAliasRangeOutOfBand(dynamicCIDR)
	f.eventually("daemon marks the block Unknown", func() (bool, string) {
		st := f.blockState(dynamicCIDR)
		return st == store.StateUnknown, fmt.Sprintf("state=%q", st)
	})

	// The initial block is full and the only other block is Unknown. A new pod
	// must trigger a fresh scale-up rather than fail.
	start := time.Now()
	newCIDR, err := f.tryAllocatePod("pod-after-loss")
	if err != nil {
		t.Fatalf("pod-after-loss could not be allocated %s after the out-of-band loss: %v (spec pods=%d, gce=%v)",
			time.Since(start).Round(time.Millisecond), err, specPods(f.nnc()), f.gceAliasRanges())
	}
	if newCIDR == initialPodCIDR || newCIDR == dynamicCIDR {
		t.Fatalf("pod-after-loss allocated from %s, want a newly added range", newCIDR)
	}
}
