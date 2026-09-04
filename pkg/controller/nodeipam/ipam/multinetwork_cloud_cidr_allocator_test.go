package ipam

import (
	"fmt"
	"testing"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	clSetFake "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	ntfakeclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/fake"
	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/pkg/controller/testutil"
	"k8s.io/cloud-provider-gcp/providers/gce"
)

const (
	group                = "networking.gke.io"
	gkeNetworkParamsKind = "GKENetworkParams"
)

func network(name, gkeNetworkParamsName string, isReady bool) *networkv1.Network {
	return networkAll(name, gkeNetworkParamsName, networkv1.L3NetworkType, isReady)
}

func networkAll(name, gkeNetworkParamsName string, netType networkv1.NetworkType, isReady bool) *networkv1.Network {
	status := metav1.ConditionFalse
	if isReady {
		status = metav1.ConditionTrue
	}

	return &networkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.NetworkSpec{
			Type: netType,
			ParametersRef: &networkv1.NetworkParametersReference{
				Group: group,
				Kind:  gkeNetworkParamsKind,
				Name:  gkeNetworkParamsName,
			},
		},
		Status: networkv1.NetworkStatus{
			Conditions: []metav1.Condition{
				{
					Type:   string(networkv1.NetworkConditionStatusReady),
					Status: status,
				},
			},
		},
	}
}

func TestNetworkToNodes(t *testing.T) {

	testCases := []struct {
		desc            string
		network         *networkv1.Network
		expectNodes     map[string]struct{}
		fakeNodeHandler *testutil.FakeNodeHandler
	}{
		{
			desc:    "all nodes, network is nil",
			network: nil,
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node0": {}, "node1": {}},
		},
		{
			desc:    "all nodes with the network",
			network: network("test", "test", false),
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test2\",\"ipAddress\":\"10.240.2.27\"}]",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test3\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test\",\"ipAddress\":\"10.241.0.29\"}]",
							},
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node0": {}, "node1": {}},
		},
		{
			desc:    "only one node with the network",
			network: network("test", "test", true),
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test1\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test2\",\"ipAddress\":\"10.240.2.27\"}]",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.241.0.29\"}]",
							},
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node1": {}},
		},
		{
			desc:    "redo node with corrupted annotation",
			network: network("test", "test", false),
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "zzz",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test2\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test1\",\"ipAddress\":\"10.241.0.29\"}]",
							},
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node0": {}},
		},
		{
			desc:    "skip node with annotation==nil",
			network: network("test", "test", false),
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test1\",\"ipAddress\":\"10.241.0.29\"}]",
							},
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node1": {}},
		},
		{
			desc:    "skip node with no MN annotation",
			network: network("test", "test", false),
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "node0",
							Annotations: map[string]string{},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Annotations: map[string]string{
								networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.241.0.29\"},{\"network\":\"test1\",\"ipAddress\":\"10.241.0.29\"}]",
							},
						},
					},
				},
				Clientset: k8sfake.NewSimpleClientset(),
			},
			expectNodes: map[string]struct{}{"node1": {}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			fakeNodeInformer := getFakeNodeInformer(tc.fakeNodeHandler)

			ca := &cloudCIDRAllocator{
				nodeLister:  fakeNodeInformer.Lister(),
				nodesSynced: fakeNodeInformer.Informer().HasSynced,
				queue:       workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: "cloudCIDRAllocator"}),
			}

			// test
			err := ca.NetworkToNodes(tc.network)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if ca.queue.Len() != len(tc.expectNodes) {
				t.Fatalf("unexpected number of requests (nodesInProcessing): %v\nexpected (expectNodes): %v", ca.queue.Len(), tc.expectNodes)
			}

			n := ca.queue.Len()
			for i := 1; i < n; i++ {
				val, sh := ca.queue.Get()
				if sh {
					t.Fatalf("got preemtive queue shutdown")
				}
				_, ok := tc.expectNodes[val.(string)]
				if !ok {
					t.Fatalf("unexpected node %s in processing", val)
				}
			}
		})
	}
}

func TestGetNodeCapacity(t *testing.T) {
	testCases := []struct {
		desc      string
		input     networkv1.NodeNetwork
		want      int64
		expectErr bool
	}{
		{
			desc:      "no cidrs",
			input:     networkv1.NodeNetwork{},
			want:      -1,
			expectErr: true,
		},
		{
			desc: "incorrect cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2000.2.2.2/24"},
			},
			want:      -1,
			expectErr: true,
		},
		{
			desc: "24 v4 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2.2.2.2/24"},
			},
			want: 128,
		},
		{
			desc: "32 v4 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2.2.2.2/32"},
			},
			want: 1,
		},
		{
			desc: "31 v4 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2.2.2.2/31"},
			},
			want: 1,
		},
		{
			desc: "30 v4 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2.2.2.2/30"},
			},
			want: 2,
		},
		{
			desc: "2 v4 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"2.2.2.2/2"},
			},
			want: 536870912,
		},
		{
			desc: "120 v6 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"200:12::/120"},
			},
			want: 128,
		},
		{
			desc: "128 v6 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"200:12::/128"},
			},
			want: 1,
		},
		{
			desc: "127 v6 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"200:12::/127"},
			},
			want: 1,
		},
		{
			desc: "126 v6 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"200:12::/126"},
			},
			want: 2,
		},
		{
			desc: "2 v6 cidrs",
			input: networkv1.NodeNetwork{
				Cidrs: []string{"200:12::/2"},
			},
			want: 4611686018427387903,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			got, err := getNodeCapacity(tc.input)
			if err == nil && tc.expectErr {
				t.Fatalf("getNodeCapacity(%+v) error expected but got nil", tc.input)
			} else if err != nil && !tc.expectErr {
				t.Fatalf("getNodeCapacity(%+v) got unexpected error", tc.input)
			}

			if got != tc.want {
				t.Fatalf("getNodeCapacity(%+v) returns %v but want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetUpNetworks(t *testing.T) {
	tests := []struct {
		name        string
		node        *v1.Node
		expected    map[string]struct{}
		expectError bool
	}{
		{
			name:        "empty node",
			node:        &v1.Node{},
			expected:    map[string]struct{}{},
			expectError: false,
		},
		{
			name: "node with no annotations",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{

					Annotations: map[string]string{},
				},
			},
			expected:    map[string]struct{}{},
			expectError: false,
		},
		{
			name: "node with valid annotation",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						networkv1.NodeNetworkAnnotationKey: `[{"name": "net1"}]`,
					},
				},
			},
			expected: map[string]struct{}{
				"net1": {},
			},
			expectError: false,
		},
		{
			name: "node with invalid annotation",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{

					Annotations: map[string]string{
						networkv1.NodeNetworkAnnotationKey: `invalid`,
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := getUpNetworks(test.node)
			if test.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", test.expected) {
				t.Errorf("expected %v, but got %v", test.expected, result)
			}
		})
	}
}

func TestExtractDefaultNwCIDRs(t *testing.T) {
	ca := &cloudCIDRAllocator{
		cloud: &gce.Cloud{},
	}
	interfaces := []*compute.NetworkInterface{
		{
			Subnetwork: "invalid-subnetwork-url",
		},
		{
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/default",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					SubnetworkRangeName: "RangeA",
					IpCidrRange:         "10.0.0.0/24",
				},
			},
		},
	}
	// This should not panic and should successfully ignore the invalid subnetwork URL and extract default CIDR
	res := ca.extractDefaultNwCIDRs(interfaces, "default", "RangeA")
	if len(res) != 1 || res[0] != "10.0.0.0/24" {
		t.Errorf("Expected [10.0.0.0/24], got %v", res)
	}
}

func TestDefaultNetworkCIDRs_IPv4Only(t *testing.T) {
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 0).Networking()

	// Default Network
	defaultNetwork := networkAll("default", "default", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(defaultNetwork)
	defaultGNP := gkeNetworkParams("default", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/default", []string{"test-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(defaultGNP)

	// Additional non-default network with same VPC but different subnet
	secondary1 := networkAll("secondary1", "secondary1-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary1)
	sec1GNP := gkeNetworkParams("secondary1-params", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/secondary-subnet", []string{"sec1-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec1GNP)

	// Additional non-default network with different VPC
	secondary2 := networkAll("secondary2", "secondary2-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary2)
	sec2GNP := gkeNetworkParams("secondary2-params", "projects/testProject/global/networks/other-vpc", "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet", []string{"sec2-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec2GNP)

	kubeClient := fake.NewSimpleClientset()
	k8sInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := k8sInformerFactory.Core().V1().Nodes()

	ca, _ := NewCloudCIDRAllocator(kubeClient, fakeGCE, nwInfFactory.V1().Networks(), nwInfFactory.V1().GKENetworkParamSets(), ntfakeclient.NewSimpleClientset(), false, true, nodeInformer, CIDRAllocatorParams{})
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ipv4-node",
			Annotations: map[string]string{
				// Mark default and additional networks as ready
				networkv1.NodeNetworkAnnotationKey: `[{"name":"default"}, {"name":"secondary1"}, {"name":"secondary2"}]`,
			},
		},
	}

	interfaces := []*compute.NetworkInterface{
		{
			Name:       "nic0",
			Network:    "projects/testProject/global/networks/default",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/default",
			NetworkIP:  "10.0.0.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.1.0/24",
					SubnetworkRangeName: "test-pod-range",
				},
			},
		},
		{
			Name:       "nic1",
			Network:    "projects/testProject/global/networks/default",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/secondary-subnet",
			NetworkIP:  "10.0.2.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.5.0/24",
					SubnetworkRangeName: "sec1-pod-range",
				},
			},
		},
		{
			Name:       "nic2",
			Network:    "projects/testProject/global/networks/other-vpc",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet",
			NetworkIP:  "10.0.3.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.4.0/24",
					SubnetworkRangeName: "sec2-pod-range",
				},
			},
		},
	}

	cidrs, err := cloudAllocator.performMultiNetworkCIDRAllocation(node, interfaces, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cidrs) != 1 || cidrs[0] != "10.0.1.0/24" {
		t.Errorf("Expected exact 1 CIDR block string for IPv4 podCIDR [10.0.1.0/24], got: %v", cidrs)
	}
}

func TestDefaultNetworkCIDRs_DualStack_NoLabels(t *testing.T) {
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 0).Networking()

	defaultNetwork := networkAll("default", "default", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(defaultNetwork)

	// Initialize mock for default network - with IPv4 parameters
	defaultGNP := gkeNetworkParams("default", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/default", []string{"test-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(defaultGNP)

	// Additional networks
	secondary1 := networkAll("secondary1", "secondary1-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary1)
	sec1GNP := gkeNetworkParams("secondary1-params", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/secondary-subnet", []string{"sec1-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec1GNP)

	secondary2 := networkAll("secondary2", "secondary2-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary2)
	sec2GNP := gkeNetworkParams("secondary2-params", "projects/testProject/global/networks/other-vpc", "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet", []string{"sec2-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec2GNP)

	kubeClient := fake.NewSimpleClientset()
	k8sInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := k8sInformerFactory.Core().V1().Nodes()

	ca, _ := NewCloudCIDRAllocator(kubeClient, fakeGCE, nwInfFactory.V1().Networks(), nwInfFactory.V1().GKENetworkParamSets(), ntfakeclient.NewSimpleClientset(), false, true, nodeInformer, CIDRAllocatorParams{})
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dual-stack-node",
			Annotations: map[string]string{
				networkv1.NodeNetworkAnnotationKey: `[{"name":"default"}, {"name":"secondary1"}, {"name":"secondary2"}]`,
			},
		},
	}

	// Simulate node using v4/v6 stack on default network
	interfaces := []*compute.NetworkInterface{
		{
			Name:        "nic0",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/default",
			NetworkIP:   "10.0.0.1",
			Ipv6Address: "2600:1900:4000:fd1::110",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.1.0/24",
					SubnetworkRangeName: "test-pod-range",
				},
			},
		},
		{
			Name:        "nic1",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/secondary-subnet",
			NetworkIP:   "10.0.2.1",
			Ipv6Address: "2001:db9:1::110",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.5.0/24",
					SubnetworkRangeName: "sec1-pod-range",
				},
			},
		},
		{
			Name:       "nic2",
			Network:    "projects/testProject/global/networks/other-vpc",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet",
			NetworkIP:  "10.0.3.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.4.0/24",
					SubnetworkRangeName: "sec2-pod-range",
				},
			},
		},
	}

	// Test case without node labels (hasNodeLabels = false)
	cidrs, err := cloudAllocator.performMultiNetworkCIDRAllocation(node, interfaces, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without labels, the function should extract both IPv4 and IPv6 CIDRs
	if len(cidrs) != 2 || cidrs[0] != "10.0.1.0/24" || cidrs[1] != "2600:1900:4000:fd1::/112" {
		t.Errorf("Expected exactly IPv4 and IPv6 CIDR blocks from dual-stack, got: %v", cidrs)
	}
}

func TestDefaultNetworkCIDRs_DualStack_WithLabels(t *testing.T) {
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 0).Networking()

	defaultNetwork := networkAll("default", "default", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(defaultNetwork)

	defaultGNP := gkeNetworkParams("default", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/default", []string{"test-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(defaultGNP)

	secondary1 := networkAll("secondary1", "secondary1-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary1)
	sec1GNP := gkeNetworkParams("secondary1-params", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/secondary-subnet", []string{"sec1-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec1GNP)

	secondary2 := networkAll("secondary2", "secondary2-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary2)
	sec2GNP := gkeNetworkParams("secondary2-params", "projects/testProject/global/networks/other-vpc", "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet", []string{"sec2-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec2GNP)

	kubeClient := fake.NewSimpleClientset()
	k8sInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := k8sInformerFactory.Core().V1().Nodes()

	ca, _ := NewCloudCIDRAllocator(kubeClient, fakeGCE, nwInfFactory.V1().Networks(), nwInfFactory.V1().GKENetworkParamSets(), ntfakeclient.NewSimpleClientset(), false, true, nodeInformer, CIDRAllocatorParams{})
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dual-stack-node-labeled",
			Annotations: map[string]string{
				networkv1.NodeNetworkAnnotationKey: `[{"name":"default"}, {"name":"secondary1"}, {"name":"secondary2"}]`,
			},
		},
	}

	interfaces := []*compute.NetworkInterface{
		{
			Name:        "nic0",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/default",
			NetworkIP:   "10.0.0.1",
			Ipv6Address: "2600:1900:4000:fd1::110",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.1.0/24",
					SubnetworkRangeName: "test-pod-range",
				},
			},
		},
		{
			Name:        "nic1",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/secondary-subnet",
			NetworkIP:   "10.0.2.1",
			Ipv6Address: "2001:db9:1::110",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.5.0/24",
					SubnetworkRangeName: "sec1-pod-range",
				},
			},
		},
		{
			Name:       "nic2",
			Network:    "projects/testProject/global/networks/other-vpc",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet",
			NetworkIP:  "10.0.3.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.4.0/24",
					SubnetworkRangeName: "sec2-pod-range",
				},
			},
		},
	}

	// Test case WITH known node labels (hasNodeLabels = true)
	cidrs, err := cloudAllocator.performMultiNetworkCIDRAllocation(node, interfaces, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With hasNodeLabels==true, the function drops default network allocation, leaving an empty slice
	if len(cidrs) != 0 {
		t.Errorf("Expected empty CIDR blocks because node has labels, got: %v", cidrs)
	}
}

func TestDefaultNetworkCIDRs_IPv6Only(t *testing.T) {
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 0).Networking()

	// Default Network
	defaultNetwork := networkAll("default", "default", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(defaultNetwork)
	defaultGNP := gkeNetworkParams("default", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/default", []string{})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(defaultGNP)

	// Additional non-default network with same VPC but different subnet
	secondary1 := networkAll("secondary1", "secondary1-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary1)
	sec1GNP := gkeNetworkParams("secondary1-params", "projects/testProject/global/networks/default", "projects/testProject/regions/us-central1/subnetworks/secondary-subnet", []string{"sec1-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec1GNP)

	// Additional non-default network with different VPC
	secondary2 := networkAll("secondary2", "secondary2-params", networkv1.L3NetworkType, true)
	nwInfFactory.V1().Networks().Informer().GetIndexer().Add(secondary2)
	sec2GNP := gkeNetworkParams("secondary2-params", "projects/testProject/global/networks/other-vpc", "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet", []string{"sec2-pod-range"})
	nwInfFactory.V1().GKENetworkParamSets().Informer().GetIndexer().Add(sec2GNP)

	kubeClient := fake.NewSimpleClientset()
	k8sInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := k8sInformerFactory.Core().V1().Nodes()

	ca, _ := NewCloudCIDRAllocator(kubeClient, fakeGCE, nwInfFactory.V1().Networks(), nwInfFactory.V1().GKENetworkParamSets(), ntfakeclient.NewSimpleClientset(), false, true, nodeInformer, CIDRAllocatorParams{})
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ipv6-node",
			Annotations: map[string]string{
				// Mark default and additional networks as ready
				networkv1.NodeNetworkAnnotationKey: `[{"name":"default"}, {"name":"secondary1"}, {"name":"secondary2"}]`,
			},
		},
	}

	interfaces := []*compute.NetworkInterface{
		{
			Name:        "nic0",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/default",
			Ipv6Address: "2600:1900:4000:fd1::110",
		},
		{
			Name:        "nic1",
			Network:     "projects/testProject/global/networks/default",
			Subnetwork:  "projects/testProject/regions/us-central1/subnetworks/secondary-subnet",
			NetworkIP:   "10.0.1.1",
			Ipv6Address: "2001:db9:1::110",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.5.0/24",
					SubnetworkRangeName: "sec1-pod-range",
				},
			},
		},
		{
			Name:       "nic2",
			Network:    "projects/testProject/global/networks/other-vpc",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/other-vpc-subnet",
			NetworkIP:  "10.0.3.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.4.0/24",
					SubnetworkRangeName: "sec2-pod-range",
				},
			},
		},
	}

	cidrs, err := cloudAllocator.performMultiNetworkCIDRAllocation(node, interfaces, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cidrs) != 1 || cidrs[0] != "2600:1900:4000:fd1::/112" {
		t.Errorf("Expected exactly 1 CIDR block string for IPv6 podCIDR [2600:1900:4000:fd1::/112], got: %v", cidrs)
	}
}

func TestDefaultNetworkCIDRs_DefaultNetworkNotUp(t *testing.T) {
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	clientSet := clSetFake.NewSimpleClientset()
	// Empty informer - no default network added
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 0).Networking()

	kubeClient := fake.NewSimpleClientset()
	k8sInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := k8sInformerFactory.Core().V1().Nodes()

	ca, _ := NewCloudCIDRAllocator(kubeClient, fakeGCE, nwInfFactory.V1().Networks(), nwInfFactory.V1().GKENetworkParamSets(), ntfakeclient.NewSimpleClientset(), false, true, nodeInformer, CIDRAllocatorParams{})
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "missing-network-node",
			Annotations: map[string]string{
				networkv1.NodeNetworkAnnotationKey: `[{"name":"default"}]`,
			},
		},
	}

	interfaces := []*compute.NetworkInterface{
		{
			Name:       "nic0",
			Network:    "projects/testProject/global/networks/default",
			Subnetwork: "projects/testProject/regions/us-central1/subnetworks/default",
			NetworkIP:  "10.0.0.1",
			AliasIpRanges: []*compute.AliasIpRange{
				{
					IpCidrRange:         "10.0.1.0/24",
					SubnetworkRangeName: "test-pod-range",
				},
			},
		},
	}

	cidrs, err := cloudAllocator.performMultiNetworkCIDRAllocation(node, interfaces, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that missing default Network CR returns an empty slice safely instead of throwing a panic
	if len(cidrs) != 0 {
		t.Errorf("Expected empty CIDR blocks because default network was not in Informer (not up/ready), got: %v", cidrs)
	}
}
