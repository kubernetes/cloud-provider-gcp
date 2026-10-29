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
)

// TestIPv6_LocalExpansionWithoutCloudCall verifies that when IPv6 pod demand exceeds
// the initial populated batch (64 addresses), Metis scales up locally by expanding
// the block directly in its SQLite store without issuing cloud scale-up requests in
// NodeNetworkConfig spec or invoking GCE APIs.
func TestIPv6_LocalExpansionWithoutCloudCall(t *testing.T) {
	f := newFixture(t, withStackType(StackTypeIPv6Only))

	// Initial batch saturation: allocate 64 pods to exhaust the store's initial populated batch.
	for i := 1; i <= 64; i++ {
		ip, cidr := f.allocatePod(fmt.Sprintf("pod-ipv6-%d", i))
		if ip == "" {
			t.Fatalf("allocated empty IP for pod %d", i)
		}
		if cidr != initialPodCIDRv6 {
			t.Fatalf("pod %d allocated from %s, want %s", i, cidr, initialPodCIDRv6)
		}
	}

	// Local batch expansion trigger: allocate the 65th pod to trigger expandIPv6Block in SQLite.
	ip65, cidr65 := f.allocatePod("pod-ipv6-65-overflow")
	if ip65 == "" {
		t.Fatalf("overflow pod allocated empty IP")
	}
	if cidr65 != initialPodCIDRv6 {
		t.Fatalf("overflow pod allocated from %s, want %s", cidr65, initialPodCIDRv6)
	}

	// Cloud provider quiescence verification: assert NNC spec allocations remain untouched.
	f.consistently("NNC spec allocations remains empty with zero cloud scale-up calls", 300*time.Millisecond, func() (bool, string) {
		currentNNC := f.nnc()
		hasCloudScaleUp := false
		for _, a := range currentNNC.Spec.Allocations {
			if a.Pods > 0 {
				hasCloudScaleUp = true
			}
		}
		ranges := f.gceAliasRanges()
		return !hasCloudScaleUp && len(ranges) == 0,
			fmt.Sprintf("hasCloudScaleUp=%v gceRanges=%v", hasCloudScaleUp, ranges)
	})

	// Store verification: assert IPv6 usage in SQLite reflects 65 allocated addresses.
	usage := f.ipv6Usage()
	if usage.Allocated != 65 {
		t.Fatalf("expected 65 allocated IPv6 addresses in store, got %d", usage.Allocated)
	}
}
