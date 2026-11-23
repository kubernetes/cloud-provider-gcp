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

package dynamicpodip

// Tests for the status controller's ownership of NodeNetworkConfig existence:
//
//	nncWanted(node) = specControllerEnabled
//	               || node has >= 1 Ready additional Network matching one of its interfaces
//
// Create when wanted and missing; delete when not wanted, present, and ours.

import (
	"context"
	"fmt"
	"sort"
	"testing"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNodeUID = types.UID("node-uid-1234")

	// A second VPC/subnet carrying an additional Network.
	testVPCB       = "vpc-b"
	testSubnetB    = "subnet-b"
	testPodRangeB  = "pods-b"
	testNetworkB   = "net-b"
	testGNPB       = "net-b-gnp"
	testPodCIDRB   = "10.200.0.0/28"
	testDefaultPod = "10.100.0.0/28"
)

var (
	testVPCBURL    = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", testProject, testVPCB)
	testSubnetBURL = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1/subnetworks/%s", testProject, testSubnetB)
)

// createTestNode creates the Node that backs testNodeName, with a UID so that
// owner references can be checked.
func createTestNode(ctx context.Context, t *testing.T, f *testFixture) *corev1.Node {
	t.Helper()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, UID: testNodeUID},
		Spec:       corev1.NodeSpec{ProviderID: testProviderID},
	}
	created, err := f.kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Node: %v", err)
	}
	return created
}

// insertTwoNICInstance creates a fake GCE instance with nic0 on the cluster
// network carrying testDefaultPod, and nic1 in (testVPCB, testSubnetB)
// carrying the given ranges from testPodRangeB.
func insertTwoNICInstance(ctx context.Context, t *testing.T, f *testFixture, nic1CIDRs ...string) {
	t.Helper()
	var nic1Ranges []*compute.AliasIpRange
	for _, cidr := range nic1CIDRs {
		nic1Ranges = append(nic1Ranges, &compute.AliasIpRange{IpCidrRange: cidr, SubnetworkRangeName: testPodRangeB})
	}
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:          "nic0",
				Network:       testNetworkURL,
				Subnetwork:    "default",
				AliasIpRanges: []*compute.AliasIpRange{{IpCidrRange: testDefaultPod}},
			},
			{
				Name:          "nic1",
				Network:       testVPCBURL,
				Subnetwork:    testSubnetBURL,
				AliasIpRanges: nic1Ranges,
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, meta.ZonalKey(testNodeName, testZone), instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}
}

// addNetworkB registers the additional Network testNetworkB (and its GNP) in
// the resolver, Ready or not.
func addNetworkB(f *testFixture, ready bool) {
	f.networkResolver.UpsertGNP(gnp(testGNPB, testVPCB, testSubnetB, testPodRangeB))
	if ready {
		f.networkResolver.UpsertNetwork(readyL3Network(testNetworkB, testGNPB))
	} else {
		f.networkResolver.UpsertNetwork(l3Network(testNetworkB, testGNPB))
	}
}

func getNNC(ctx context.Context, t *testing.T, f *testFixture) (*nncv1.NodeNetworkConfig, bool) {
	t.Helper()
	nnc, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false
		}
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	return nnc, true
}

func publishedCIDRs(nnc *nncv1.NodeNetworkConfig) []string {
	var got []string
	for _, pc := range nnc.Status.PodCIDRs {
		got = append(got, cidrKey(pc.Network, pc.CIDR))
	}
	sort.Strings(got)
	return got
}

func TestStatusController_CreatesNNCForReadyAdditionalNetwork(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureMultiNetworkOnly(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	node := createTestNode(ctx, t, f)
	insertTwoNICInstance(ctx, t, f, testPodCIDRB)
	addNetworkB(f, true)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}

	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected a NodeNetworkConfig to be created")
	}
	if !isManagedByCCM(nnc) {
		t.Errorf("Expected label %s=%s, got labels %v", ManagedByLabelKey, ManagedByLabelValue, nnc.Labels)
	}
	if len(nnc.OwnerReferences) != 1 {
		t.Fatalf("Expected exactly one owner reference, got %v", nnc.OwnerReferences)
	}
	owner := nnc.OwnerReferences[0]
	if owner.Kind != "Node" || owner.Name != node.Name || owner.UID != node.UID || owner.Controller == nil || !*owner.Controller {
		t.Errorf("Unexpected owner reference: %+v", owner)
	}
	if owner.BlockOwnerDeletion != nil {
		t.Errorf("blockOwnerDeletion must not be set (got %v): it would hold up Node deletion and needs nodes/finalizers", *owner.BlockOwnerDeletion)
	}

	// Status is published in the same pass as creation: default on nic0 plus
	// the additional network's range on nic1.
	want := []string{cidrKey("default", testDefaultPod), cidrKey(testNetworkB, testPodCIDRB)}
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

// A Ready Network whose interface has no alias range attached yet still
// warrants a NodeNetworkConfig: the object is where the ranges will be
// published once they arrive.
func TestStatusController_CreatesNNCBeforeRangesExist(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureMultiNetworkOnly(t)

	createTestNode(ctx, t, f)
	insertTwoNICInstance(ctx, t, f /* no nic1 ranges */)
	addNetworkB(f, true)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected a NodeNetworkConfig to be created")
	}
	want := []string{cidrKey("default", testDefaultPod)}
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

func TestStatusController_NoNNCWithoutReadyAdditionalNetwork(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name  string
		setup func(f *testFixture)
	}{
		{
			name:  "no additional Network at all",
			setup: func(f *testFixture) {},
		},
		{
			// The default Network CR exists in every multi-network cluster and
			// is always considered Ready; it must not count.
			name: "only the default Network",
			setup: func(f *testFixture) {
				f.networkResolver.UpsertGNP(gnp("default-gnp", "default", "default", "pods"))
				f.networkResolver.UpsertNetwork(readyL3Network(networkv1.DefaultPodNetworkName, "default-gnp"))
			},
		},
		{
			name:  "additional Network exists but is not Ready",
			setup: func(f *testFixture) { addNetworkB(f, false) },
		},
		{
			// A Ready Network the node has no interface for is somebody else's.
			name: "additional Network is Ready but matches no interface",
			setup: func(f *testFixture) {
				f.networkResolver.UpsertGNP(gnp("other-gnp", "vpc-other", "subnet-other", "pods-other"))
				f.networkResolver.UpsertNetwork(readyL3Network("net-other", "other-gnp"))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixtureMultiNetworkOnly(t)
			createTestNode(ctx, t, f)
			insertTwoNICInstance(ctx, t, f, testPodCIDRB)
			tc.setup(f)

			if err := f.statusCtrl.syncNode(testNodeName); err != nil {
				t.Fatalf("syncNode failed: %v", err)
			}
			if _, ok := getNNC(ctx, t, f); ok {
				t.Errorf("Expected no NodeNetworkConfig to be created")
			}
		})
	}
}

// With no resolver (--enable-multi-networking off) and no spec controller,
// nothing is ever wanted.
func TestStatusController_NoNNCWithoutResolver(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureWithOptions(t, testNetworkURL, false, false)
	createTestNode(ctx, t, f)
	insertInstanceWithRanges(ctx, t, f, testDefaultPod)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	if _, ok := getNNC(ctx, t, f); ok {
		t.Errorf("Expected no NodeNetworkConfig to be created")
	}
}

// When the spec controller is enabled every node wants a NodeNetworkConfig,
// additional Networks or not, and nothing is ever deleted.
func TestStatusController_AlwaysWantedWithSpecController(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t) // nncAlwaysWanted
	createTestNode(ctx, t, f)
	insertInstanceWithRanges(ctx, t, f, testDefaultPod)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected a NodeNetworkConfig to be created")
	}
	if !isManagedByCCM(nnc) {
		t.Errorf("Expected the created object to carry the managed-by label")
	}
	want := []string{cidrKey("default", testDefaultPod)}
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}

	// A second pass with nothing changed is a no-op, not a delete.
	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("second syncNode failed: %v", err)
	}
	if _, ok := getNNC(ctx, t, f); !ok {
		t.Errorf("NodeNetworkConfig must not be deleted while the spec controller is enabled")
	}
}

// The rollback path: the Network the node participated in goes Ready=False
// (as NCM does when MultiNetwork::IPv6 is turned off) or disappears, and the
// NodeNetworkConfig the CCM created goes with it.
func TestStatusController_DeletesOwnNNCWhenNetworkGoesAway(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name   string
		remove func(f *testFixture)
	}{
		{name: "Network becomes not Ready", remove: func(f *testFixture) { addNetworkB(f, false) }},
		{name: "Network is deleted", remove: func(f *testFixture) { f.networkResolver.DeleteNetwork(testNetworkB) }},
		{name: "GKENetworkParamSet is deleted", remove: func(f *testFixture) { f.networkResolver.DeleteGNP(testGNPB) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixtureMultiNetworkOnly(t)
			createTestNode(ctx, t, f)
			insertTwoNICInstance(ctx, t, f, testPodCIDRB)
			addNetworkB(f, true)

			if err := f.statusCtrl.syncNode(testNodeName); err != nil {
				t.Fatalf("initial syncNode failed: %v", err)
			}
			if _, ok := getNNC(ctx, t, f); !ok {
				t.Fatalf("Expected a NodeNetworkConfig to be created")
			}

			gceWrites := countGCEWrites(t, f)
			tc.remove(f)
			if err := f.statusCtrl.syncNode(testNodeName); err != nil {
				t.Fatalf("syncNode after removal failed: %v", err)
			}
			if _, ok := getNNC(ctx, t, f); ok {
				t.Errorf("Expected the NodeNetworkConfig to be deleted")
			}
			// Lifecycle never touches GCE: the alias ranges stay where they are.
			if *gceWrites != 0 {
				t.Errorf("Expected no GCE writes, got %d", *gceWrites)
			}
		})
	}
}

// A NodeNetworkConfig the CCM did not create -- for instance one left behind
// by Metis -- is never deleted, even when the node wants none. Its status is
// still kept current.
func TestStatusController_LeavesForeignNNCAlone(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureMultiNetworkOnly(t)
	createTestNode(ctx, t, f)
	insertInstanceWithRanges(ctx, t, f, testDefaultPod)

	foreign := &nncv1.NodeNetworkConfig{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, foreign, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("A NodeNetworkConfig without the managed-by label must not be deleted")
	}
	if isManagedByCCM(nnc) {
		t.Errorf("The controller must not adopt a foreign NodeNetworkConfig")
	}
	want := []string{cidrKey("default", testDefaultPod)}
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

// Two workers, or the CCM and Metis, may race to create the same object. The
// loser must carry on with the winner's object rather than fail the sync.
func TestStatusController_CreateToleratesAlreadyExists(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureMultiNetworkOnly(t)
	createTestNode(ctx, t, f)
	insertTwoNICInstance(ctx, t, f, testPodCIDRB)
	addNetworkB(f, true)

	// Simulate a concurrent creator: the object lands in the tracker between
	// our Get (NotFound) and our Create (AlreadyExists).
	gvr := schema.GroupVersionResource{Group: "networking.gke.io", Version: "v1", Resource: "nodenetworkconfigs"}
	f.nncClient.PrependReactor("create", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		existing := &nncv1.NodeNetworkConfig{ObjectMeta: metav1.ObjectMeta{Name: testNodeName, UID: "winner"}}
		if err := f.nncClient.Tracker().Add(existing); err != nil {
			t.Fatalf("Failed to add concurrent object: %v", err)
		}
		return true, nil, apierrors.NewAlreadyExists(gvr.GroupResource(), testNodeName)
	})

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected the concurrently created NodeNetworkConfig to exist")
	}
	if nnc.UID != "winner" {
		t.Errorf("Expected to continue with the concurrently created object, got UID %q", nnc.UID)
	}
	want := []string{cidrKey("default", testDefaultPod), cidrKey(testNetworkB, testPodCIDRB)}
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

// A Network or GKENetworkParamSet change is invisible to the Node watch, so
// the resolver's change hook must re-enqueue every node.
func TestStatusController_NetworkChangeEnqueuesAllNodes(t *testing.T) {
	ctx := context.Background()
	f := newTestFixtureMultiNetworkOnly(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	nodeNames := []string{"node-a", "node-b", "node-c"}
	for _, name := range nodeNames {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: corev1.NodeSpec{ProviderID: "gce://p/z/" + name}}
		if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Node %q: %v", name, err)
		}
	}
	f.runNodeInformer(stopCh)

	// Drain whatever the Node informer's own Add events enqueued.
	for f.statusCtrl.queue.Len() > 0 {
		key, _ := f.statusCtrl.queue.Get()
		f.statusCtrl.queue.Done(key)
		f.statusCtrl.queue.Forget(key)
	}

	addNetworkB(f, true)

	var got []string
	for f.statusCtrl.queue.Len() > 0 {
		key, _ := f.statusCtrl.queue.Get()
		got = append(got, key)
		f.statusCtrl.queue.Done(key)
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(nodeNames) {
		t.Errorf("Enqueued %v after a Network change, want %v", got, nodeNames)
	}
}

func TestNetworkResolver_ReadyAdditionalNetworks(t *testing.T) {
	r := NewNetworkResolver(nil, nil)
	r.UpsertGNP(gnp("default-gnp", "default", "default", "pods"))
	r.UpsertNetwork(l3Network(networkv1.DefaultPodNetworkName, "default-gnp")) // Ready by fiat, but excluded
	r.UpsertGNP(gnp("legacy-gnp", "default", "default", "pods-legacy"))
	r.UpsertNetwork(l3Network(networkv1.DefaultNetworkName, "legacy-gnp")) // legacy default name, also excluded
	r.UpsertGNP(gnp("b-gnp", "vpc-b", "subnet-b", "pods-b"))
	r.UpsertNetwork(readyL3Network("net-b", "b-gnp"))
	r.UpsertGNP(gnp("c-gnp", "vpc-c", "subnet-c", "pods-c"))
	r.UpsertNetwork(l3Network("net-c", "c-gnp")) // not Ready
	r.UpsertGNP(gnp("a-gnp", "vpc-a", "subnet-a", "pods-a"))
	r.UpsertNetwork(readyL3Network("net-a", "a-gnp"))

	var got []string
	for _, p := range r.ReadyAdditionalNetworks() {
		got = append(got, p.NetworkName)
	}
	want := []string{"net-a", "net-b"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ReadyAdditionalNetworks = %v, want %v", got, want)
	}

	var nilResolver *NetworkResolver
	if got := nilResolver.ReadyAdditionalNetworks(); got != nil {
		t.Errorf("nil resolver returned %v", got)
	}
}

func TestNetworkResolver_OnChangeFiresOutsideLock(t *testing.T) {
	var r *NetworkResolver
	calls := 0
	r = NewNetworkResolver(nil, nil, func() {
		calls++
		// Calling back into the resolver must not deadlock.
		_ = r.ReadyAdditionalNetworks()
	})

	r.UpsertGNP(gnp("b-gnp", "vpc-b", "subnet-b", "pods-b"))
	r.UpsertNetwork(readyL3Network("net-b", "b-gnp"))
	r.DeleteNetwork("net-b")
	r.DeleteGNP("b-gnp")
	if calls != 4 {
		t.Errorf("Expected 4 onChange calls, got %d", calls)
	}

	var nilResolver *NetworkResolver
	nilResolver.DeleteNetwork("x") // must not panic
}
