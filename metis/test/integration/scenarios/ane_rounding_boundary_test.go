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

	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/pkg/daemon"
	"k8s.io/metis/test/integration"
)

// TestScaleDown_ANE_RoundingBoundary exercises the fractional threshold boundary
// during scale-down of /32 ANE allocations.
//
// Skipped pending fix for Issue #1429 (metis/pkg/daemon/monitor.go) where
// targetUsedIPs := int(m.lowUtilizationThreshold * float64(totalPods)) truncates
// fractional targets, causing totalUsedIPs >= targetUsedIPs to break early and
// freeze idle /32 ANEs when usedIPs sits right on the boundary.
func TestScaleDown_ANE_RoundingBoundary(t *testing.T) {
	t.Skip("Skipping pending Issue #1429 fix in metis/pkg/daemon/monitor.go for /32 rounding threshold truncation")

	f := integration.NewFixture(t, integration.WithBackend(dynamicpodip.BackendTypeANE), integration.WithDaemonConfig(func(cfg *daemon.Config) {
		cfg.LowUtilizationThreshold = 0.50
	}))

	// Bootstrap block saturation: allocate pods to exhaust initial block.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up: trigger dynamic allocation of /32 ANEs.
	_, dynamicCIDR := f.AllocatePod("pod-ane-overflow")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}
	f.Eventually("ANEs are attached to GCE", func() (bool, string) {
		return len(f.GCEANEs()) > 0, fmt.Sprintf("anes=%d", len(f.GCEANEs()))
	})

	// Pod deletion to reach the fractional threshold boundary:
	// Deleting the overflow pod and 5 initial pods leaves 8 active pods.
	// Total IPs = 23 (16 initial + 7 ANEs). Used IPs = 8 active + 3 reserved = 11.
	// 11 / 23 = 47.8% < 50%.
	// Without math.Ceil: targetUsedIPs = int(0.50 * 23) = 11, so 11 >= 11 breaks early.
	// With math.Ceil: targetUsedIPs = ceil(11.5) = 12, allowing the idle ANEs to drain.
	f.DeletePod("pod-ane-overflow")
	for i := 1; i <= 5; i++ {
		f.DeletePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// Boundary release verification: idle ANEs should be drained and detached from GCE.
	f.Eventually("idle ANEs are drained and removed from GCE", func() (bool, string) {
		anes := f.GCEANEs()
		return len(anes) == 0, fmt.Sprintf("anes remaining: %d", len(anes))
	})
}
