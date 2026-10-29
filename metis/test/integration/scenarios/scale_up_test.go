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

// TestScaleUpLoop tests the dynamic scale-up cycle when pod demand exhausts the
// bootstrap CIDR block. It verifies that pod exhaustion triggers the daemon's
// background monitor to request additional capacity in NodeNetworkConfig spec,
// the CCM spec controller attaches a new dynamic /28 alias range to the GCE
// instance, the status controller publishes the range, and the waiting pod
// allocation completes.
func TestScaleUpLoop(t *testing.T) {
	f := integration.NewFixture(t)

	// Initial saturation: allocate pods until the initial block is fully
	// exhausted.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-init-%d", i))
	}

	// Scale-up demand: an overflow pod triggers the daemon to request
	// additional capacity.
	_, dynamicCIDR := f.AllocatePod("pod-overflow")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected a dynamically added range")
	}

	// Infrastructure expansion: GCE, status, and SQLite converge on the
	// expanded capacity.
	f.Eventually("GCE, status, and SQLite converge on two ranges", func() (bool, string) {
		nnc := f.NNC()
		want := []string{integration.InitialPodCIDR, dynamicCIDR}
		var unmet []string
		if !integration.EqualSets(f.GCEAliasRanges(), want) {
			unmet = append(unmet, fmt.Sprintf("GCE alias ranges: got %v, want %v", f.GCEAliasRanges(), want))
		}
		if !integration.EqualSets(integration.StatusCIDRs(nnc), want) {
			unmet = append(unmet, fmt.Sprintf("NNC status CIDRs: got %v, want %v", integration.StatusCIDRs(nnc), want))
		}
		if !integration.StatusReady(nnc) {
			unmet = append(unmet, "NNC Ready condition is false")
		}
		if state := f.BlockState(dynamicCIDR); state != store.StateReady {
			unmet = append(unmet, fmt.Sprintf("DB block state: got %s, want %s", state, store.StateReady))
		}
		return integration.FormatUnmet(unmet)
	})

	if u := f.Usage(); u.Total != 32 {
		t.Fatalf("usage.Total after scale-up = %d, want 32", u.Total)
	}
}
