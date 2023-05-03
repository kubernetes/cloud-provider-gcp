package ipam

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/cloud-provider-gcp/pkg/controller/testutil"
)

const (
	group                = "networking.gke.io"
	gkeNetworkParamsKind = "GKENetworkParams"
)

func network(name, gkeNetworkParamsName string) *networkv1.Network {
	return &networkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.NetworkSpec{
			Type: "L3",
			ParametersRef: &networkv1.NetworkParametersReference{
				Group: group,
				Kind:  gkeNetworkParamsKind,
				Name:  gkeNetworkParamsName,
			},
		},
	}
}

func TestNetworkToNodes(t *testing.T) {

	testCases := []struct {
		desc            string
		network         *networkv1.Network
		expectNodes     []string
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
			expectNodes: []string{"node0", "node1"},
		},
		{
			desc:    "all nodes with the network",
			network: network("test", "test"),
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
			expectNodes: []string{"node0", "node1"},
		},
		{
			desc:    "only one node with the network",
			network: network("test", "test"),
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
			expectNodes: []string{"node1"},
		},
		{
			desc:    "redo node with corrupted annotation",
			network: network("test", "test"),
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
			expectNodes: []string{"node0"},
		},
		{
			desc:    "skip node with annotation==nil",
			network: network("test", "test"),
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
			expectNodes: []string{"node1"},
		},
		{
			desc:    "skip node with no MN annotation",
			network: network("test", "test"),
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
			expectNodes: []string{"node1"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			fakeNodeInformer := getFakeNodeInformer(tc.fakeNodeHandler)

			ca := &cloudCIDRAllocator{
				nodeLister:        fakeNodeInformer.Lister(),
				nodesSynced:       fakeNodeInformer.Informer().HasSynced,
				nodeUpdateChannel: make(chan string, cidrUpdateQueueSize),
				nodesInProcessing: map[string]*nodeProcessingInfo{},
			}

			// test
			err := ca.NetworkToNodes(tc.network)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(ca.nodesInProcessing) != len(tc.expectNodes) {
				t.Fatalf("unexpected number of requests (nodesInProcessing): %v\nexpected (expectNodes): %v", ca.nodesInProcessing, tc.expectNodes)
			}

			for _, node := range tc.expectNodes {
				_, ok := ca.nodesInProcessing[node]
				if !ok {
					t.Fatalf("node %s not in processing", node)
				}
			}
		})
	}
}
