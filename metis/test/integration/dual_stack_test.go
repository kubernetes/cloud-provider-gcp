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
	"net"
	"testing"

	"k8s.io/metis/pkg/store"
)

// TestDualStack_IPv4ScaleUpWithLocalIPv6Expansion tests concurrent dual-stack
// pod IP allocation where IPv4 capacity exhausts and scales up dynamically through
// GCE and NNC, while IPv6 addresses are assigned concurrently from the local IPv6
// pool without triggering cloud requests.
func TestDualStack_IPv4ScaleUpWithLocalIPv6Expansion(t *testing.T) {
	f := newFixture(t, withStackType(StackTypeDualStack))

	// Bootstrap saturation: allocate pods with dual-stack IPs until IPv4 capacity exhausts.
	for i := 1; i <= initialUsableIPs; i++ {
		v4IP, v4CIDR, v6IP, v6CIDR := f.allocateDualStackPod(fmt.Sprintf("pod-dual-%d", i))
		if net.ParseIP(v4IP).To4() == nil {
			t.Fatalf("expected valid IPv4 address for pod %d, got %s", i, v4IP)
		}
		if net.ParseIP(v6IP).To16() == nil {
			t.Fatalf("expected valid IPv6 address for pod %d, got %s", i, v6IP)
		}
		if v4CIDR != initialPodCIDR {
			t.Fatalf("expected initial IPv4 CIDR %s, got %s", initialPodCIDR, v4CIDR)
		}
		if v6CIDR != initialPodCIDRv6 {
			t.Fatalf("expected initial IPv6 CIDR %s, got %s", initialPodCIDRv6, v6CIDR)
		}
	}

	// Dual-stack overflow: the next pod triggers dynamic IPv4 scale-up while IPv6 allocates locally.
	v4OverflowIP, v4DynCIDR, v6OverflowIP, v6DynCIDR := f.allocateDualStackPod("pod-dual-overflow")
	if v4DynCIDR == initialPodCIDR {
		t.Fatalf("overflow pod allocated from initial IPv4 range; expected dynamic range")
	}
	if v6DynCIDR != initialPodCIDRv6 {
		t.Fatalf("overflow pod allocated from unexpected IPv6 range: %s", v6DynCIDR)
	}
	if net.ParseIP(v4OverflowIP).To4() == nil || net.ParseIP(v6OverflowIP).To16() == nil {
		t.Fatalf("invalid IP returned: v4=%s v6=%s", v4OverflowIP, v6OverflowIP)
	}

	// Verification of infrastructure state: GCE reflects two IPv4 ranges, status publishes both.
	f.eventually("GCE and NNC status reflect two IPv4 ranges", func() (bool, string) {
		nnc := f.nnc()
		wantV4 := []string{initialPodCIDR, v4DynCIDR}
		ok := equalSets(f.gceAliasRanges(), wantV4) &&
			equalSets(statusCIDRs(nnc), wantV4) && statusReady(nnc) &&
			f.blockState(v4DynCIDR) == store.StateReady
		return ok, fmt.Sprintf("gce=%v status=%v ready=%v", f.gceAliasRanges(), statusCIDRs(nnc), statusReady(nnc))
	})

	// Verification of SQLite usage: both IPv4 and IPv6 reflect 14 active pods.
	v4Usage := f.usage()
	v6Usage := f.ipv6Usage()
	if v4Usage.Allocated != 3+14 {
		t.Fatalf("expected 17 allocated IPv4 addresses (3 reserved + 14 pods), got %d", v4Usage.Allocated)
	}
	if v6Usage.Allocated != 14 {
		t.Fatalf("expected 14 allocated IPv6 addresses, got %d", v6Usage.Allocated)
	}
}
