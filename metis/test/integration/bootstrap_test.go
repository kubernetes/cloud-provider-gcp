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

	"k8s.io/metis/pkg/store"
)

// TestBootstrapAllocation tests node bootstrap where the initial pod is allocated
// from the CNI-provided initial CIDR before cloud controllers have published
// status. It verifies that the daemon serves the local allocation immediately,
// and the CCM status controller discovers the existing instance alias range,
// publishes it to NodeNetworkConfig status, and marks the node Ready.
func TestBootstrapAllocation(t *testing.T) {
	f := newFixture(t)

	// Bootstrap: allocate the first pod from the CNI-provided initial CIDR.
	_, cidr := f.allocatePod("pod-bootstrap")
	if cidr != initialPodCIDR {
		t.Fatalf("pod allocated from %s, want initial range %s", cidr, initialPodCIDR)
	}

	// Convergence: CCM discovers the initial range on the VM and publishes Ready status.
	f.eventually("CCM publishes initial range with Ready=True", func() (bool, string) {
		nnc := f.nnc()
		return equalSets(statusCIDRs(nnc), []string{initialPodCIDR}) && statusReady(nnc),
			fmt.Sprintf("status=%v ready=%v", statusCIDRs(nnc), statusReady(nnc))
	})

	// Store verification: daemon watcher sets the block state to Ready in SQLite.
	if got := f.blockState(initialPodCIDR); got != store.StateReady {
		t.Fatalf("initial block state after bootstrap = %q, want Ready", got)
	}
	if got := f.gceAliasRanges(); !equalSets(got, []string{initialPodCIDR}) {
		t.Fatalf("GCE alias ranges = %v, want exactly %v", got, []string{initialPodCIDR})
	}
}
