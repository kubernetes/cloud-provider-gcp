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
	"net"
	"testing"

	"k8s.io/metis/test/integration"
)

// TestIPv6Only_BootstrapAndAllocation tests a single-stack IPv6 cluster where the
// node is initialized with only an IPv6 CIDR prefix and pod allocations request
// exclusively IPv6 addresses with no IPv4 configuration.
func TestIPv6Only_BootstrapAndAllocation(t *testing.T) {
	f := integration.NewFixture(t, integration.WithStackType(integration.StackTypeIPv6Only))

	// Allocate multiple IPv6 pods.
	for i := 1; i <= 5; i++ {
		ip, cidr := f.AllocatePod(fmt.Sprintf("pod-v6only-%d", i))
		if net.ParseIP(ip).To16() == nil || net.ParseIP(ip).To4() != nil {
			t.Fatalf("expected valid IPv6 address for pod %d, got %s", i, ip)
		}
		if cidr != integration.InitialPodCIDRv6 {
			t.Fatalf("expected IPv6 CIDR %s, got %s", integration.InitialPodCIDRv6, cidr)
		}
	}

	// Verification of zero IPv4 state in GCE: no alias ranges configured.
	gceRanges := f.GCEAliasRanges()
	if len(gceRanges) != 0 {
		t.Fatalf("expected 0 GCE IPv4 alias ranges in IPv6-only mode, got %v", gceRanges)
	}

	// Verification of store state: IPv6 usage reflects 5 active allocations.
	usage := f.IPv6Usage()
	if usage.Allocated != 5 {
		t.Fatalf("expected 5 allocated IPv6 addresses, got %d", usage.Allocated)
	}
}
