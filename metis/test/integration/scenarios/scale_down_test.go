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

package scenarios

import (
	"fmt"
	"testing"

	"k8s.io/metis/test/integration"
)

// TestScaleDownAndReleaseLoop tests dynamic scale-down and resource
// reclamation when workload drops. It verifies that when active pods are
// deleted, the daemon stages the idle dynamic block in spec.releasableCIDRs,
// and the CCM spec controller detaches the alias range from the GCE instance
// and prunes the release request from NodeNetworkConfig spec.
func TestScaleDownAndReleaseLoop(t *testing.T) {
	f := integration.NewFixture(t)

	// Capacity expansion: scale up to two blocks so an expansion block exists.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-scale-%d", i))
	}
	_, dynamicCIDR := f.AllocatePod("pod-drain-target")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected a dynamically added range")
	}

	f.Eventually("dynamic block is active in GCE and status", func() (bool, string) {
		nnc := f.NNC()
		want := []string{integration.InitialPodCIDR, dynamicCIDR}
		return integration.EqualSets(f.GCEAliasRanges(), want) && integration.EqualSets(integration.StatusCIDRs(nnc), want) && integration.StatusReady(nnc),
			fmt.Sprintf("gce=%v status=%v", f.GCEAliasRanges(), integration.StatusCIDRs(nnc))
	})

	// Workload reduction: delete all pods so utilization drops and idle
	// capacity can be released.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.DeletePod(fmt.Sprintf("pod-scale-%d", i))
	}
	f.DeletePod("pod-drain-target")

	// Drain and release initiation: mark the idle block for release and
	// reduce desired pods.
	f.SetAllocationsPods(16)
	f.AddReleasableCIDR(dynamicCIDR)

	// Infrastructure release: CCM detaches the released alias range from GCE VM.
	f.Eventually("GCE instance removes the released alias range", func() (bool, string) {
		ranges := f.GCEAliasRanges()
		return integration.EqualSets(ranges, []string{integration.InitialPodCIDR}), fmt.Sprintf("gce=%v", ranges)
	})

	// Spec hygiene: CCM spec controller prunes the released range from
	// spec.releasableCIDRs.
	f.Eventually("CCM prunes releasableCIDRs from NNC spec", func() (bool, string) {
		rel := integration.ReleasableCIDRs(f.NNC())
		return len(rel) == 0, fmt.Sprintf("releasableCIDRs=%v", rel)
	})

	// Status reconciliation: CCM status controller updates status to
	// reflect only the retained block.
	f.Eventually("status reflects only the retained initial range with Ready=True", func() (bool, string) {
		nnc := f.NNC()
		return integration.EqualSets(integration.StatusCIDRs(nnc), []string{integration.InitialPodCIDR}) && integration.StatusReady(nnc),
			fmt.Sprintf("status=%v ready=%v", integration.StatusCIDRs(nnc), integration.StatusReady(nnc))
	})
}
