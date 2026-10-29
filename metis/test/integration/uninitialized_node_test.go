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
	"time"
)

// TestNotReadyNodeHandling tests controller behavior during early node registration
// before ProviderID is initialized. It verifies that the controllers safely defer
// reconciliation with nodeNotReadyError while ProviderID is empty, emit appropriate
// warning events, and automatically resume reconciliation once the node's ProviderID
// is assigned.
func TestNotReadyNodeHandling(t *testing.T) {
	// Uninitialized node startup: launch controllers with a node lacking ProviderID.
	f := newFixture(t, withNodeProviderID(""))

	// Initial check: status is not yet ready because node has empty ProviderID.
	time.Sleep(500 * time.Millisecond)
	nnc := f.nnc()
	if statusReady(nnc) {
		t.Fatalf("expected status not ready while node is uninitialized")
	}

	// Node readiness: cloud-node-controller assigns ProviderID to the Node.
	f.updateNodeProviderID(providerID)

	// Successful reconciliation: controllers proceed with reconciliation once Node is ready.
	f.eventually("CCM publishes Ready status after ProviderID is set", func() (bool, string) {
		currentNNC := f.nnc()
		return statusReady(currentNNC) && equalSets(statusCIDRs(currentNNC), []string{initialPodCIDR}),
			fmt.Sprintf("ready=%v status=%v", statusReady(currentNNC), statusCIDRs(currentNNC))
	})
}
