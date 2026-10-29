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

// TestPartialReleaseWithActivePods tests selective scale-down when an instance has
// multiple dynamic blocks allocated. It verifies that an idle dynamic block is drained
// and released to GCE while sibling blocks with active pods remain untouched and
// operational.
func TestPartialReleaseWithActivePods(t *testing.T) {
	f := integration.NewFixture(t)

	// Multi-block expansion: exhaust initial block and expand into two dynamic blocks.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-partial-init-%d", i))
	}
	_, dynamicBlock1 := f.AllocatePod("pod-on-block-1")
	for i := 1; i <= 15; i++ {
		f.AllocatePod(fmt.Sprintf("pod-block1-filler-%d", i))
	}
	_, dynamicBlock2 := f.AllocatePod("pod-on-block-2")

	if dynamicBlock2 == dynamicBlock1 || dynamicBlock1 == integration.InitialPodCIDR || dynamicBlock2 == integration.InitialPodCIDR {
		t.Fatalf("expected distinct blocks: init=%s b1=%s b2=%s", integration.InitialPodCIDR, dynamicBlock1, dynamicBlock2)
	}

	f.Eventually("three distinct ranges active in GCE and status", func() (bool, string) {
		ranges := f.GCEAliasRanges()
		return len(ranges) == 3, fmt.Sprintf("ranges=%v", ranges)
	})

	// Targeted pod deletion: delete only the pod on the second dynamic block, leaving the first active.
	f.DeletePod("pod-on-block-2")

	// Partial release: release only dynamicBlock2.
	f.SetAllocationsPods(32)
	f.AddReleasableCIDR(dynamicBlock2)

	// Selective removal: CCM removes only dynamicBlock2 and preserves dynamicBlock1 and initial block.
	f.Eventually("only dynamicBlock2 is removed; dynamicBlock1 and initial block are preserved", func() (bool, string) {
		ranges := f.GCEAliasRanges()
		currentNNC := f.NNC()
		want := []string{integration.InitialPodCIDR, dynamicBlock1}
		ok := integration.EqualSets(ranges, want) && integration.EqualSets(integration.StatusCIDRs(currentNNC), want) && len(integration.ReleasableCIDRs(currentNNC)) == 0
		return ok, fmt.Sprintf("gce=%v status=%v releasable=%v", ranges, integration.StatusCIDRs(currentNNC), integration.ReleasableCIDRs(currentNNC))
	})
}
