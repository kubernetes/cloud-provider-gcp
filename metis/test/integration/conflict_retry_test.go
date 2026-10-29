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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
)

// TestConflictRetryOnStatusUpdate tests optimistic concurrency handling during status
// reconciliation. It injects 409 Conflict errors into NodeNetworkConfig status update
// calls to verify that the status controller retries updates on conflict and converges
// successfully without dropping state (PR #1410 resilience).
func TestConflictRetryOnStatusUpdate(t *testing.T) {
	conflictInjected := false
	conflictResolved := false

	// Conflict injection: fail the first status write with 409 Conflict.
	f := newFixture(t, withPrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if !conflictInjected {
			conflictInjected = true
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"},
				nodeName, fmt.Errorf("simulated status conflict error"))
		}
		conflictResolved = true
		return false, nil, nil
	}))

	// Initial pod allocation triggers status reconciliation.
	f.allocatePod("pod-conflict-test")

	// Reconcile resilience: status controller retries on conflict and completes status publish.
	f.eventually("status publish succeeds despite initial 409 conflict", func() (bool, string) {
		nnc := f.nnc()
		return conflictResolved && statusReady(nnc) && len(statusCIDRs(nnc)) == 1,
			fmt.Sprintf("conflictResolved=%v ready=%v status=%v", conflictResolved, statusReady(nnc), statusCIDRs(nnc))
	})
}
