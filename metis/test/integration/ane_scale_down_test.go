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

	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
)

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
