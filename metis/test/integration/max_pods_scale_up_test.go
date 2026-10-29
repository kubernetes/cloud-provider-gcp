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

// TestMaxPodsScaleUp_512Pods tests dynamic scale-up up to GKE's maximum supported
// capacity of 512 pods per node. It verifies that when capacity demand expands to
// 512 pods, the CCM spec controller attaches the full complement of dynamic CIDR
// blocks (32 x /28 alias ranges) to the GCE instance, the status controller publishes
// all 32 ranges, and Metis ingests the blocks to achieve 512 total IP capacity.
func TestMaxPodsScaleUp_512Pods(t *testing.T) {
	f := newFixture(t)

	// Ensure bootstrap: allocate the initial pod.
	f.allocatePod("pod-bootstrap")

	// Max pods scale-up trigger: request 512 pods in NodeNetworkConfig spec.
	const targetMaxPods = 512
	f.setAllocationsPods(targetMaxPods)

	// Multi-block expansion verification: 512 total IPs requires 32 blocks of /28 (16 IPs each).
	const expectedTotalBlocks = 32
	f.eventually("GCE and NNC status expand to 32 alias ranges (512 total IPs)", func() (bool, string) {
		nnc := f.nnc()
		ranges := f.gceAliasRanges()
		cidrs := statusCIDRs(nnc)
		ok := len(ranges) == expectedTotalBlocks && len(cidrs) == expectedTotalBlocks && statusReady(nnc)
		return ok, fmt.Sprintf("gceRanges=%d statusCIDRs=%d ready=%v", len(ranges), len(cidrs), statusReady(nnc))
	})

	// Store capacity verification: Metis SQLite store reflects 512 total IP capacity.
	f.eventually("Metis SQLite store registers full 512 IP capacity", func() (bool, string) {
		u := f.usage()
		return u.Total == targetMaxPods, fmt.Sprintf("usage.Total=%d (want %d)", u.Total, targetMaxPods)
	})

	// Allocation verification at scale: allocate pods across the expanded capacity.
	for i := 1; i <= 10; i++ {
		ip, cidr := f.allocatePod(fmt.Sprintf("pod-scaled-%d", i))
		if ip == "" || cidr == "" {
			t.Fatalf("failed to allocate pod %d at high capacity", i)
		}
	}
}
