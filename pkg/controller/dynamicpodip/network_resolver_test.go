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

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	networkfake "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

func gnp(name, vpc, subnet string, ranges ...string) *networkv1.GKENetworkParamSet {
	return &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       networkv1.GKENetworkParamSetSpec{VPC: vpc, VPCSubnet: subnet, PodIPv4Ranges: podRanges(ranges...)},
	}
}

// newTestResolver builds a NetworkResolver populated with L3 Networks for the
// given (network name -> GNP spec) pairs, bypassing informers.
func newTestResolver(gnps map[string]networkv1.GKENetworkParamSetSpec) *NetworkResolver {
	r := NewNetworkResolver(nil, nil)
	for name, spec := range gnps {
		r.UpsertGNP(&networkv1.GKENetworkParamSet{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-gnp"},
			Spec:       spec,
		})
		r.UpsertNetwork(readyL3Network(name, name+"-gnp"))
	}
	return r
}

// readyL3Network builds an L3 Network referencing gnpName with Ready=True, as
// NCM would publish it once routes are programmed.
func readyL3Network(name, gnpName string) *networkv1.Network {
	nw := l3Network(name, gnpName)
	nw.Status.Conditions = []metav1.Condition{{
		Type:               string(networkv1.NetworkConditionStatusReady),
		Status:             metav1.ConditionTrue,
		Reason:             "Ready",
		LastTransitionTime: metav1.Now(),
	}}
	return nw
}

// l3Network builds an L3 Network referencing gnpName with no status.
func l3Network(name, gnpName string) *networkv1.Network {
	return &networkv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: networkv1.NetworkSpec{
			Type: networkv1.L3NetworkType,
			ParametersRef: &networkv1.NetworkParametersReference{
				Group: "networking.gke.io",
				Kind:  "GKENetworkParamSet",
				Name:  gnpName,
			},
		},
	}
}

func podRanges(names ...string) *networkv1.SecondaryRanges {
	return &networkv1.SecondaryRanges{RangeNames: names}
}

// TestResolveKubernetesNetworkName pins the reverse mapping from an alias IP
// range on a GCE interface to a Kubernetes network name.
func TestResolveKubernetesNetworkName(t *testing.T) {
	const (
		vpcURL    = "https://www.googleapis.com/compute/v1/projects/p/global/networks/custom-vpc"
		subnetURL = "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/subnetworks/custom-subnet"
		attURL    = "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/networkAttachments/my-psc-att"
	)
	nic0 := &networkInterface{Name: "nic0", Network: vpcURL, Subnetwork: subnetURL}
	nic1 := &networkInterface{Name: "nic1", Network: vpcURL, Subnetwork: subnetURL}
	psc := &networkInterface{Name: "nic2", NetworkAttachment: attURL}

	resolver := newTestResolver(map[string]networkv1.GKENetworkParamSetSpec{
		"red-net":  {VPC: "custom-vpc", VPCSubnet: "custom-subnet", PodIPv4Ranges: podRanges("red-pods")},
		"blue-net": {VPC: "custom-vpc", VPCSubnet: "custom-subnet", PodIPv4Ranges: podRanges("blue-pods")},
		"psc-net":  {NetworkAttachment: "projects/p/regions/us-central1/networkAttachments/my-psc-att"},
	})

	// Networks that reference valid GNPs but have not been marked Ready by NCM.
	notReadyResolver := NewNetworkResolver(nil, nil)
	notReadyResolver.UpsertGNP(&networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: "red-gnp"},
		Spec:       networkv1.GKENetworkParamSetSpec{VPC: "custom-vpc", VPCSubnet: "custom-subnet", PodIPv4Ranges: podRanges("red-pods")},
	})
	notReadyResolver.UpsertNetwork(l3Network("red-net", "red-gnp"))
	notReadyResolver.UpsertGNP(&networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec:       networkv1.GKENetworkParamSetSpec{VPC: "custom-vpc", VPCSubnet: "custom-subnet", PodIPv4Ranges: podRanges("default-pods")},
	})
	notReadyResolver.UpsertNetwork(l3Network("default", "default"))

	tests := []struct {
		name      string
		resolver  *NetworkResolver
		iface     *networkInterface
		rangeName string
		want      string
		wantErr   bool
	}{
		{name: "nic0 without resolver is default", iface: nic0, want: "default"},
		{name: "nic0 unnamed range is default", resolver: resolver, iface: nic0, want: "default"},
		{name: "nic0 range unclaimed by any Network is default", resolver: resolver, iface: nic0, rangeName: "default-pods", want: "default"},
		{name: "nic0 range claimed by additional Network", resolver: resolver, iface: nic0, rangeName: "red-pods", want: "red-net"},
		{name: "nic1 without resolver is an error", iface: nic1, rangeName: "red-pods", wantErr: true},
		{name: "nic1 shared by two Networks, first range", resolver: resolver, iface: nic1, rangeName: "red-pods", want: "red-net"},
		{name: "nic1 shared by two Networks, second range", resolver: resolver, iface: nic1, rangeName: "blue-pods", want: "blue-net"},
		{name: "nic1 range from an unclaimed secondary range is an error", resolver: resolver, iface: nic1, rangeName: "other-pods", wantErr: true},
		{name: "nic1 range from the primary range is an error", resolver: resolver, iface: nic1, wantErr: true},
		{name: "PSC interface matches on attachment", resolver: resolver, iface: psc, want: "psc-net"},
		{name: "PSC interface with no Network is an error", resolver: newTestResolver(nil), iface: psc, wantErr: true},
		{name: "unnamed interface is an error", resolver: resolver, iface: &networkInterface{Network: vpcURL}, wantErr: true},
		{name: "not-Ready additional Network on nic1 is an error", resolver: notReadyResolver, iface: nic1, rangeName: "red-pods", wantErr: true},
		{name: "not-Ready additional Network on nic0 is an error, not default", resolver: notReadyResolver, iface: nic0, rangeName: "red-pods", wantErr: true},
		{name: "not-Ready default Network is still Ready", resolver: notReadyResolver, iface: nic0, rangeName: "default-pods", want: "default"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveKubernetesNetworkName(tc.resolver, tc.iface, tc.rangeName)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveKubernetesNetworkName() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("resolveKubernetesNetworkName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestForEachPodCIDR verifies that forEachPodCIDREntry visits all Pod CIDRs --
// alias IP ranges and AliasNetworkEndpoints alike -- on supported interfaces
// (including multi-network secondary interfaces and shared interfaces
// disambiguated by SubnetworkRangeName), attributes both kinds the same way,
// and ignores Pod CIDRs on unmanaged interfaces.
func TestForEachPodCIDR(t *testing.T) {
	resolver := newTestResolver(map[string]networkv1.GKENetworkParamSetSpec{
		// An additional network sharing nic0's subnet with the default network.
		"red-net": {VPC: "custom-vpc", VPCSubnet: "default-subnet", PodIPv4Ranges: podRanges("red-pods")},
		// Two additional networks sharing nic1.
		"blue-net":  {VPC: "secondary-vpc", VPCSubnet: "secondary-subnet", PodIPv4Ranges: podRanges("blue-pods")},
		"green-net": {VPC: "secondary-vpc", VPCSubnet: "secondary-subnet", PodIPv4Ranges: podRanges("green-pods")},
	})

	ifaces := []*networkInterface{
		{
			Name:       "nic0",
			Network:    "https://www.googleapis.com/compute/v1/projects/p/global/networks/custom-vpc",
			Subnetwork: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/subnetworks/default-subnet",
			AliasIPRanges: []aliasIPRange{
				{IPCIDRRange: "10.4.0.0/28", SubnetworkRangeName: "default-pods"},
				{IPCIDRRange: "10.4.0.16/28", SubnetworkRangeName: "red-pods"},
			},
			ANEs: []aliasNetworkEndpoint{
				{Name: "ane-a", IPCIDRRange: "10.4.0.32/32", SubnetworkRangeName: "default-pods"},
				{Name: "ane-b", IPCIDRRange: "10.4.0.33/32", SubnetworkRangeName: "red-pods"},
				// Unclaimed range on nic0 is the default network, as for alias ranges.
				{Name: "ane-c", IPCIDRRange: "10.4.0.34/32"},
			},
		},
		{
			Name:       "nic1",
			Network:    "https://www.googleapis.com/compute/v1/projects/p/global/networks/secondary-vpc",
			Subnetwork: "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/subnetworks/secondary-subnet",
			AliasIPRanges: []aliasIPRange{
				{IPCIDRRange: "10.5.0.0/28", SubnetworkRangeName: "blue-pods"},
				{IPCIDRRange: "10.6.0.0/28", SubnetworkRangeName: "green-pods"},
				{IPCIDRRange: "10.7.0.0/28", SubnetworkRangeName: "unmanaged-pods"},
			},
			ANEs: []aliasNetworkEndpoint{
				{Name: "ane-d", IPCIDRRange: "10.6.0.16/32", SubnetworkRangeName: "green-pods"},
				{Name: "ane-e", IPCIDRRange: "10.7.0.16/32", SubnetworkRangeName: "unmanaged-pods"},
			},
		},
		{
			Name:          "nic2",
			Network:       "https://www.googleapis.com/compute/v1/projects/p/global/networks/unmanaged-vpc",
			Subnetwork:    "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/subnetworks/unmanaged-subnet",
			AliasIPRanges: []aliasIPRange{{IPCIDRRange: "10.8.0.0/28"}},
			ANEs:          []aliasNetworkEndpoint{{Name: "ane-f", IPCIDRRange: "10.8.0.16/32"}},
		},
	}

	var visited []string
	b := &nodeSyncBase{networkResolver: resolver}
	b.forEachPodCIDREntry(ifaces, func(network, id, cidr string, reusePolicy nncv1.PodCIDRType) {
		visited = append(visited, fmt.Sprintf("%s:%s:%s:%s", network, id, cidr, reusePolicy))
	})

	expected := []string{
		"default:10.4.0.0/28:10.4.0.0/28:Reusable",
		"red-net:10.4.0.16/28:10.4.0.16/28:Reusable",
		"default:ane-a:10.4.0.32/32:NonReusable",
		"red-net:ane-b:10.4.0.33/32:NonReusable",
		"default:ane-c:10.4.0.34/32:NonReusable",
		"blue-net:10.5.0.0/28:10.5.0.0/28:Reusable",
		"green-net:10.6.0.0/28:10.6.0.0/28:Reusable",
		"green-net:ane-d:10.6.0.16/32:NonReusable",
	}
	if !reflect.DeepEqual(visited, expected) {
		t.Fatalf("visited = %v, want %v", visited, expected)
	}

	// forEachPodCIDR is the same walk without the ID and reuse policy.
	var plain []string
	b.forEachPodCIDR(ifaces, func(network, cidr string) {
		plain = append(plain, fmt.Sprintf("%s:%s", network, cidr))
	})
	if len(plain) != len(expected) {
		t.Fatalf("forEachPodCIDR visited %d entries, want %d: %v", len(plain), len(expected), plain)
	}
}

// TestResolveGCENetworkURL pins the forward mapping from a Kubernetes network
// name to the GCE URL that selects the right interface on a node.
func TestResolveGCENetworkURL(t *testing.T) {
	const (
		clusterNetURL = "https://www.googleapis.com/compute/v1/projects/my-net-proj/global/networks/custom-vpc"
		clusterSubURL = "https://www.googleapis.com/compute/v1/projects/my-net-proj/regions/us-central1/subnetworks/custom-subnet"
		otherSubURL   = "https://www.googleapis.com/compute/v1/projects/my-net-proj/regions/us-central1/subnetworks/other-subnet"
		// Shared VPC: the secondary VPC lives in a different host project, so
		// the interface URL cannot be derived from the cluster's own URL.
		blueNetURL = "https://www.googleapis.com/compute/v1/projects/host-proj/global/networks/blue-vpc"
		blueSubURL = "https://www.googleapis.com/compute/v1/projects/host-proj/regions/us-central1/subnetworks/blue-subnet"
		attURL     = "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/networkAttachments/my-psc-att"
	)
	values := gce.DefaultTestClusterValues()
	values.NetworkURL = clusterNetURL
	cloud := gce.NewFakeGCECloud(values)

	emptyValues := gce.DefaultTestClusterValues()
	emptyValues.NetworkURL = ""
	emptyCloud := gce.NewFakeGCECloud(emptyValues)

	resolver := newTestResolver(map[string]networkv1.GKENetworkParamSetSpec{
		"default":  {VPC: "custom-vpc", VPCSubnet: "custom-subnet", PodIPv4Ranges: podRanges("default-pods")},
		"blue-net": {VPC: "blue-vpc", VPCSubnet: "blue-subnet", PodIPv4Ranges: podRanges("blue-pods")},
		"psc-net":  {NetworkAttachment: "projects/p/regions/us-central1/networkAttachments/my-psc-att"},
	})

	notReadyResolver := NewNetworkResolver(nil, nil)
	notReadyResolver.UpsertGNP(&networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: "blue-gnp"},
		Spec:       networkv1.GKENetworkParamSetSpec{VPC: "blue-vpc", VPCSubnet: "blue-subnet", PodIPv4Ranges: podRanges("blue-pods")},
	})
	notReadyResolver.UpsertNetwork(l3Network("blue-net", "blue-gnp"))

	nic0 := &networkInterface{Name: "nic0", Network: clusterNetURL, Subnetwork: clusterSubURL}
	nic0OtherSubnet := &networkInterface{Name: "nic0", Network: clusterNetURL, Subnetwork: otherSubURL}
	nic1 := &networkInterface{Name: "nic1", Network: blueNetURL, Subnetwork: blueSubURL}
	psc := &networkInterface{Name: "nic2", NetworkAttachment: attURL}

	tests := []struct {
		name     string
		cloud    *gce.Cloud
		resolver *NetworkResolver
		netName  string
		ifaces   []*networkInterface
		want     string
		wantErr  bool
	}{
		{name: "default without resolver", cloud: cloud, netName: "default", ifaces: []*networkInterface{nic0}, want: clusterNetURL},
		{name: "empty name is default", cloud: cloud, netName: "", ifaces: []*networkInterface{nic0}, want: clusterNetURL},
		{name: "default with matching GNP returns interface URL", cloud: cloud, resolver: resolver, netName: "default", ifaces: []*networkInterface{nic0, nic1}, want: clusterNetURL},
		{name: "default on a node in another subnet falls back to cluster URL", cloud: cloud, resolver: resolver, netName: "default", ifaces: []*networkInterface{nic0OtherSubnet}, want: clusterNetURL},
		{name: "additional network returns its interface URL", cloud: cloud, resolver: resolver, netName: "blue-net", ifaces: []*networkInterface{nic0, nic1}, want: blueNetURL},
		{name: "PSC network returns attachment URL", cloud: cloud, resolver: resolver, netName: "psc-net", ifaces: []*networkInterface{nic0, psc}, want: attURL},
		{name: "additional network with no matching interface", cloud: cloud, resolver: resolver, netName: "blue-net", ifaces: []*networkInterface{nic0}, wantErr: true},
		{name: "unknown network", cloud: cloud, resolver: resolver, netName: "green-net", ifaces: []*networkInterface{nic0}, wantErr: true},
		{name: "not-Ready network", cloud: cloud, resolver: notReadyResolver, netName: "blue-net", ifaces: []*networkInterface{nic0, nic1}, wantErr: true},
		{name: "unknown network without resolver", cloud: cloud, netName: "blue-net", ifaces: []*networkInterface{nic0, nic1}, wantErr: true},
		{name: "nil cloud", resolver: resolver, netName: "default", ifaces: []*networkInterface{nic0}, wantErr: true},
		{name: "default with unconfigured cluster URL", cloud: emptyCloud, netName: "default", ifaces: []*networkInterface{nic0}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveGCENetworkURL(tc.cloud, tc.resolver, tc.netName, tc.ifaces)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveGCENetworkURL() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("resolveGCENetworkURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGCECIDRSet verifies that gceCIDRSet indexes alias IP ranges attached to the
// primary interface (nic0) under the "default" network key, indexes configured secondary
// networks under their Kubernetes Network name, and excludes ranges from unmanaged
// secondary interfaces.
func TestGCECIDRSet(t *testing.T) {
	ifaces := []*networkInterface{
		{
			Name:          "nic0",
			Network:       "https://www.googleapis.com/compute/v1/projects/p/global/networks/custom-vpc",
			AliasIPRanges: []aliasIPRange{{IPCIDRRange: "10.4.0.0/28"}},
		},
		{
			Name:          "nic1",
			Network:       "https://www.googleapis.com/compute/v1/projects/p/global/networks/secondary-net",
			AliasIPRanges: []aliasIPRange{{IPCIDRRange: "10.5.0.0/28"}},
		},
	}

	active := (&nodeSyncBase{}).gceCIDRSet(ifaces)
	if !active.Has("default/10.4.0.0/28") {
		t.Errorf("expected nic0 range to be in active set")
	}
	if active.Has("default/10.5.0.0/28") || active.Has("secondary-net/10.5.0.0/28") {
		t.Errorf("nic1 range should be ignored when unconfigured, but was in active set: %v", active)
	}
}

// TestNetworkResolver_IndexLifecycle drives the index directly through the
// upsert/delete entry points the informer handlers call, and checks that each
// transition is reflected in both the forward and reverse lookups.
func TestNetworkResolver_IndexLifecycle(t *testing.T) {
	r := NewNetworkResolver(nil, nil)

	lookup := func() (fwd bool, rev string, revOK bool) {
		_, fwd = r.LookupNetworkParams("blue-net")
		rev, _, revOK = r.networkByVPCSubnetRange("blue-vpc", "blue-subnet", "blue-pods")
		return
	}

	// Network alone: its GNP does not exist yet, so nothing resolves.
	r.UpsertNetwork(readyL3Network("blue-net", "blue-gnp"))
	if fwd, _, rev := lookup(); fwd || rev {
		t.Fatalf("Network without GNP should not be indexed (fwd=%v rev=%v)", fwd, rev)
	}

	// GNP arrives: both directions resolve.
	r.UpsertGNP(gnp("blue-gnp", "blue-vpc", "blue-subnet", "blue-pods"))
	if fwd, rev, ok := lookup(); !fwd || !ok || rev != "blue-net" {
		t.Fatalf("expected blue-net to be indexed, got fwd=%v rev=%q ok=%v", fwd, rev, ok)
	}

	// GNP spec changes to a different range: old key gone, new key present.
	r.UpsertGNP(gnp("blue-gnp", "blue-vpc", "blue-subnet", "blue-pods-v2"))
	if _, rev, ok := lookup(); ok {
		t.Errorf("stale range key should have been dropped on GNP update, still maps to %q", rev)
	}
	if rev, _, ok := r.networkByVPCSubnetRange("blue-vpc", "blue-subnet", "blue-pods-v2"); !ok || rev != "blue-net" {
		t.Errorf("new range key not indexed after GNP update: %q %v", rev, ok)
	}

	// Network repointed to a GNP that does not exist: disappears.
	r.UpsertNetwork(readyL3Network("blue-net", "missing-gnp"))
	if _, ok := r.LookupNetworkParams("blue-net"); ok {
		t.Errorf("Network referencing a missing GNP should not be indexed")
	}
	r.UpsertNetwork(readyL3Network("blue-net", "blue-gnp"))

	// Readiness flips are reflected without re-indexing anything else.
	notReady := l3Network("blue-net", "blue-gnp")
	r.UpsertNetwork(notReady)
	if p, ok := r.LookupNetworkParams("blue-net"); !ok || p.Ready {
		t.Errorf("expected blue-net indexed but not Ready, got ok=%v ready=%v", ok, p.Ready)
	}
	if _, ready, ok := r.networkByVPCSubnetRange("blue-vpc", "blue-subnet", "blue-pods-v2"); !ok || ready {
		t.Errorf("reverse lookup should still claim the range but report not Ready (ok=%v ready=%v)", ok, ready)
	}

	// Deleting the GNP removes the Network from the index; deleting the
	// Network afterwards must not panic on an already-absent entry.
	r.DeleteGNP("blue-gnp")
	if _, ok := r.LookupNetworkParams("blue-net"); ok {
		t.Errorf("Network should be dropped when its GNP is deleted")
	}
	r.DeleteNetwork("blue-net")
	r.DeleteNetwork("blue-net")

	// Device networks and device-mode GNPs are never indexed.
	r.UpsertGNP(&networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{Name: "dev-gnp"},
		Spec:       networkv1.GKENetworkParamSetSpec{VPC: "v", VPCSubnet: "s", DeviceMode: networkv1.NetDevice},
	})
	dev := readyL3Network("dev-net", "dev-gnp")
	dev.Spec.Type = networkv1.DeviceNetworkType
	r.UpsertNetwork(dev)
	if _, ok := r.LookupNetworkParams("dev-net"); ok {
		t.Errorf("device network should not be indexed")
	}
}

// TestNetworkResolver_DuplicateClaimIsDeterministic pins that when two
// Networks claim the same secondary range, the lexically first wins regardless
// of insertion order, so reconciles are stable across restarts.
func TestNetworkResolver_DuplicateClaimIsDeterministic(t *testing.T) {
	for _, order := range [][]string{{"a-net", "b-net"}, {"b-net", "a-net"}} {
		r := NewNetworkResolver(nil, nil)
		for _, n := range order {
			r.UpsertGNP(gnp(n+"-gnp", "vpc", "subnet", "shared-pods"))
			r.UpsertNetwork(readyL3Network(n, n+"-gnp"))
		}
		if got, _, _ := r.networkByVPCSubnetRange("vpc", "subnet", "shared-pods"); got != "a-net" {
			t.Errorf("insertion order %v: winner = %q, want a-net", order, got)
		}
	}
}

// TestNetworkResolver_InformerDriven checks the informer wiring end to end:
// handler registration, Synced() covering initial delivery, and add/update/
// delete events (including the tombstone path) reaching the index.
func TestNetworkResolver_InformerDriven(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := networkfake.NewSimpleClientset(
		gnp("blue-gnp", "blue-vpc", "blue-subnet", "blue-pods"),
		readyL3Network("blue-net", "blue-gnp"),
	)
	factory := networkinformers.NewSharedInformerFactory(client, 0)
	r := NewNetworkResolver(factory.Networking().V1().Networks(), factory.Networking().V1().GKENetworkParamSets())
	if len(r.Synced()) != 2 {
		t.Fatalf("expected 2 sync funcs, got %d", len(r.Synced()))
	}
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), r.Synced()...) {
		t.Fatal("resolver did not sync")
	}

	// Pre-existing objects are indexed by the time Synced() reports true.
	if _, ok := r.LookupNetworkParams("blue-net"); !ok {
		t.Fatal("pre-existing Network not indexed after sync")
	}

	eventually := func(desc string, cond func() bool) {
		t.Helper()
		if err := wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 5*time.Second, true, func(context.Context) (bool, error) { return cond(), nil }); err != nil {
			t.Fatalf("timed out waiting for %s", desc)
		}
	}

	// Update: readiness removed.
	nw, _ := client.NetworkingV1().Networks().Get(ctx, "blue-net", metav1.GetOptions{})
	nw.Status.Conditions = nil
	if _, err := client.NetworkingV1().Networks().UpdateStatus(ctx, nw, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("Network to become not Ready", func() bool {
		p, ok := r.LookupNetworkParams("blue-net")
		return ok && !p.Ready
	})

	// Delete GNP: Network disappears from the index.
	if err := client.NetworkingV1().GKENetworkParamSets().Delete(ctx, "blue-gnp", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	eventually("Network to be dropped after GNP delete", func() bool {
		_, ok := r.LookupNetworkParams("blue-net")
		return !ok
	})

	// Tombstone path: handlers must unwrap DeletedFinalStateUnknown.
	r.UpsertGNP(gnp("blue-gnp", "blue-vpc", "blue-subnet", "blue-pods"))
	if _, ok := r.LookupNetworkParams("blue-net"); !ok {
		t.Fatal("expected blue-net back after re-adding GNP")
	}
	r.handleNetworkDelete(cache.DeletedFinalStateUnknown{Key: "blue-net", Obj: readyL3Network("blue-net", "blue-gnp")})
	if _, ok := r.LookupNetworkParams("blue-net"); ok {
		t.Error("tombstoned Network delete was not applied")
	}
}
