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

// TestMaxPodsScaleUp_512Pods tests dynamic scale-up up to GKE's maximum supported
// capacity of 512 pods per node. It verifies that when capacity demand expands to
// 512 pods, the CCM spec controller attaches the full complement of dynamic CIDR
// blocks (32 x /28 alias ranges) to the GCE instance, the status controller publishes
// all 32 ranges, and Metis ingests the blocks to achieve 512 total IP capacity.
func TestMaxPodsScaleUp_512Pods(t *testing.T) {
	f := integration.NewFixture(t)

	// Ensure bootstrap: allocate the initial pod.
	f.AllocatePod("pod-bootstrap")

	// Max pods scale-up trigger: request 512 pods in NodeNetworkConfig spec.
	const targetMaxPods = 512
	f.SetAllocationsPods(targetMaxPods)

	// Multi-block expansion verification: 512 total IPs requires 32 blocks of /28 (16 IPs each).
	const expectedTotalBlocks = 32
	f.Eventually("GCE and NNC status expand to 32 alias ranges (512 total IPs)", func() (bool, string) {
		nnc := f.NNC()
		ranges := f.GCEAliasRanges()
		cidrs := integration.StatusCIDRs(nnc)
		ok := len(ranges) == expectedTotalBlocks && len(cidrs) == expectedTotalBlocks && integration.StatusReady(nnc)
		return ok, fmt.Sprintf("gceRanges=%d statusCIDRs=%d ready=%v", len(ranges), len(cidrs), integration.StatusReady(nnc))
	})

	// Store capacity verification: Metis SQLite store reflects 512 total IP capacity.
	f.Eventually("Metis SQLite store registers full 512 IP capacity", func() (bool, string) {
		u := f.Usage()
		return u.Total == targetMaxPods, fmt.Sprintf("usage.Total=%d (want %d)", u.Total, targetMaxPods)
	})

	// Allocation verification at scale: allocate pods across the expanded capacity.
	for i := 1; i <= 10; i++ {
		ip, cidr := f.AllocatePod(fmt.Sprintf("pod-scaled-%d", i))
		if ip == "" || cidr == "" {
			t.Fatalf("failed to allocate pod %d at high capacity", i)
		}
	}
}
