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

	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/test/integration"
)

// TestScaleDown_ANE exercises dynamic scale-down and release of /32 NonReusable
// allocations. Skipped pending Issue #1429 for /32 scale-down rounding and pruning stabilization.
func TestScaleDown_ANE(t *testing.T) {
	t.Skip("Skipping ANE /32 scale-down test pending Issue #1429 for /32 rounding and pruning stabilization")

	f := integration.NewFixture(t, integration.WithBackend(dynamicpodip.BackendTypeANE))

	// Bootstrap: allocate pods to exhaust the CNI-provided initial block.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up demand: an overflow pod triggers the daemon to request additional capacity.
	_, dynamicCIDR := f.AllocatePod("pod-ane-overflow")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}

	// Workload reduction: delete all pods to trigger scale-down of the idle /32 ANE.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.DeletePod(fmt.Sprintf("pod-ane-init-%d", i))
	}
	f.DeletePod("pod-ane-overflow")

	// Infrastructure release: CCM detaches the ANE from GCE and prunes releasableCIDRs.
	f.Eventually("GCE removes ANE and CCM prunes releasableCIDRs", func() (bool, string) {
		anes := f.GCEANEs()
		currentNNC := f.NNC()
		rel := integration.ReleasableCIDRs(currentNNC)
		return len(anes) == 0 && len(rel) == 0 && integration.EqualSets(integration.StatusCIDRs(currentNNC), []string{integration.InitialPodCIDR}),
			fmt.Sprintf("anes=%d releasable=%v status=%v", len(anes), rel, integration.StatusCIDRs(currentNNC))
	})
}
