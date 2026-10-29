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
