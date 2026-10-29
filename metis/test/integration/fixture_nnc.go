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

// customReactor stores a client-go reaction rule to prepend to the fake NNC clientset.
type customReactor struct {
	verb     string
	resource string
	reaction clientgotesting.ReactionFunc
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
		var name string
		switch a := action.(type) {
		case clientgotesting.CreateAction:
			obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig)
			if !ok {
				return false, nil, nil
			}
			name = obj.Name
		case clientgotesting.UpdateAction:
			obj, ok := a.GetObject().(*nncv1.NodeNetworkConfig)
			if !ok {
				return false, nil, nil
			}
			name = obj.Name
		case clientgotesting.PatchAction:
			name = a.GetName()
		default:
			return false, nil, nil
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()

		var before *nncv1.NodeNetworkConfig
		if existing, err := c.Tracker().Get(gvr, ns, name); err == nil {
			before, _ = existing.(*nncv1.NodeNetworkConfig)
		}

		var after *nncv1.NodeNetworkConfig
		if update, ok := action.(clientgotesting.UpdateAction); ok && action.GetSubresource() == "status" && before != nil {
			// Status subresource: keep the stored spec, take the incoming status.
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

		prevRV, prevGen := 0, int64(0)
		if before != nil {
			prevRV, _ = strconv.Atoi(before.ResourceVersion)
			prevGen = before.Generation
		}
		after.ResourceVersion = strconv.Itoa(prevRV + 1)
		if before == nil {
			after.Generation = 1
		} else if action.GetSubresource() == "" && !apiequality.Semantic.DeepEqual(before.Spec, after.Spec) {
			after.Generation = prevGen + 1
		} else {
			after.Generation = prevGen
		}

		if err := c.Tracker().Update(gvr, after, ns); err != nil {
			return true, nil, err
		}
		return true, after, nil
	})
}

// NNC fetches the current NodeNetworkConfig object for this node from the fake clientset.
func (n *NodeHandle) NNC() *nncv1.NodeNetworkConfig {
	n.fixture.t.Helper()
	nnc, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Get(n.fixture.ctx, n.name, metav1.GetOptions{})
	if err != nil {
		n.fixture.t.Fatalf("failed to get NodeNetworkConfig for %s: %v", n.name, err)
	}
	return nnc
}

func (n *NodeHandle) nnc() *nncv1.NodeNetworkConfig {
	return n.NNC()
}

// NNC fetches the current NodeNetworkConfig object from the fake clientset.
func (f *Fixture) NNC() *nncv1.NodeNetworkConfig {
	return f.nodes[nodeName].NNC()
}

func (f *Fixture) nnc() *nncv1.NodeNetworkConfig {
	return f.NNC()
}

// StatusCIDRs extracts the CIDRs from NodeNetworkConfig.status.podCIDRs for the default network.
func StatusCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, pc := range nnc.Status.PodCIDRs {
		if pc.Network == network {
			cidrs = append(cidrs, pc.CIDR)
		}
	}
	return cidrs
}

func statusCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	return StatusCIDRs(nnc)
}

// StatusReady checks if the NodeNetworkConfig has Condition Ready=True.
func StatusReady(nnc *nncv1.NodeNetworkConfig) bool {
	return apimeta.IsStatusConditionTrue(nnc.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
}

func statusReady(nnc *nncv1.NodeNetworkConfig) bool {
	return StatusReady(nnc)
}

// SpecPods returns the requested pod capacity for the default network in NNC spec.
func SpecPods(nnc *nncv1.NodeNetworkConfig) int32 {
	for _, a := range nnc.Spec.Allocations {
		if a.Network == network {
			return a.Pods
		}
	}
	return 0
}

func specPods(nnc *nncv1.NodeNetworkConfig) int32 {
	return SpecPods(nnc)
}

// SettledSpecPods polls until spec.allocations stops changing, then returns the settled value.
func (n *NodeHandle) SettledSpecPods() int32 {
	n.fixture.t.Helper()
	var last int32 = -1
	n.fixture.Eventually(fmt.Sprintf("spec.allocations settles on %s", n.name), func() (bool, string) {
		current := SpecPods(n.NNC())
		if current == last && current > 0 {
			return true, fmt.Sprintf("settled at %d", current)
		}
		last = current
		return false, fmt.Sprintf("still changing: %d", current)
	})
	n.fixture.Consistently(fmt.Sprintf("spec.allocations stays at settled value on %s", n.name), 200*time.Millisecond, func() (bool, string) {
		return SpecPods(n.NNC()) == last, fmt.Sprintf("spec=%d want=%d", SpecPods(n.NNC()), last)
	})
	return last
}

func (n *NodeHandle) settledSpecPods() int32 {
	return n.SettledSpecPods()
}

// SettledSpecPods polls until spec.allocations stops changing on primary node, then returns the settled value.
func (f *Fixture) SettledSpecPods() int32 {
	return f.nodes[nodeName].SettledSpecPods()
}

func (f *Fixture) settledSpecPods() int32 {
	return f.SettledSpecPods()
}

// ReleasableCIDRs extracts CIDRs from NodeNetworkConfig.spec.releasableCIDRs for the default network.
func ReleasableCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var cidrs []string
	for _, r := range nnc.Spec.ReleasableCIDRs {
		if r.Network == network {
			cidrs = append(cidrs, r.CIDR)
		}
	}
	return cidrs
}

func releasableCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	return ReleasableCIDRs(nnc)
}

// AddReleasableCIDR appends a CIDR to NodeNetworkConfig.spec.releasableCIDRs for this node.
func (n *NodeHandle) AddReleasableCIDR(cidr string) {
	n.fixture.t.Helper()
	nnc := n.NNC().DeepCopy()
	nnc.Spec.ReleasableCIDRs = append(nnc.Spec.ReleasableCIDRs, nncv1.PodCIDR{
		Network: network,
		CIDR:    cidr,
	})
	if _, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Update(n.fixture.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		n.fixture.t.Fatalf("failed to add releasable CIDR %s on %s: %v", cidr, n.name, err)
	}
}

func (n *NodeHandle) addReleasableCIDR(cidr string) {
	n.AddReleasableCIDR(cidr)
}

// AddReleasableCIDR appends a CIDR to NodeNetworkConfig.spec.releasableCIDRs on the primary node.
func (f *Fixture) AddReleasableCIDR(cidr string) {
	f.nodes[nodeName].AddReleasableCIDR(cidr)
}

func (f *Fixture) addReleasableCIDR(cidr string) {
	f.AddReleasableCIDR(cidr)
}

// SetAllocationsPods updates NodeNetworkConfig.spec.allocations for the default network on this node.
func (n *NodeHandle) SetAllocationsPods(pods int32) {
	n.fixture.t.Helper()
	nnc := n.NNC().DeepCopy()
	found := false
	for i, a := range nnc.Spec.Allocations {
		if a.Network == network {
			nnc.Spec.Allocations[i].Pods = pods
			found = true
			break
		}
	}
	if !found {
		nnc.Spec.Allocations = append(nnc.Spec.Allocations, nncv1.Allocation{
			Network: network,
			Pods:    pods,
		})
	}
	if _, err := n.fixture.nncClient.NetworkingV1().NodeNetworkConfigs().Update(n.fixture.ctx, nnc, metav1.UpdateOptions{}); err != nil {
		n.fixture.t.Fatalf("failed to set allocations pods to %d on %s: %v", pods, n.name, err)
	}
}

func (n *NodeHandle) setAllocationsPods(pods int32) {
	n.SetAllocationsPods(pods)
}

// SetAllocationsPods updates NodeNetworkConfig.spec.allocations for the default network on the primary node.
func (f *Fixture) SetAllocationsPods(pods int32) {
	f.nodes[nodeName].SetAllocationsPods(pods)
}

func (f *Fixture) setAllocationsPods(pods int32) {
	f.SetAllocationsPods(pods)
}

// NodeEvents returns the messages of the Events recorded against the node with the given reason.
func (f *Fixture) NodeEvents(reason string) []string {
	f.t.Helper()
	list, err := f.kubeClient.CoreV1().Events("").List(f.ctx, metav1.ListOptions{})
	if err != nil {
		f.t.Fatalf("failed to list events: %v", err)
	}
	var messages []string
	for _, ev := range list.Items {
		if ev.Reason == reason && ev.InvolvedObject.Kind == "Node" && ev.InvolvedObject.Name == nodeName {
			messages = append(messages, ev.Message)
		}
	}
	return messages
}

func (f *Fixture) nodeEvents(reason string) []string {
	return f.NodeEvents(reason)
}
