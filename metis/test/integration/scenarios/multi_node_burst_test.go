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
	"sync"
	"testing"

	"k8s.io/metis/test/integration"
)

// TestMultiNodeConcurrentScaleUpBurst tests multi-node burst scaling across 5 cluster
// nodes experiencing concurrent high-volume pod scheduling. It saturates the bootstrap
// blocks across all nodes and unleashes a parallel multi-pod burst, verifying that CCM
// spec and status controllers dynamically provision secondary alias ranges across all
// nodes in parallel with strict CIDR disjointness and zero cross-node database leakage.
func TestMultiNodeConcurrentScaleUpBurst(t *testing.T) {
	f := integration.NewFixture(t)

	// Cluster provisioning: initialize 5 distinct nodes in the cluster.
	nodes := []*integration.NodeHandle{
		f.PrimaryNode(),
		f.AddNode("worker-2", "10.0.1.0/28"),
		f.AddNode("worker-3", "10.0.2.0/28"),
		f.AddNode("worker-4", "10.0.3.0/28"),
		f.AddNode("worker-5", "10.0.4.0/28"),
	}

	// Initial capacity saturation: fill initial capacity (13 usable IPs) on all 5 nodes concurrently.
	var wg sync.WaitGroup
	errCh := make(chan error, len(nodes)*integration.InitialUsableIPs)
	for nodeIdx, node := range nodes {
		wg.Add(1)
		go func(idx int, nh *integration.NodeHandle) {
			defer wg.Done()
			for i := 1; i <= integration.InitialUsableIPs; i++ {
				podName := fmt.Sprintf("pod-node%d-init-%d", idx+1, i)
				ip, cidr := nh.AllocatePod(podName)
				if ip == "" || cidr == "" {
					errCh <- fmt.Errorf("node %s failed to allocate %s", nh.Name(), podName)
				}
			}
		}(nodeIdx, node)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	// Concurrent multi-pod burst: burst 15 overflow pods on each node concurrently (75 overflow pods).
	const burstPodsPerNode = 15
	nodeBurstCIDRs := make([][]string, len(nodes))
	var burstMutex sync.Mutex
	burstErrCh := make(chan error, len(nodes)*burstPodsPerNode)

	for nodeIdx, node := range nodes {
		wg.Add(1)
		go func(idx int, nh *integration.NodeHandle) {
			defer wg.Done()
			var assignedCIDRs []string
			for p := 1; p <= burstPodsPerNode; p++ {
				podName := fmt.Sprintf("pod-node%d-burst-%d", idx+1, p)
				ip, cidr := nh.AllocatePod(podName)
				if ip == "" || cidr == "" {
					burstErrCh <- fmt.Errorf("node %s failed to allocate burst %s", nh.Name(), podName)
					return
				}
				assignedCIDRs = append(assignedCIDRs, cidr)
			}
			burstMutex.Lock()
			nodeBurstCIDRs[idx] = assignedCIDRs
			burstMutex.Unlock()
		}(nodeIdx, node)
	}
	wg.Wait()
	close(burstErrCh)
	for err := range burstErrCh {
		t.Fatal(err)
	}

	// Infrastructure convergence verification: all 5 nodes expand to 2 or more ranges and publish Ready=True.
	f.Eventually("all 5 nodes expand in GCE and status with Ready=True", func() (bool, string) {
		for _, nh := range nodes {
			ranges := nh.GCEAliasRanges()
			nnc := nh.NNC()
			cidrs := integration.StatusCIDRs(nnc)
			if len(ranges) < 2 || len(cidrs) < 2 || !integration.StatusReady(nnc) {
				return false, fmt.Sprintf("node %s not ready: gceRanges=%d statusCIDRs=%d ready=%v",
					nh.Name(), len(ranges), len(cidrs), integration.StatusReady(nnc))
			}
		}
		return true, "all 5 nodes expanded and ready"
	})

	// CIDR disjointness verification: ensure no dynamic CIDR is shared across any two nodes.
	allDynamicCIDRs := make(map[string]string)
	for idx, nh := range nodes {
		for _, c := range nodeBurstCIDRs[idx] {
			if owner, exists := allDynamicCIDRs[c]; exists && owner != nh.Name() {
				t.Fatalf("CIDR overlap detected: %s assigned to both %s and %s", c, owner, nh.Name())
			}
			allDynamicCIDRs[c] = nh.Name()
		}
	}

	// Store isolation verification: zero cross-node dynamic CIDR leakage across SQLite databases.
	for i, nodeA := range nodes {
		for j, nodeB := range nodes {
			if i == j {
				continue
			}
			for _, cidr := range nodeBurstCIDRs[j] {
				if nodeA.BlockExists(cidr) {
					t.Fatalf("cross-node leakage: node %s SQLite store contains node %s dynamic CIDR %s",
						nodeA.Name(), nodeB.Name(), cidr)
				}
			}
		}
	}
}
