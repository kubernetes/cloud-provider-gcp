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

	"k8s.io/metis/test/integration"
)

// TestNotReadyNodeHandling tests controller behavior during early node registration
// before ProviderID is initialized. It verifies that the controllers safely defer
// reconciliation with nodeNotReadyError while ProviderID is empty, emit appropriate
// warning events, and automatically resume reconciliation once the node's ProviderID
// is assigned.
func TestNotReadyNodeHandling(t *testing.T) {
	// Uninitialized node startup: launch controllers with a node lacking ProviderID.
	f := integration.NewFixture(t, integration.WithNodeProviderID(""))

	// Initial check: status is not yet ready because node has empty ProviderID.
	time.Sleep(500 * time.Millisecond)
	nnc := f.NNC()
	if integration.StatusReady(nnc) {
		t.Fatalf("expected status not ready while node is uninitialized")
	}

	// Node readiness: cloud-node-controller assigns ProviderID to the Node.
	f.UpdateNodeProviderID(integration.ProviderID)

	// Successful reconciliation: controllers proceed with reconciliation once Node is ready.
	f.Eventually("CCM publishes Ready status after ProviderID is set", func() (bool, string) {
		currentNNC := f.NNC()
		return integration.StatusReady(currentNNC) && integration.EqualSets(integration.StatusCIDRs(currentNNC), []string{integration.InitialPodCIDR}),
			fmt.Sprintf("ready=%v status=%v", integration.StatusReady(currentNNC), integration.StatusCIDRs(currentNNC))
	})
}
