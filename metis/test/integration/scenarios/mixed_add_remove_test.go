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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metis/test/integration"
)

// TestMixedAddAndRemove tests concurrent capacity expansion and release in a single
// reconcile cycle. It simulates simultaneous workload growth on one network interface
// while an idle CIDR block on another is marked for release, verifying that the CCM
// spec controller processes additions and removals together without dropping either.
func TestMixedAddAndRemove(t *testing.T) {
	f := integration.NewFixture(t)

	// Baseline provisioning: establish two blocks via scale-up.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-mixed-%d", i))
	}
	_, dynamicCIDR := f.AllocatePod("pod-to-release")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("expected dynamically added range")
	}

	f.Eventually("baseline two blocks established in GCE", func() (bool, string) {
		return len(f.GCEAliasRanges()) == 2, fmt.Sprintf("ranges=%v", f.GCEAliasRanges())
	})

	// Pod terminates on the dynamic block.
	f.DeletePod("pod-to-release")

	// Simultaneous scale-up and release: release dynamicCIDR while requesting 32 pods.
	// Since dynamicCIDR is marked releasable, its capacity does not satisfy demand,
	// prompting CCM to execute removal of dynamicCIDR and addition of a replacement block.
	nnc := f.NNC().DeepCopy()
	nnc.Spec.Allocations[0].Pods = 32
	nnc.Spec.ReleasableCIDRs = []nncv1.PodCIDR{
		{Network: integration.Network, CIDR: dynamicCIDR},
	}
	if _, err := f.NNCClient().NetworkingV1().NodeNetworkConfigs().Update(f.Ctx(), nnc, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to update NNC for mixed add/remove: %v", err)
	}

	// Reconcile execution: CCM removes dynamicCIDR, adds a new block, and prunes releasableCIDRs.
	f.Eventually("releasable CIDR is pruned and replacement range is attached", func() (bool, string) {
		currentNNC := f.NNC()
		ranges := f.GCEAliasRanges()
		releasables := integration.ReleasableCIDRs(currentNNC)
		hasOld := false
		for _, r := range ranges {
			if r == dynamicCIDR {
				hasOld = true
			}
		}
		ok := len(releasables) == 0 && len(ranges) == 2 && !hasOld && integration.StatusReady(currentNNC)
		return ok, fmt.Sprintf("releasables=%v ranges=%v statusReady=%v", releasables, ranges, integration.StatusReady(currentNNC))
	})

	// Status confirmation: status reflects two active ranges, excluding the released range.
	finalNNC := f.NNC()
	if integration.EqualSets(integration.StatusCIDRs(finalNNC), []string{integration.InitialPodCIDR, dynamicCIDR}) {
		t.Fatalf("status still contains released CIDR %s", dynamicCIDR)
	}
	if len(integration.StatusCIDRs(finalNNC)) != 2 {
		t.Fatalf("expected 2 CIDRs in status, got %d: %v", len(integration.StatusCIDRs(finalNNC)), integration.StatusCIDRs(finalNNC))
	}
}
