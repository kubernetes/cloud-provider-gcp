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
)

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
