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
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metis/test/integration"
)

// TestIPv6_NNCStatusCIDRIgnoredByMetis verifies that when an IPv6 CIDR is published
// to NodeNetworkConfig.status.podCIDRs, the Metis daemon watcher ignores the IPv6
// entry as a no-op and does not ingest it into its local database, preserving local-only
// IPv6 management.
func TestIPv6_NNCStatusCIDRIgnoredByMetis(t *testing.T) {
	f := integration.NewFixture(t)

	// Ensure daemon is active with initial pod allocation.
	f.AllocatePod("pod-init")

	// Inject an external IPv6 CIDR into NodeNetworkConfig status.
	ignoredIPv6CIDR := "2001:db8:99::/112"
	nnc := f.NNC().DeepCopy()
	nnc.Status.PodCIDRs = append(nnc.Status.PodCIDRs, nncv1.PodCIDR{
		Network: integration.Network,
		CIDR:    ignoredIPv6CIDR,
	})
	if _, err := f.NNCClient().NetworkingV1().NodeNetworkConfigs().UpdateStatus(f.Ctx(), nnc, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("failed to inject IPv6 CIDR into NNC status: %v", err)
	}

	// Verification of daemon watcher no-op: assert the IPv6 block is never created in SQLite.
	f.Consistently("injected IPv6 CIDR is ignored by Metis watcher and absent from SQLite store", 300*time.Millisecond, func() (bool, string) {
		exists := f.BlockExists(ignoredIPv6CIDR)
		return !exists, fmt.Sprintf("blockExists(%s)=%v", ignoredIPv6CIDR, exists)
	})
}
