//go:build !providerless
// +build !providerless

/*
Copyright 2018 The Kubernetes Authors.

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

package ipam

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/cloud-provider-gcp/pkg/controller/testutil"
	"k8s.io/cloud-provider-gcp/providers/gce"
	netutils "k8s.io/utils/net"
)

func hasNodeInProcessing(ca *cloudCIDRAllocator, name string) bool {
	ca.lock.Lock()
	defer ca.lock.Unlock()

	_, found := ca.nodesInProcessing[name]
	return found
}

func TestBoundedRetries(t *testing.T) {
	clientSet := fake.NewSimpleClientset()
	updateChan := make(chan string, 1) // need to buffer as we are using only on go routine
	stopChan := make(chan struct{})
	sharedInfomer := informers.NewSharedInformerFactory(clientSet, 1*time.Hour)
	ca := &cloudCIDRAllocator{
		client:            clientSet,
		nodeUpdateChannel: updateChan,
		nodeLister:        sharedInfomer.Core().V1().Nodes().Lister(),
		nodesSynced:       sharedInfomer.Core().V1().Nodes().Informer().HasSynced,
		nodesInProcessing: map[string]*nodeProcessingInfo{},
	}
	go ca.worker(stopChan)
	nodeName := "testNode"
	ca.AllocateOrOccupyCIDR(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	})
	for hasNodeInProcessing(ca, nodeName) {
		// wait for node to finish processing (should terminate and not time out)
	}
}

func withinExpectedRange(got time.Duration, expected time.Duration) bool {
	return got >= expected/2 && got <= 3*expected/2
}

func TestNodeUpdateRetryTimeout(t *testing.T) {
	for _, tc := range []struct {
		count int
		want  time.Duration
	}{
		{count: 0, want: 250 * time.Millisecond},
		{count: 1, want: 500 * time.Millisecond},
		{count: 2, want: 1000 * time.Millisecond},
		{count: 3, want: 2000 * time.Millisecond},
		{count: 50, want: 5000 * time.Millisecond},
	} {
		t.Run(fmt.Sprintf("count %d", tc.count), func(t *testing.T) {
			if got := nodeUpdateRetryTimeout(tc.count); !withinExpectedRange(got, tc.want) {
				t.Errorf("nodeUpdateRetryTimeout(tc.count) = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestUpdateCIDRAllocation(t *testing.T) {
	tests := []struct {
		name            string
		fakeNodeHandler *testutil.FakeNodeHandler
		nodeChanges     func(*v1.Node)
		gceInstance     []*compute.Instance
		expectErr       bool
		expectErrMsg    string
		expectedUpdate  bool
	}{
		{
			name: "node not found in k8s",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test1",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {},
		},
		{
			name: "provider not set",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
				},
			},
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "doesn't have providerID",
		},
		{
			name: "node not found in gce by provider",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
				},
			},
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "failed to get instance from provider",
		},
		{
			name: "gce node has no networks",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
				},
			},
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "Node test has no ranges from which CIDRs can",
		},
		{
			name: "empty single stack node",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						{
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Spec.PodCIDR = "192.168.1.0/24"
				node.Spec.PodCIDRs = []string{"192.168.1.0/24"}
				node.Status.Conditions = []v1.NodeCondition{
					{
						Type:    "NetworkUnavailable",
						Status:  "False",
						Reason:  "RouteCreated",
						Message: "NodeController create implicit route",
					},
				}
			},
			expectedUpdate: true,
		},
		{
			name: "empty dualstack node",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						{
							Ipv6Address: "2001:db9::110",
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Spec.PodCIDR = "192.168.1.0/24"
				node.Spec.PodCIDRs = []string{"192.168.1.0/24", "2001:db9::/112"}
				node.Status.Conditions = []v1.NodeCondition{
					{
						Type:    "NetworkUnavailable",
						Status:  "False",
						Reason:  "RouteCreated",
						Message: "NodeController create implicit route",
					},
				}
			},
			expectedUpdate: true,
		},
		{
			name: "incorrect cidr",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						{
							Ipv6Address: "2001:db9::/96",
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "failed to parse strings",
		},
		{
			name: "node already configured",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Conditions: []v1.NodeCondition{
								{
									Type:    "NetworkUnavailable",
									Status:  "False",
									Reason:  "RouteCreated",
									Message: "NodeController create implicit route",
								},
							},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						{
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			nodeChanges: func(node *v1.Node) {},
		},
		{
			name: "node configured but NetworkUnavailable condition",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Conditions: []v1.NodeCondition{
								{
									Type:    "NetworkUnavailable",
									Status:  "True",
									Reason:  "RouteCreated",
									Message: "NodeController create implicit route",
								},
							},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						{
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Status.Conditions[0].Status = "False"
			},
			expectedUpdate: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, stop := context.WithCancel(context.Background())
			defer stop()

			fakeNodeInformer := getFakeNodeInformer(tc.fakeNodeHandler)
			testClusterValues := gce.DefaultTestClusterValues()
			fakeGCE := gce.NewFakeGCECloud(testClusterValues)
			for _, inst := range tc.gceInstance {
				err := fakeGCE.Compute().Instances().Insert(ctx, meta.ZonalKey(inst.Name, testClusterValues.ZoneName), inst)
				if err != nil {
					t.Fatalf("error setting up the test for fakeGCE: %v", err)
				}
			}

			wantNode := tc.fakeNodeHandler.Existing[0].DeepCopy()
			tc.nodeChanges(wantNode)

			ca := &cloudCIDRAllocator{
				client:      tc.fakeNodeHandler,
				cloud:       fakeGCE,
				recorder:    testutil.NewFakeRecorder(),
				nodeLister:  fakeNodeInformer.Lister(),
				nodesSynced: fakeNodeInformer.Informer().HasSynced,
			}
			if err := ca.updateCIDRAllocation("test"); err != nil {
				if tc.expectErr {
					if tc.expectErrMsg != "" && !strings.Contains(err.Error(), tc.expectErrMsg) {
						t.Fatalf("received unexpected error message:\nwant: %s\ngot: %v", tc.expectErrMsg, err)
					}
					return
				}
				t.Fatalf("unexpected error %v", err)
			}

			updNodes := tc.fakeNodeHandler.GetUpdatedNodesCopy()
			var gotNode *v1.Node
			if len(updNodes) == 0 {
				if tc.expectedUpdate {
					t.Fatalf("Node update expected but none done")
				}
				gotNode = tc.fakeNodeHandler.Existing[0]
			} else {
				if !tc.expectedUpdate {
					t.Fatalf("Node update not expected but received: %v", updNodes[0])
				}
				gotNode = updNodes[0]
			}
			sanitizeDates(gotNode)

			diff := cmp.Diff(wantNode, gotNode)
			if diff != "" {
				t.Fatalf("updateCIDRAllocation() node not updated (-want +got) = %s", diff)
			}
		})
	}
}

func sanitizeDates(node *v1.Node) {
	for i := range node.Status.Conditions {
		node.Status.Conditions[i].LastHeartbeatTime = metav1.Time{}
		node.Status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
}

func gkeNetworkParams(name, vpc, subnet string, secRangeNames []string) *networkv1.GKENetworkParamSet {
	gnp := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			VPC:       vpc,
			VPCSubnet: subnet,
		},
	}
	if len(secRangeNames) > 0 {
		gnp.Spec.PodIPv4Ranges = &networkv1.SecondaryRanges{
			RangeNames: secRangeNames,
		}
	}
	return gnp
}

func interfaces(network, subnetwork, networkIP string, aliasIPRanges []*compute.AliasIpRange) *compute.NetworkInterface {
	return &compute.NetworkInterface{
		AliasIpRanges: aliasIPRanges,
		Network:       network,
		Subnetwork:    subnetwork,
		NetworkIP:     networkIP,
	}
}

func TestNeedPodCIDRsUpdate(t *testing.T) {
	for _, tc := range []struct {
		desc         string
		cidrs        []string
		nodePodCIDR  string
		nodePodCIDRs []string
		want         bool
		wantErr      bool
	}{
		{
			desc:         "want error - invalid cidr",
			cidrs:        []string{"10.10.10.0/24"},
			nodePodCIDR:  "10.10..0/24",
			nodePodCIDRs: []string{"10.10..0/24"},
			want:         true,
		},
		{
			desc:         "want error - cidr len 2 but not dual stack",
			cidrs:        []string{"10.10.10.0/24", "10.10.11.0/24"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24", "2001:db8::/64"},
			wantErr:      true,
		},
		{
			desc:         "want false - matching v4 only cidr",
			cidrs:        []string{"10.10.10.0/24"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24"},
			want:         false,
		},
		{
			desc:  "want false - nil node.Spec.PodCIDR",
			cidrs: []string{"10.10.10.0/24"},
			want:  true,
		},
		{
			desc:         "want true - non matching v4 only cidr",
			cidrs:        []string{"10.10.10.0/24"},
			nodePodCIDR:  "10.10.11.0/24",
			nodePodCIDRs: []string{"10.10.11.0/24"},
			want:         true,
		},
		{
			desc:         "want false - matching v4 and v6 cidrs",
			cidrs:        []string{"10.10.10.0/24", "2001:db8::/64"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24", "2001:db8::/64"},
			want:         false,
		},
		{
			desc:         "want false - matching v4 and v6 cidrs, different strings but same CIDRs",
			cidrs:        []string{"10.10.10.0/24", "2001:db8::/64"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24", "2001:db8:0::/64"},
			want:         false,
		},
		{
			desc:         "want true - matching v4 and non matching v6 cidrs",
			cidrs:        []string{"10.10.10.0/24", "2001:db8::/64"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24", "2001:dba::/64"},
			want:         true,
		},
		{
			desc:  "want true - nil node.Spec.PodCIDRs",
			cidrs: []string{"10.10.10.0/24", "2001:db8::/64"},
			want:  true,
		},
		{
			desc:         "want true - matching v6 and non matching v4 cidrs",
			cidrs:        []string{"10.10.10.0/24", "2001:db8::/64"},
			nodePodCIDR:  "10.10.1.0/24",
			nodePodCIDRs: []string{"10.10.1.0/24", "2001:db8::/64"},
			want:         true,
		},
		{
			desc:         "want true - missing v6",
			cidrs:        []string{"10.10.10.0/24", "2001:db8::/64"},
			nodePodCIDR:  "10.10.10.0/24",
			nodePodCIDRs: []string{"10.10.10.0/24"},
			want:         true,
		},
	} {
		var node v1.Node
		node.Spec.PodCIDR = tc.nodePodCIDR
		node.Spec.PodCIDRs = tc.nodePodCIDRs
		netCIDRs, err := netutils.ParseCIDRs(tc.cidrs)
		if err != nil {
			t.Errorf("failed to parse %v as CIDRs: %v", tc.cidrs, err)
		}

		t.Run(tc.desc, func(t *testing.T) {
			got, err := needPodCIDRsUpdate(&node, netCIDRs)
			if tc.wantErr == (err == nil) {
				t.Errorf("err: %v, wantErr: %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}
}
