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
	"testing"

	"k8s.io/metis/pkg/daemon"
)

// TestIPReuseAndChurn verifies that when a pod is deleted, its IP address is
// returned to the local pool and reused for subsequent allocations within the
// same block without triggering scale-up.
func TestIPReuseAndChurn(t *testing.T) {
	f := newFixture(t, withDaemonConfig(func(cfg *daemon.Config) {
		cfg.ReleaseCooldown = 0
	}))

	// Initial pod allocation: allocate a pod and record its assigned IP.
	ip1, cidr1 := f.allocatePod("pod-churn-1")
	if cidr1 != initialPodCIDR {
		t.Fatalf("expected allocation from initial range, got %s", cidr1)
	}

	// Pod release: delete the pod to return the IP to the local store.
	f.deletePod("pod-churn-1")

	// Local IP reuse: allocate a new pod and verify it receives the freed IP.
	ip2, cidr2 := f.allocatePod("pod-churn-2")
	if ip2 != ip1 {
		t.Fatalf("expected IP reuse of %s, got %s", ip1, ip2)
	}
	if cidr2 != cidr1 {
		t.Fatalf("expected CIDR %s, got %s", cidr1, cidr2)
	}

	// Verification of zero cloud provider churn: GCE ranges and NNC spec unchanged.
	aliasRanges := f.gceAliasRanges()
	if len(aliasRanges) != 1 || aliasRanges[0] != initialPodCIDR {
		t.Fatalf("unexpected GCE alias ranges: %v", aliasRanges)
	}
}
