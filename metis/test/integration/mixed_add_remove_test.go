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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
