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

// Subsystem: Kubernetes NodeNetworkConfig CRD operations and API server semantics.
//
// Scope & Architecture:
// This file owns the simulated Kubernetes API server environment for
// NodeNetworkConfig (NNC) custom resources. It manages client-go reaction rules,
// API server generation auto-increment semantics, and NNC spec/status queries.
// Any custom API server reactions or NNC object inspection helpers belong in
// this file.

package integration

import (
	"fmt"
	"strconv"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
)

// customReactor stores a client-go reaction rule to prepend to the fake NNC
// clientset.
type customReactor struct {
	verb     string
	resource string
	reaction clientgotesting.ReactionFunc
}

// extractTargetName returns the target NodeNetworkConfig resource name from a
// client-go action.
func extractTargetName(action clientgotesting.Action) string {
	switch a := action.(type) {
	case clientgotesting.CreateAction:
		if obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig); ok {
			return obj.Name
		}
	case clientgotesting.UpdateAction:
		if obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig); ok {
			return obj.Name
		}
	case clientgotesting.PatchAction:
		return a.GetName()
	}
	return ""
}

// emulateAPIServerSemantics installs a reactor on the fake NodeNetworkConfig
// clientset that reproduces the three apiserver behaviours the controllers
// depend on and the fake tracker does not implement:
//
//   - Status subresource isolation: a status write must not clobber spec
//     fields written concurrently by the other side (the daemon patches spec
//     while CCM writes status).
//   - metadata.resourceVersion changes on every write. The metis watcher drops
//     updates whose resourceVersion did not change.
//   - metadata.generation increments when a non-status write changes the spec.
//     The CCM spec controller drops updates whose generation did not change.
//
// Writes go through the tracker rather than back through the clientset, because
// the clientset lock is held while a reactor runs.
func emulateAPIServerSemantics(c *nncfake.Clientset) {
	defaultReaction := clientgotesting.ObjectReaction(c.Tracker())
	c.PrependReactor("*", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		name := extractTargetName(action)
		if name == "" {
			return false, nil, nil
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()

		// Retrieve current stored state to compare against.
		var before *nncv1.NodeNetworkConfig
		if existing, err := c.Tracker().Get(gvr, ns, name); err == nil {
			before, _ = existing.(*nncv1.NodeNetworkConfig)
		}

		// Apply mutation with subresource isolation.
		// A write to /status updates status while preserving the stored spec,
		// preventing concurrent daemon spec updates from being clobbered by CCM.
		var after *nncv1.NodeNetworkConfig
		if action.GetVerb() == "update" && action.GetSubresource() == "status" && before != nil {
			update, ok := action.(clientgotesting.UpdateAction)
			if !ok {
				return false, nil, nil
			}
			after = update.GetObject().(*nncv1.NodeNetworkConfig).DeepCopy()
			after.Spec = *before.Spec.DeepCopy()
		} else {
			handled, obj, err := defaultReaction(action)
			if !handled || err != nil {
				return handled, obj, err
			}
			stored, ok := obj.(*nncv1.NodeNetworkConfig)
			if !ok {
				return true, obj, nil
			}
			after = stored.DeepCopy()
		}

		// ResourceVersion: increment on every write to simulate etcd mod_revision
		// (the Metis watcher drops updates whose resourceVersion did not change).
		prevRV, prevGen := 0, int64(0)
		if before != nil {
			prevRV, _ = strconv.Atoi(before.ResourceVersion)
			prevGen = before.Generation
		}
		after.ResourceVersion = strconv.Itoa(prevRV + 1)

		// Generation: increment only when a non-status write modifies the spec
		// (the CCM spec controller drops updates whose generation did not change).
		if before == nil {
			after.Generation = 1
		} else if action.GetSubresource() == "" && !apiequality.Semantic.DeepEqual(before.Spec, after.Spec) {
			after.Generation = prevGen + 1
		} else {
			after.Generation = prevGen
		}

		// Commit updated object with synthesized metadata back to the tracker.
		if err := c.Tracker().Update(gvr, after, ns); err != nil {
			return true, nil, err
		}
		return true, after, nil
	})
}

// NNC fetches the current NodeNetworkConfig object for this node from the fake
// clientset.
func (n *nodeHandle) NNC() *nncv1.NodeNetworkConfig {
	n.fixture.t.Helper()
	nnc, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Get(n.fixture.ctx, n.name, metav1.GetOptions{})
	if err != nil {
		n.fixture.t.Fatalf("failed to get NodeNetworkConfig for %s: %v", n.name, err)
	}
	return nnc
}

// StatusCIDRs extracts the CIDRs from NodeNetworkConfig.status.podCIDRs for
// the default network.
func StatusCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, pc := range nnc.Status.PodCIDRs {
		if pc.Network == defaultNetwork {
			cidrs = append(cidrs, pc.CIDR)
		}
	}
	return cidrs
}

// StatusReady checks if the NodeNetworkConfig has Condition Ready=True.
func StatusReady(nnc *nncv1.NodeNetworkConfig) bool {
	return apimeta.IsStatusConditionTrue(nnc.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
}

// SpecPods returns the requested pod capacity for the default network in NNC
// spec.
func SpecPods(nnc *nncv1.NodeNetworkConfig) int32 {
	for _, a := range nnc.Spec.Allocations {
		if a.Network == defaultNetwork {
			return a.Pods
		}
	}
	return 0
}

// SettledSpecPods polls until spec.allocations stops changing, then returns
// the settled value.
func (n *nodeHandle) SettledSpecPods() int32 {
	n.fixture.t.Helper()
	var last int32 = -1
	n.fixture.Eventually(fmt.Sprintf("spec.allocations settles on %s", n.name), func(a *Assert) {
		current := SpecPods(n.NNC())
		if a.Check(current == last && current > 0, "spec.allocations still changing: %d", current) {
			return
		}
		last = current
	})
	n.fixture.Consistently(fmt.Sprintf("spec.allocations stays at settled value on %s", n.name), 200*time.Millisecond, func(a *Assert) {
		a.Equal("spec.allocations", SpecPods(n.NNC()), last)
	})
	return last
}

// ReleasableCIDRs extracts CIDRs from NodeNetworkConfig.spec.releasableCIDRs
// for the default network.
func ReleasableCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, r := range nnc.Spec.ReleasableCIDRs {
		if r.Network == defaultNetwork {
			cidrs = append(cidrs, r.CIDR)
		}
	}
	return cidrs
}

// AddReleasableCIDR appends a CIDR to NodeNetworkConfig.spec.releasableCIDRs
// for this node.
func (n *nodeHandle) AddReleasableCIDR(cidr string) {
	n.fixture.t.Helper()
	nnc := n.NNC().DeepCopy()
	nnc.Spec.ReleasableCIDRs = append(nnc.Spec.ReleasableCIDRs, nncv1.PodCIDR{
		Network: defaultNetwork,
		CIDR:    cidr,
	})
	if _, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Update(n.fixture.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		n.fixture.t.Fatalf("failed to add releasable CIDR %s on %s: %v", cidr, n.name, err)
	}
}

// SetAllocationsPods updates NodeNetworkConfig.spec.allocations for the
// default network on this node.
func (n *nodeHandle) SetAllocationsPods(pods int32) {
	n.fixture.t.Helper()
	nnc := n.NNC().DeepCopy()
	found := false
	for i, a := range nnc.Spec.Allocations {
		if a.Network == defaultNetwork {
			nnc.Spec.Allocations[i].Pods = pods
			found = true
			break
		}
	}
	if !found {
		nnc.Spec.Allocations = append(nnc.Spec.Allocations, nncv1.Allocation{
			Network: defaultNetwork,
			Pods:    pods,
		})
	}
	if _, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Update(n.fixture.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		n.fixture.t.Fatalf("failed to set allocations pods to %d on %s: %v", pods, n.name, err)
	}
}

// NodeEvents returns the messages of the Events recorded against this node
// with the given reason.
func (n *nodeHandle) NodeEvents(reason string) []string {
	n.fixture.t.Helper()
	list, err := n.fixture.kubeClient.CoreV1().Events("").List(n.fixture.ctx, metav1.ListOptions{})
	if err != nil {
		n.fixture.t.Fatalf("failed to list events: %v", err)
	}
	var messages []string
	for _, ev := range list.Items {
		if ev.Reason == reason && ev.InvolvedObject.Kind == "Node" && ev.InvolvedObject.Name == n.name {
			messages = append(messages, ev.Message)
		}
	}
	return messages
}
