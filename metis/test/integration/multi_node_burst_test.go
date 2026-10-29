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
	"sync"
	"testing"
)

// TestMultiNodeConcurrentScaleUpBurst tests multi-node burst scaling where multiple
// nodes in the cluster experience simultaneous pod traffic that exhausts their
// bootstrap CIDR blocks concurrently. It verifies that the CCM controllers reconcile
// both nodes in parallel without cross-talk or CIDR leakage between instances.
func TestMultiNodeConcurrentScaleUpBurst(t *testing.T) {
	f := newFixture(t)

	// Cluster expansion: spin up a second node in the cluster.
	node1 := f.primaryNode()
	node2 := f.addNode("worker-2", "10.0.1.0/28")

	// Concurrent saturation: fill bootstrap capacity on both nodes in parallel.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1; i <= initialUsableIPs; i++ {
			node1.allocatePod(fmt.Sprintf("pod-node1-init-%d", i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 1; i <= initialUsableIPs; i++ {
			node2.allocatePod(fmt.Sprintf("pod-node2-init-%d", i))
		}
	}()
	wg.Wait()

	// Concurrent burst demand: trigger simultaneous scale-up across both nodes.
	var node1DynCIDR, node2DynCIDR string
	var err1, err2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, node1DynCIDR = node1.allocatePod("pod-node1-burst-overflow")
	}()
	go func() {
		defer wg.Done()
		_, node2DynCIDR = node2.allocatePod("pod-node2-burst-overflow")
	}()
	wg.Wait()

	if err1 != nil || err2 != nil {
		t.Fatalf("burst allocation failed: err1=%v err2=%v", err1, err2)
	}

	// Distinct CIDR isolation: verify both nodes received unique, non-overlapping dynamic ranges.
	if node1DynCIDR == node2DynCIDR {
		t.Fatalf("nodes received overlapping dynamic CIDRs: node1=%s node2=%s", node1DynCIDR, node2DynCIDR)
	}

	// Infrastructure convergence: both nodes converge on two ranges in GCE and NNC status.
	f.eventually("both nodes expand to two ranges in GCE and status", func() (bool, string) {
		ranges1 := node1.gceAliasRanges()
		ranges2 := node2.gceAliasRanges()
		nnc1 := node1.nnc()
		nnc2 := node2.nnc()

		ok1 := len(ranges1) == 2 && len(statusCIDRs(nnc1)) == 2 && statusReady(nnc1)
		ok2 := len(ranges2) == 2 && len(statusCIDRs(nnc2)) == 2 && statusReady(nnc2)
		return ok1 && ok2, fmt.Sprintf("node1(gce=%d, status=%d) node2(gce=%d, status=%d)",
			len(ranges1), len(statusCIDRs(nnc1)), len(ranges2), len(statusCIDRs(nnc2)))
	})

	// Verification of zero CIDR leakage between instances.
	if node1.blockExists(node2DynCIDR) {
		t.Fatalf("node 1 SQLite database contains node 2 dynamic CIDR %s", node2DynCIDR)
	}
	if node2.blockExists(node1DynCIDR) {
		t.Fatalf("node 2 SQLite database contains node 1 dynamic CIDR %s", node1DynCIDR)
	}
}
