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

	"k8s.io/metis/pkg/store"
	"k8s.io/metis/test/integration"
)

// TestSequentialMultiBlockScaleUp verifies that the system progressively scales
// up across multiple dynamic blocks as pod capacity fills each preceding block.
func TestSequentialMultiBlockScaleUp(t *testing.T) {
	f := integration.NewFixture(t)

	// Bootstrap block saturation: allocate pods to exhaust initial block capacity.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-seq-init-%d", i))
	}

	// First dynamic block trigger: overflow pod triggers the first scale-up.
	_, dynCIDR1 := f.AllocatePod("pod-seq-dyn1-overflow")
	if dynCIDR1 == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected dynamic range")
	}

	// First dynamic block verification: wait for range to attach and status to publish.
	f.Eventually("first dynamic range is attached and ready", func() (bool, string) {
		currentNNC := f.NNC()
		ranges := f.GCEAliasRanges()
		return len(ranges) == 2 && len(integration.StatusCIDRs(currentNNC)) == 2 && f.BlockState(dynCIDR1) == store.StateReady,
			fmt.Sprintf("ranges=%v status=%v blockState=%q", ranges, integration.StatusCIDRs(currentNNC), f.BlockState(dynCIDR1))
	})

	// First dynamic block saturation: fill remaining capacity of the first dynamic block.
	// For secondary blocks, 0 IPs are reserved, so 15 additional pods can be allocated.
	for i := 1; i <= 15; i++ {
		_, c := f.AllocatePod(fmt.Sprintf("pod-seq-dyn1-%d", i))
		if c != dynCIDR1 {
			t.Fatalf("expected allocation from first dynamic block %s, got %s", dynCIDR1, c)
		}
	}

	// Second dynamic block trigger: second overflow pod triggers another scale-up.
	_, dynCIDR2 := f.AllocatePod("pod-seq-dyn2-overflow")
	if dynCIDR2 == integration.InitialPodCIDR || dynCIDR2 == dynCIDR1 {
		t.Fatalf("expected allocation from second dynamic block, got %s", dynCIDR2)
	}

	// Progressive watermarking verification: wait for second dynamic range to attach.
	f.Eventually("second dynamic range is attached and ready", func() (bool, string) {
		currentNNC := f.NNC()
		ranges := f.GCEAliasRanges()
		return len(ranges) == 3 && len(integration.StatusCIDRs(currentNNC)) == 3 && f.BlockState(dynCIDR2) == store.StateReady,
			fmt.Sprintf("ranges=%v status=%v blockState=%q", ranges, integration.StatusCIDRs(currentNNC), f.BlockState(dynCIDR2))
	})
}
