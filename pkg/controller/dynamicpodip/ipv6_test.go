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

// Tests for IPv6 pod range publication: for each Ready Network whose StackType
// includes IPv6, the first /112 of the IPv6 /96 on the interface carrying that
// Network is published into Status.PodCIDRs.

import (
	"context"
	"fmt"
	"sort"
	"testing"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// The /96 GCE assigns to nic1; its first address is the interface address.
	testNIC1IPv6     = "2600:1900:4000:1:0:1::"
	testNIC1IPv6Pods = "2600:1900:4000:1:0:1::/112"
	testNIC0IPv6     = "2600:1900:4000:0:0:7::"
	testNIC0IPv6Pods = "2600:1900:4000::7:0:0/112" // net.IP canonical form of 2600:1900:4000:0:0:7::/112
)

func TestIPv6PodRange(t *testing.T) {
	cases := []struct {
		addr string
		want string
		ok   bool
	}{
		{addr: "2600:1900:4000:1:0:1::", want: "2600:1900:4000:1:0:1::/112", ok: true},
		// Only the /96's first /112 is the pod range, whatever the host bits say.
		{addr: "2600:1900:4000:1:0:1:0:1", want: "2600:1900:4000:1:0:1::/112", ok: true},
		{addr: "2600:1900:4000:1:0:1:ffff:ffff", want: "2600:1900:4000:1:0:1:ffff:0/112", ok: true},
		{addr: "", ok: false},
		{addr: "10.0.0.1", ok: false},
		{addr: "not-an-ip", ok: false},
	}
	for _, tc := range cases {
		got, ok := ipv6PodRange(tc.addr)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ipv6PodRange(%q) = (%q, %v), want (%q, %v)", tc.addr, got, ok, tc.want, tc.ok)
		}
	}
}

func TestInterfaceIPv6Address(t *testing.T) {
	cases := []struct {
		name  string
		iface *computebeta.NetworkInterface
		want  string
	}{
		{name: "internal", iface: &computebeta.NetworkInterface{Ipv6Address: testNIC1IPv6}, want: testNIC1IPv6},
		{
			name: "external access type uses the access config",
			iface: &computebeta.NetworkInterface{
				Ipv6AccessType:    "EXTERNAL",
				Ipv6AccessConfigs: []*computebeta.AccessConfig{{ExternalIpv6: testNIC1IPv6}},
			},
			want: testNIC1IPv6,
		},
		{
			name:  "internal wins when both are present",
			iface: &computebeta.NetworkInterface{Ipv6Address: testNIC0IPv6, Ipv6AccessType: "EXTERNAL", Ipv6AccessConfigs: []*computebeta.AccessConfig{{ExternalIpv6: testNIC1IPv6}}},
			want:  testNIC0IPv6,
		},
		{
			name:  "access config ignored unless access type is EXTERNAL",
			iface: &computebeta.NetworkInterface{Ipv6AccessConfigs: []*computebeta.AccessConfig{{ExternalIpv6: testNIC1IPv6}}},
			want:  "",
		},
		{name: "none", iface: &computebeta.NetworkInterface{}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := interfaceIPv6Address(tc.iface); got != tc.want {
				t.Errorf("interfaceIPv6Address = %q, want %q", got, tc.want)
			}
			// toNetworkInterfaces and deepCopyInterfaces must carry it through.
			ifaces := deepCopyInterfaces(toNetworkInterfaces([]*computebeta.NetworkInterface{tc.iface}))
			if got := ifaces[0].IPv6Address; got != tc.want {
				t.Errorf("IPv6Address after conversion and copy = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNetworkResolver_StackType(t *testing.T) {
	r := NewNetworkResolver(nil, nil)
	r.UpsertGNP(gnp("g", "vpc", "subnet", "pods"))

	cases := []struct {
		set  networkv1.StackType
		want networkv1.StackType
	}{
		{set: "", want: networkv1.IPv4StackType},
		{set: networkv1.IPv4StackType, want: networkv1.IPv4StackType},
		{set: networkv1.IPv6StackType, want: networkv1.IPv6StackType},
		{set: networkv1.DualStackType, want: networkv1.DualStackType},
	}
	for _, tc := range cases {
		nw := readyL3Network("n", "g")
		nw.Spec.StackType = tc.set
		r.UpsertNetwork(nw)
		params, ok := r.LookupNetworkParams("n")
		if !ok {
			t.Fatalf("network not indexed")
		}
		if params.StackType != tc.want {
			t.Errorf("StackType for spec %q = %q, want %q", tc.set, params.StackType, tc.want)
		}
	}
}

// ipv6Fixture builds a multi-network-only fixture with a two-NIC instance
// where nic0 carries the default IPv4 range and nic1 carries an IPv4 range from
// testPodRangeB plus an IPv6 address, and registers testNetworkB with the
// given stack type and readiness.
func ipv6Fixture(ctx context.Context, t *testing.T, stackType networkv1.StackType, ready bool, nic0IPv6, nic1IPv6 string) *testFixture {
	t.Helper()
	f := newTestFixtureMultiNetworkOnly(t)
	createTestNode(ctx, t, f)

	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:          "nic0",
				Network:       testNetworkURL,
				Subnetwork:    "default",
				AliasIpRanges: []*compute.AliasIpRange{{IpCidrRange: testDefaultPod}},
				Ipv6Address:   nic0IPv6,
			},
			{
				Name:          "nic1",
				Network:       testVPCBURL,
				Subnetwork:    testSubnetBURL,
				AliasIpRanges: []*compute.AliasIpRange{{IpCidrRange: testPodCIDRB, SubnetworkRangeName: testPodRangeB}},
				Ipv6Address:   nic1IPv6,
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, meta.ZonalKey(testNodeName, testZone), instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	f.networkResolver.UpsertGNP(gnp(testGNPB, testVPCB, testSubnetB, testPodRangeB))
	var nw *networkv1.Network
	if ready {
		nw = readyL3Network(testNetworkB, testGNPB)
	} else {
		nw = l3Network(testNetworkB, testGNPB)
	}
	nw.Spec.StackType = stackType
	f.networkResolver.UpsertNetwork(nw)
	return f
}

func TestStatusController_PublishesIPv6PodRange(t *testing.T) {
	ctx := context.Background()
	ipv4Only := []string{cidrKey("default", testDefaultPod), cidrKey(testNetworkB, testPodCIDRB)}
	withIPv6 := append(append([]string{}, ipv4Only...), cidrKey(testNetworkB, testNIC1IPv6Pods))

	cases := []struct {
		name      string
		stackType networkv1.StackType
		ready     bool
		nic1IPv6  string
		want      []string
	}{
		{name: "dual-stack Network", stackType: networkv1.DualStackType, ready: true, nic1IPv6: testNIC1IPv6, want: withIPv6},
		{name: "IPv6-only Network", stackType: networkv1.IPv6StackType, ready: true, nic1IPv6: testNIC1IPv6, want: withIPv6},
		// A dual-stack subnet can host a Network that is deliberately IPv4-only.
		{name: "IPv4 Network on an interface with IPv6", stackType: networkv1.IPv4StackType, ready: true, nic1IPv6: testNIC1IPv6, want: ipv4Only},
		{name: "unset StackType means IPv4", stackType: "", ready: true, nic1IPv6: testNIC1IPv6, want: ipv4Only},
		{name: "dual-stack Network but no IPv6 on the interface", stackType: networkv1.DualStackType, ready: true, nic1IPv6: "", want: ipv4Only},
		// Rollback: Ready=False withholds the Network's IPv4 and IPv6 ranges alike.
		{name: "dual-stack Network not Ready", stackType: networkv1.DualStackType, ready: false, nic1IPv6: testNIC1IPv6, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := ipv6Fixture(ctx, t, tc.stackType, tc.ready, "", tc.nic1IPv6)
			if tc.want == nil {
				// Nothing to publish and no Ready additional Network: the
				// status controller creates nothing. Pre-create a foreign NNC
				// so that what *would* be published is observable.
				nnc := &nncv1.NodeNetworkConfig{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
				if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
					t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
				}
				tc.want = []string{cidrKey("default", testDefaultPod)}
			}

			if err := f.statusCtrl.syncNode(testNodeName); err != nil {
				t.Fatalf("syncNode failed: %v", err)
			}
			nnc, ok := getNNC(ctx, t, f)
			if !ok {
				t.Fatalf("Expected a NodeNetworkConfig")
			}
			if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(sortedCopy(tc.want)) {
				t.Errorf("Published CIDRs = %v, want %v", got, sortedCopy(tc.want))
			}
			for _, pc := range nnc.Status.PodCIDRs {
				if pc.Condition == nil || pc.Condition.Status != metav1.ConditionTrue {
					t.Errorf("PodCIDR %s/%s is not Ready: %+v", pc.Network, pc.CIDR, pc.Condition)
				}
				// Neither alias ranges nor IPv6 /112s are ANEs: the reuse
				// policy says only which GCE resource backs the CIDR.
				if pc.ReusePolicy != nncv1.PodCIDRTypeReusable {
					t.Errorf("PodCIDR %s/%s ReusePolicy = %q, want %q", pc.Network, pc.CIDR, pc.ReusePolicy, nncv1.PodCIDRTypeReusable)
				}
				if pc.Id != pc.CIDR {
					t.Errorf("PodCIDR %s/%s Id = %q, want the CIDR", pc.Network, pc.CIDR, pc.Id)
				}
			}
		})
	}
}

// The default network's IPv6 range is published under the same rule, with the
// nic0 fall back that IPv4 attribution has.
func TestStatusController_PublishesDefaultIPv6PodRange(t *testing.T) {
	ctx := context.Background()
	f := ipv6Fixture(ctx, t, networkv1.DualStackType, true, testNIC0IPv6, testNIC1IPv6)

	// The default GNP names a subnet this node is not in, as happens for
	// node pools in non-default subnets; nic0 is still the default interface.
	f.networkResolver.UpsertGNP(gnp("default-gnp", "default", "some-other-subnet", "pods"))
	defaultNW := readyL3Network(networkv1.DefaultPodNetworkName, "default-gnp")
	defaultNW.Spec.StackType = networkv1.DualStackType
	f.networkResolver.UpsertNetwork(defaultNW)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected a NodeNetworkConfig")
	}
	want := sortedCopy([]string{
		cidrKey("default", testDefaultPod),
		cidrKey("default", testNIC0IPv6Pods),
		cidrKey(testNetworkB, testPodCIDRB),
		cidrKey(testNetworkB, testNIC1IPv6Pods),
	})
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

// Two Networks sharing one interface each get that interface's /112: the /96
// belongs to the NIC, not to either Network.
func TestStatusController_IPv6SharedInterface(t *testing.T) {
	ctx := context.Background()
	f := ipv6Fixture(ctx, t, networkv1.DualStackType, true, "", testNIC1IPv6)

	f.networkResolver.UpsertGNP(gnp("net-c-gnp", testVPCB, testSubnetB, "pods-c"))
	nwC := readyL3Network("net-c", "net-c-gnp")
	nwC.Spec.StackType = networkv1.IPv6StackType
	f.networkResolver.UpsertNetwork(nwC)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	nnc, ok := getNNC(ctx, t, f)
	if !ok {
		t.Fatalf("Expected a NodeNetworkConfig")
	}
	want := sortedCopy([]string{
		cidrKey("default", testDefaultPod),
		cidrKey(testNetworkB, testPodCIDRB),
		cidrKey(testNetworkB, testNIC1IPv6Pods),
		cidrKey("net-c", testNIC1IPv6Pods),
	})
	if got := publishedCIDRs(nnc); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
}

// A StackType change on an already-published Network must be detected by
// statusNeedsSync and reflected on the next pass, with the IPv4 entry's
// transition time preserved.
func TestStatusController_IPv6FollowsStackTypeChange(t *testing.T) {
	ctx := context.Background()
	f := ipv6Fixture(ctx, t, networkv1.IPv4StackType, true, "", testNIC1IPv6)

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode failed: %v", err)
	}
	before, _ := getNNC(ctx, t, f)
	if got := publishedCIDRs(before); len(got) != 2 {
		t.Fatalf("Expected 2 IPv4 CIDRs before the change, got %v", got)
	}
	v4Transition := podCIDRReadySince(before)[cidrKey(testNetworkB, testPodCIDRB)]

	nw := readyL3Network(testNetworkB, testGNPB)
	nw.Spec.StackType = networkv1.DualStackType
	f.networkResolver.UpsertNetwork(nw)
	f.fakeClock.Step(1) // ensure a restamp would be visible

	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode after StackType change failed: %v", err)
	}
	after, _ := getNNC(ctx, t, f)
	want := sortedCopy([]string{
		cidrKey("default", testDefaultPod),
		cidrKey(testNetworkB, testPodCIDRB),
		cidrKey(testNetworkB, testNIC1IPv6Pods),
	})
	if got := publishedCIDRs(after); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Published CIDRs = %v, want %v", got, want)
	}
	if got := podCIDRReadySince(after)[cidrKey(testNetworkB, testPodCIDRB)]; !got.Equal(&v4Transition) {
		t.Errorf("IPv4 entry's LastTransitionTime was restamped: %v -> %v", v4Transition, got)
	}

	// Flipping back removes it again.
	nw.Spec.StackType = networkv1.IPv4StackType
	f.networkResolver.UpsertNetwork(nw)
	if err := f.statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode after reverting StackType failed: %v", err)
	}
	reverted, _ := getNNC(ctx, t, f)
	if got := publishedCIDRs(reverted); fmt.Sprint(got) != fmt.Sprint(publishedCIDRs(before)) {
		t.Errorf("Published CIDRs after revert = %v, want %v", got, publishedCIDRs(before))
	}
}

// The spec controller's view of GCE must stay IPv4-only: IPv6 /112s are not
// alias IP ranges and must never enter releasable-CIDR accounting.
func TestGCECIDRSet_ExcludesIPv6(t *testing.T) {
	ctx := context.Background()
	f := ipv6Fixture(ctx, t, networkv1.DualStackType, true, testNIC0IPv6, testNIC1IPv6)

	ifaces, err := f.statusCtrl.gceCache.ForceGet(ctx, testNodeName, testProviderID)
	if err != nil {
		t.Fatalf("ForceGet failed: %v", err)
	}
	v4 := f.specCtrl.gceCIDRSet(ifaces)
	all := f.statusCtrl.publishableCIDRSet(ifaces)
	for key := range v4 {
		if !all.Has(key) {
			t.Errorf("publishableCIDRSet is missing IPv4 key %q", key)
		}
	}
	if v4.Has(cidrKey(testNetworkB, testNIC1IPv6Pods)) {
		t.Errorf("gceCIDRSet must not contain IPv6 ranges: %v", v4.List())
	}
	if !all.Has(cidrKey(testNetworkB, testNIC1IPv6Pods)) {
		t.Errorf("publishableCIDRSet should contain the IPv6 range: %v", all.List())
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
