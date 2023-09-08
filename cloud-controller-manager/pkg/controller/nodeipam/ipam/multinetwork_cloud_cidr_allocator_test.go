package ipam

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/testutil"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
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
