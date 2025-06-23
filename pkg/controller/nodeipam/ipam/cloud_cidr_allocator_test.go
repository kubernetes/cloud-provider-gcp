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
	"net"
	"strings"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	ntv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	clSetFake "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	ntfakeclient "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/fake"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/pkg/controller/testutil"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/cloud-provider-gcp/providers/gce"
	metricsUtil "k8s.io/component-base/metrics/testutil"
)

const (
	// Default Network
	defaultGKENetworkParamsName = "DefaultGKENetworkParams"
	defaultVPCName              = "projects/testProject/global/networks/default"
	defaultVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/default"
	defaultSecondaryRangeA      = "RangeA"
	defaultSecondaryRangeB      = "RangeB"
	// Red Network
	redNetworkName           = "Red-Network"
	redGKENetworkParamsName  = "RedGKENetworkParams"
	redVPCName               = "projects/testProject/global/networks/red"
	redVPCSubnetName         = "projects/testProject/regions/us-central1/subnetworks/red"
	redSecondaryRangeA       = "RedRangeA"
	redSecondaryRangeB       = "RedRangeB"
	redNetworkAttachmentName = "projects/testProject/regions/us-central1/networkAttachments/red"
	// Blue Network
	blueNetworkName           = "Blue-Network"
	blueGKENetworkParamsName  = "BlueGKENetworkParams"
	blueVPCName               = "projects/testProject/global/networks/blue"
	blueVPCSubnetName         = "projects/testProject/regions/us-central1/subnetworks/blue"
	blueSecondaryRangeA       = "BlueRangeA"
	blueNetworkAttachmentName = "projects/testProject/regions/us-central1/networkAttachments/blue"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

func hasNodeInProcessing(ca *cloudCIDRAllocator, name string) bool {
	if ca.queue.Len() > 0 {
		val, _ := ca.queue.Get()
		if val.(string) == name {
			return true
		}
	}
	return false
}

func TestBoundedRetries(t *testing.T) {
	clientSet := fake.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(clientSet, 1*time.Hour)
	ca := &cloudCIDRAllocator{
		client:      clientSet,
		nodeLister:  sharedInfomer.Core().V1().Nodes().Lister(),
		nodesSynced: sharedInfomer.Core().V1().Nodes().Informer().HasSynced,
		queue:       workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: "cloudCIDRAllocator"}),
	}
	go wait.UntilWithContext(context.TODO(), ca.runWorker, time.Second)
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

func TestNodeTopologyQueuePeriodicSync(t *testing.T) {
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.SubnetworkURL = exampleSubnetURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, noResyncPeriodFunc()).Networking()
	nwInformer := nwInfFactory.V1().Networks()
	gnpInformer := nwInfFactory.V1().GKENetworkParamSets()

	defaultnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNodeTopologyLifecycle",
		},
	}
	mscnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode",
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet1",
			},
		},
	}
	fakeClient := fake.NewSimpleClientset(defaultnode, mscnode)
	fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, time.Second)
	fakeNodeInformer := fakeInformerFactory.Core().V1().Nodes()

	ensuredNodeTopologyCR := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	nodeTopologyClient := ntfakeclient.NewSimpleClientset(ensuredNodeTopologyCR)
	allocatorParams := CIDRAllocatorParams{}

	KuberntesClientSet := fake.NewSimpleClientset()
	ca, _ := NewCloudCIDRAllocator(KuberntesClientSet, fakeGCE, nwInformer, gnpInformer, nodeTopologyClient, true, fakeNodeInformer, allocatorParams)
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)
	cloudAllocator.nodeTopologyQueue.Run()

	stopCh := make(chan struct{})
	fakeInformerFactory.Start(wait.NeverStop)
	go wait.Until(
		func() {
			cloudAllocator.nodeTopologyQueue.Enqueue(nodeTopologyReconcileFakeNode)
		},
		time.Millisecond*500, stopCh)

	i := 0
	expectedSubnets := []string{"subnet-def", "subnet1"}
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("Periodic sync node topology queue in not working.")
	}

	stopCh <- struct{}{}
	cloudAllocator.nodeTopologyQueue.Shutdown()
	time.Sleep(time.Second * 1)
	mscnode2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode2",
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet2",
			},
		},
	}
	fakeClient.Tracker().Add(mscnode2)
	time.Sleep(time.Second * 1)
	if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); !ok {
		t.Fatalf("After queue shutdown there should be no sync.")
	}
}

func TestNodeTopologyCR_AddOrUpdateNode(t *testing.T) {
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.SubnetworkURL = exampleSubnetURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, noResyncPeriodFunc()).Networking()
	nwInformer := nwInfFactory.V1().Networks()
	gnpInformer := nwInfFactory.V1().GKENetworkParamSets()

	defaultnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNodeTopologyLifecycle",
		},
	}
	mscnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode",
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet1",
			},
		},
	}
	fakeClient := fake.NewSimpleClientset(defaultnode)
	fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, time.Second)
	fakeNodeInformer := fakeInformerFactory.Core().V1().Nodes()

	ensuredNodeTopologyCR := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	nodeTopologyClient := ntfakeclient.NewSimpleClientset(ensuredNodeTopologyCR)
	allocatorParams := CIDRAllocatorParams{}

	KuberntesClientSet := fake.NewSimpleClientset()
	ca, _ := NewCloudCIDRAllocator(KuberntesClientSet, fakeGCE, nwInformer, gnpInformer, nodeTopologyClient, true, fakeNodeInformer, allocatorParams)
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	fakeInformerFactory.Start(wait.NeverStop)
	go cloudAllocator.Run(wait.NeverStop)

	// TODO: Fix node_topology_syncer addOrUpdate should add default subnet regardless of nodes ordering on the informer
	time.Sleep(time.Millisecond * 500)
	fakeClient.Tracker().Add(mscnode)
	expectedSubnets := []string{"subnet-def", "subnet1"}
	i := 0
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("AddOrUpdate node topology CRD not working as expected")
	}

	mscnode2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode2",
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet2",
			},
		},
	}
	fakeClient.Tracker().Add(mscnode2)
	expectedSubnets = []string{"subnet-def", "subnet1", "subnet2"}
	i = 0
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("AddOrUpdate node topology CRD not working as expected")
	}
	// Node subnet label should be immutable, update it just to test update node path
	mscnode2.ObjectMeta.Labels[testNodePoolSubnetLabelPrefix] = "subnet3"
	// TODO: automatically get gvr instead of hardcode
	gvr := runtimeSchema.GroupVersionResource{
		Version:  "v1",
		Resource: "nodes",
	}
	fakeClient.Tracker().Update(gvr, mscnode2, mscnode2.GetNamespace(), metav1.UpdateOptions{})
	expectedSubnets = []string{"subnet-def", "subnet1", "subnet2", "subnet3"}
	i = 0
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("UpdateNode with different subnet lable should not dedup when enqueueing")
	}
	// Reset nodetopology just for test update node de-dup when the label didn't change
	nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(context.TODO(), ensuredNodeTopologyCR, metav1.UpdateOptions{})
	// Update the node w/o changing node pool subnet label should de-dup, not enqueue
	mscnode2.ObjectMeta.Labels[testNodePoolSubnetLabelPrefix] = "subnet3"
	fakeClient.Tracker().Update(gvr, mscnode2, mscnode2.GetNamespace(), metav1.UpdateOptions{})
	time.Sleep(time.Millisecond * 500)
	expectedSubnets = []string{}
	if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); !ok {
		t.Fatalf("UpdateNode with the same subnet lable should dedup when enqueueing")
	}
}

func TestNodeTopologyCR_DeleteNode(t *testing.T) {
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.SubnetworkURL = exampleSubnetURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	clientSet := clSetFake.NewSimpleClientset()
	nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, noResyncPeriodFunc()).Networking()
	nwInformer := nwInfFactory.V1().Networks()
	gnpInformer := nwInfFactory.V1().GKENetworkParamSets()

	defaultnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nodeTopologyDefautNode",
		},
	}
	fakeClient := fake.NewSimpleClientset(defaultnode)
	fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, time.Second)
	fakeNodeInformer := fakeInformerFactory.Core().V1().Nodes()

	ensuredNodeTopologyCR := &ntv1.NodeTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	nodeTopologyClient := ntfakeclient.NewSimpleClientset(ensuredNodeTopologyCR)
	allocatorParams := CIDRAllocatorParams{}

	KuberntesClientSet := fake.NewSimpleClientset()
	ca, _ := NewCloudCIDRAllocator(KuberntesClientSet, fakeGCE, nwInformer, gnpInformer, nodeTopologyClient, true, fakeNodeInformer, allocatorParams)
	cloudAllocator, _ := ca.(*cloudCIDRAllocator)

	fakeInformerFactory.Start(wait.NeverStop)
	go cloudAllocator.Run(wait.NeverStop)

	mscnode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testNode",
			Labels: map[string]string{
				testNodePoolSubnetLabelPrefix: "subnet1",
			},
		},
	}
	fakeClient.Tracker().Add(mscnode)

	expectedSubnets := []string{"subnet-def", "subnet1"}
	i := 0
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("Add node topology CR not working as expected")
	}
	// TODO: automatically get gvr instead of using hardcoded value
	gvr := runtimeSchema.GroupVersionResource{
		Version:  "v1",
		Resource: "nodes",
	}
	fakeClient.Tracker().Delete(gvr, mscnode.GetNamespace(), mscnode.GetName(), metav1.DeleteOptions{})

	expectedSubnets = []string{"subnet-def"}
	i = 0
	for i < 5 {
		if ok, _ := verifySubnetsInCR(t, expectedSubnets, nodeTopologyClient); ok {
			break
		} else {
			time.Sleep(time.Millisecond * 500)
			i++
		}
	}
	if i >= 5 {
		t.Fatalf("Delete node topology CR not working as expected")
	}
}

func TestUpdateUniqueNode(t *testing.T) {
	testClusterValues := gce.DefaultTestClusterValues()
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)
	nodeTopologySyncer := &NodeTopologySyncer{
		nodeTopologyClient: ntfakeclient.NewSimpleClientset(),
		cloud:              fakeGCE,
	}
	tests := []struct {
		name    string
		oldNode *v1.Node
		newNode *v1.Node
		queued  bool
	}{
		{
			name: "DuplicatedNodeLabel",
			oldNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet1",
					},
				},
			},
			newNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet1",
					},
				},
			},
			queued: false,
		},
		{
			name: "UpdatedNodeLable",
			oldNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet1",
					},
				},
			},
			newNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet2",
					},
				},
			},
			queued: true,
		},
		{
			name: "DifferentLabelName",
			oldNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						"cloud.google.com/unrelated": "subnet1",
					},
				},
			},
			newNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet1",
					},
				},
			},
			queued: true,
		},
		{
			name: "EmptyLabel",
			oldNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "testNode",
					Labels: map[string]string{},
				},
			},
			newNode: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNode",
					Labels: map[string]string{
						testNodePoolSubnetLabelPrefix: "subnet1",
					},
				},
			},
			queued: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodetopologyQueue := NewTaskQueue("nodetopologgTaskQueueForTest", "nodetopologyCRD", 1, nodeTopologyKeyFun, nodeTopologySyncer.sync)
			ca := &cloudCIDRAllocator{
				nodeTopologyQueue: nodetopologyQueue,
			}
			ca.updateUniqueNode(tc.oldNode, tc.newNode)
			expectLen := 0
			if tc.queued {
				expectLen = 1
			}
			got := nodetopologyQueue.queue.Len()
			if got != expectLen {
				t.Errorf("updateUniqueNode(%v, %v) returned queued %v, but want %v", tc.oldNode, tc.newNode, got, expectLen)
			}
		})
	}
}

func TestUpdateCIDRAllocation(t *testing.T) {
	ipv4ipv6Stack := stackIPv4IPv6
	ipv6ipv4Stack := stackIPv6IPv4
	ipv6Stack := stackIPv6

	tests := []struct {
		name            string
		fakeNodeHandler *testutil.FakeNodeHandler
		networks        []*networkv1.Network
		gkeNwParams     []*networkv1.GKENetworkParamSet
		nodeChanges     func(*v1.Node)
		gceInstance     []*compute.Instance
		stackType       *clusterStackType
		expectErr       bool
		expectErrMsg    string
		expectedUpdate  bool
		// expectedMetrics is optional if you'd also like to assert a metric
		expectedMetrics map[string]float64
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
			name: "want error - provider not set",
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
			name: "want error - node not found in gce by provider",
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
			name: "want error - gce node has no networks",
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
			name: "empty single stack node, single stack ipv4 cluster",
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
			name: "empty dual stack node, single stack ipv4 cluster",
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
			name: "empty dual stack node, IPv4IPv6 cluster",
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
			stackType: &ipv4ipv6Stack,
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
			name: "empty dual stack node, IPv6IPv4 cluster",
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
			stackType: &ipv6ipv4Stack,
			nodeChanges: func(node *v1.Node) {
				node.Spec.PodCIDR = "2001:db9::/112"
				node.Spec.PodCIDRs = []string{"2001:db9::/112", "192.168.1.0/24"}
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
			name: "empty single stack ipv6 node, single stack ipv6 cluster",
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
						},
					},
				},
			},
			stackType: &ipv6Stack,
			nodeChanges: func(node *v1.Node) {
				node.Spec.PodCIDR = "2001:db9::/112"
				node.Spec.PodCIDRs = []string{"2001:db9::/112"}
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
			name: "want error - incorrect ipv6 cidr instead of address",
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
			stackType:    &ipv4ipv6Stack,
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "failed to parse strings",
		},
		{
			name: "want error - incorrect ipv4 address instead of ipv6 address",
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
							Ipv6Address: "10.10.1.0",
							AliasIpRanges: []*compute.AliasIpRange{
								{
									IpCidrRange: "192.168.1.0/24",
								},
							},
						},
					},
				},
			},
			stackType:    &ipv4ipv6Stack,
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "err: IPs are not dual stack",
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
		{
			name: "[mn] default network only",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, false),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
							{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeB},
						}),
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
				node.Annotations = map[string]string{
					networkv1.NorthInterfacesAnnotationKey: "[]",
					networkv1.MultiNetworkAnnotationKey:    "[]",
				}
			},
			expectedUpdate:  true,
			expectedMetrics: map[string]float64{},
		},
		{
			name: "[mn] default network only, get PodCIDR with node labels",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, false),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, nil),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Labels: map[string]string{
								utilnode.DefaultSubnetLabelPrefix:    "default",
								utilnode.NodePoolPodRangeLabelPrefix: defaultSecondaryRangeA,
							},
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
							{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeB},
						}),
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
				node.Annotations = map[string]string{
					networkv1.NorthInterfacesAnnotationKey: "[]",
					networkv1.MultiNetworkAnnotationKey:    "[]",
				}
			},
			expectedUpdate:  true,
			expectedMetrics: map[string]float64{},
		},
		{
			name: "[mn] one additional network along with default network",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
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
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] one additional network (PSC aka network attachment) along with default network",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParamsWithNetworkAttachment(redGKENetworkParamsName, redNetworkAttachmentName),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(redNetworkName, redNetworkAttachmentName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24"},
						}),
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
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			// this is incorrect configuration, Network should be Device type in such situation
			// no annotation change for such network
			name: "[mn] no secondary ranges in GKENetworkParams",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/24", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] networks without matching gce interfaces should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{blueSecondaryRangeA}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName:  float64(1),
				blueNetworkName: float64(0),
			},
		},
		{
			name: "[mn] networks (PSC aka network attachment) without matching gce interfaces should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParamsWithNetworkAttachment(redGKENetworkParamsName, redNetworkAttachmentName),
				gkeNetworkParamsWithNetworkAttachment(blueGKENetworkParamsName, blueNetworkAttachmentName),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(redVPCName, redNetworkAttachmentName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24"},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName:  float64(1),
				blueNetworkName: float64(0),
			},
		},
		{
			name: "[mn] 2 additional networks",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{blueSecondaryRangeA}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
					"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName:  float64(1),
				blueNetworkName: float64(1),
			},
		},
		{
			name: "[mn] 2 additional networks (PSC aka network attachment)",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParamsWithNetworkAttachment(redGKENetworkParamsName, redNetworkAttachmentName),
				gkeNetworkParamsWithNetworkAttachment(blueGKENetworkParamsName, blueNetworkAttachmentName),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(redVPCName, redNetworkAttachmentName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24"},
						}),
						interfacesWithNetworkAttachment(blueVPCName, blueNetworkAttachmentName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26"},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
					"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName:  float64(1),
				blueNetworkName: float64(1),
			},
		},
		{
			name: "[mn] 2 additional networks (only one is PSC aka network attachment)",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeA}),
				gkeNetworkParamsWithNetworkAttachment(blueGKENetworkParamsName, blueNetworkAttachmentName),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(blueVPCName, blueNetworkAttachmentName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26"},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
					"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName:  float64(1),
				blueNetworkName: float64(1),
			},
		},
		{
			name: "[mn] interfaces without matching k8s networks should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/24", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] interfaces (PSC aka network attachment) without matching k8s networks should be ignored",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(blueVPCName, blueNetworkAttachmentName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/24"},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] want error - node with cidrs in incorrect format",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "30.20.1000/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges:  func(node *v1.Node) {},
			expectErr:    true,
			expectErrMsg: "invalid CIDR address",
		},
		{
			name: "[mn] want error - one additional network with /32 cidr",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/32", SubnetworkRangeName: redSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/32\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(1, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] node already configured",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{blueSecondaryRangeA}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey:     fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
								networkv1.NorthInterfacesAnnotationKey: fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName),
								networkv1.MultiNetworkAnnotationKey:    fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
								"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
							},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {},
		},
		{
			name: "[mn] node already configured with PSC aka network attachment",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParamsWithNetworkAttachment(redGKENetworkParamsName, redNetworkAttachmentName),
				gkeNetworkParamsWithNetworkAttachment(blueGKENetworkParamsName, blueNetworkAttachmentName),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey:     fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
								networkv1.NorthInterfacesAnnotationKey: fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName),
								networkv1.MultiNetworkAnnotationKey:    fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
								"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
							},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfacesWithNetworkAttachment(redVPCName, redNetworkAttachmentName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24"},
						}),
						interfacesWithNetworkAttachment(blueVPCName, blueNetworkAttachmentName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26"},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {},
		},
		{
			name: "[mn] blue network status change to down",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{blueSecondaryRangeA}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey:     fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
								networkv1.NorthInterfacesAnnotationKey: fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName),
								networkv1.MultiNetworkAnnotationKey:    fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
								"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
							},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
		},
		{
			name: "[mn] blue network becomes not ready",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
				network(blueNetworkName, blueGKENetworkParamsName, false),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
				gkeNetworkParams(blueGKENetworkParamsName, blueVPCName, blueVPCSubnetName, []string{blueSecondaryRangeA}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey:     fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName, blueNetworkName),
								networkv1.NorthInterfacesAnnotationKey: fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"},{\"network\":\"%s\",\"ipAddress\":\"84.1.2.1\"}]", redNetworkName, blueNetworkName),
								networkv1.MultiNetworkAnnotationKey:    fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"},{\"name\":\"%s\",\"cidrs\":[\"20.28.1.0/26\"],\"scope\":\"host-local\"}]", redNetworkName, blueNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							PodCIDR:    "192.168.1.0/24",
							PodCIDRs:   []string{"192.168.1.0/24"},
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"networking.gke.io.networks/Red-Network.IP":  *resource.NewQuantity(128, resource.DecimalSI),
								"networking.gke.io.networks/Blue-Network.IP": *resource.NewQuantity(32, resource.DecimalSI),
							},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
						interfaces(blueVPCName, blueVPCSubnetName, "84.1.2.1", []*compute.AliasIpRange{
							{IpCidrRange: "20.28.1.0/26", SubnetworkRangeName: blueSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(128, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
		},
		{
			name: "[mn] one additional device network along with default network",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				networkAll(redNetworkName, redGKENetworkParamsName, networkv1.DeviceNetworkType, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey: fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
							},
						},
						Spec: v1.NodeSpec{
							ProviderID: "gce://test-project/us-central1-b/test",
						},
						Status: v1.NodeStatus{
							Capacity: v1.ResourceList{},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			gceInstance: []*compute.Instance{
				{
					Name: "test",
					NetworkInterfaces: []*compute.NetworkInterface{
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
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
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName)
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"10.1.1.1/32\"],\"scope\":\"host-local\"}]", redNetworkName)
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/Red-Network.IP": *resource.NewQuantity(1, resource.DecimalSI),
				}
			},
			expectedUpdate: true,
			expectedMetrics: map[string]float64{
				redNetworkName: float64(1),
			},
		},
		{
			name: "[mn] node with capacity not configured",
			networks: []*networkv1.Network{
				network(networkv1.DefaultPodNetworkName, defaultGKENetworkParamsName, true),
				network(redNetworkName, redGKENetworkParamsName, true),
			},
			gkeNwParams: []*networkv1.GKENetworkParamSet{
				gkeNetworkParams(defaultGKENetworkParamsName, defaultVPCName, defaultVPCSubnetName, []string{defaultSecondaryRangeA, defaultSecondaryRangeB}),
				gkeNetworkParams(redGKENetworkParamsName, redVPCName, redVPCSubnetName, []string{redSecondaryRangeA, redSecondaryRangeB}),
			},
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								networkv1.NodeNetworkAnnotationKey:     fmt.Sprintf("[{\"name\":\"%s\"},{\"name\":\"%s\"}]", networkv1.DefaultPodNetworkName, redNetworkName),
								networkv1.NorthInterfacesAnnotationKey: fmt.Sprintf("[{\"network\":\"%s\",\"ipAddress\":\"10.1.1.1\"}]", redNetworkName),
								networkv1.MultiNetworkAnnotationKey:    fmt.Sprintf("[{\"name\":\"%s\",\"cidrs\":[\"172.11.1.0/24\"],\"scope\":\"host-local\"}]", redNetworkName),
							},
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
						interfaces(defaultVPCName, defaultVPCSubnetName, "80.1.172.1", []*compute.AliasIpRange{
							{IpCidrRange: "192.168.1.0/24", SubnetworkRangeName: defaultSecondaryRangeA},
						}),
						interfaces(redVPCName, redVPCSubnetName, "10.1.1.1", []*compute.AliasIpRange{
							{IpCidrRange: "172.11.1.0/24", SubnetworkRangeName: redSecondaryRangeA},
						}),
					},
				},
			},
			nodeChanges: func(node *v1.Node) {
				if node.Status.Capacity == nil {
					node.Status.Capacity = map[v1.ResourceName]resource.Quantity{}
				}
				node.Status.Capacity["networking.gke.io.networks/Red-Network.IP"] = *resource.NewQuantity(128, resource.DecimalSI)
			},
			expectedUpdate: true,
		},
	}

	registerCloudCidrAllocatorMetrics()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			multiNetworkNodes.Reset()
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			testClusterValues := gce.DefaultTestClusterValues()
			fakeGCE := gce.NewFakeGCECloud(testClusterValues)
			for _, inst := range tc.gceInstance {
				err := fakeGCE.Compute().Instances().Insert(ctx, meta.ZonalKey(inst.Name, testClusterValues.ZoneName), inst)
				if err != nil {
					t.Fatalf("error setting up the test for fakeGCE: %v", err)
				}
			}

			clientSet := clSetFake.NewSimpleClientset()
			nwInfFactory := networkinformers.NewSharedInformerFactory(clientSet, 1*time.Second).Networking()
			nwInformer := nwInfFactory.V1().Networks()
			gnpInformer := nwInfFactory.V1().GKENetworkParamSets()
			for _, nw := range tc.networks {
				err := nwInformer.Informer().GetStore().Add(nw)
				if err != nil {
					t.Fatalf("error in test setup, could not create network %s: %v", nw.Name, err)
				}
			}
			for _, gnp := range tc.gkeNwParams {
				err := gnpInformer.Informer().GetStore().Add(gnp)
				if err != nil {
					t.Fatalf("error in test setup, could not create gke network param set %s: %v", gnp.Name, err)
				}
			}
			fakeNodeInformer := getFakeNodeInformer(tc.fakeNodeHandler)

			wantNode := tc.fakeNodeHandler.Existing[0].DeepCopy()
			tc.nodeChanges(wantNode)

			stackType := stackIPv4
			if tc.stackType != nil {
				stackType = *tc.stackType
			}
			nodeTopologySyncer := &NodeTopologySyncer{
				nodeTopologyClient: ntfakeclient.NewSimpleClientset(),
				cloud:              fakeGCE,
			}
			nodetopologyQueue := NewTaskQueue("nodetopologgTaskQueueForTest", "nodetopologyCRD", 1, nodeTopologyKeyFun, nodeTopologySyncer.sync)

			ca := &cloudCIDRAllocator{
				client:            tc.fakeNodeHandler,
				cloud:             fakeGCE,
				recorder:          testutil.NewFakeRecorder(),
				nodeLister:        fakeNodeInformer.Lister(),
				nodesSynced:       fakeNodeInformer.Informer().HasSynced,
				networksLister:    nwInformer.Lister(),
				gnpLister:         gnpInformer.Lister(),
				stackType:         stackType,
				nodeTopologyQueue: nodetopologyQueue,
			}

			// test
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

			if tc.expectedMetrics != nil {
				for nw, em := range tc.expectedMetrics {
					m, err := metricsUtil.GetGaugeMetricValue(multiNetworkNodes.WithLabelValues(nw))
					if err != nil {
						t.Errorf("failed to get %s value, err: %v", multiNetworkNodes.Name, err)
					}
					if m != em {
						t.Fatalf("metrics error: expected %v, received %v for %v", em, m, nw)
					}
				}
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

func gkeNetworkParamsWithNetworkAttachment(name, networkAttachment string) *networkv1.GKENetworkParamSet {
	gnp := &networkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkv1.GKENetworkParamSetSpec{
			NetworkAttachment: networkAttachment,
		},
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

func interfacesWithNetworkAttachment(network, networkAttachment, networkIP string, aliasIPRanges []*compute.AliasIpRange) *compute.NetworkInterface {
	return &compute.NetworkInterface{
		AliasIpRanges:     aliasIPRanges,
		Network:           network,
		NetworkAttachment: networkAttachment,
		NetworkIP:         networkIP,
	}
}

func TestIsIP4_net_nil(t *testing.T) {
	if isIP4(nil) != false {
		t.Fatalf("isIP4(nil) = true, want false")
	}
}

func TestIsIP4_ip_nil(t *testing.T) {
	ipnet := &net.IPNet{IP: nil}
	if isIP4(ipnet) != false {
		t.Fatalf("isIP4(%+v) = true, want false", ipnet)
	}
}

func TestIsIP4(t *testing.T) {
	testCases := []struct {
		desc string
		cidr string
		want bool
	}{
		{
			desc: "ipv4 cidr",
			cidr: "10.1.0.0/16",
			want: true,
		},
		{
			desc: "ipv6 cidr",
			cidr: "2001:db9::/110",
			want: false,
		},
	}

	for _, tc := range testCases {
		_, ipnet, err := net.ParseCIDR(tc.cidr)
		if err != nil {
			t.Fatalf("net.ParseCIDR(%v): %v", tc.cidr, err)
		}

		if isIP4(ipnet) != tc.want {
			t.Fatalf("ipIP4(%+v): %v, want %v", ipnet, isIP4(ipnet), tc.want)
		}
	}
}

func TestIsIP6_net_nil(t *testing.T) {
	if isIP6(nil) != false {
		t.Fatalf("isIP6(nil) = true, want false")
	}
}

func TestIsIP6_ip_nil(t *testing.T) {
	ipnet := &net.IPNet{IP: nil}
	if isIP6(ipnet) != false {
		t.Fatalf("isIP6(%+v) = true, want false", ipnet)
	}
}

func TestIsIP6(t *testing.T) {
	testCases := []struct {
		desc string
		cidr string
		want bool
	}{
		{
			desc: "ipv4 cidr",
			cidr: "10.1.0.0/16",
			want: false,
		},
		{
			desc: "ipv6 cidr",
			cidr: "2001:db9::/110",
			want: true,
		},
	}

	for _, tc := range testCases {
		_, ipnet, err := net.ParseCIDR(tc.cidr)
		if err != nil {
			t.Fatalf("net.ParseCIDR(%v): %v", tc.cidr, err)
		}

		if isIP6(ipnet) != tc.want {
			t.Fatalf("ipIP6(%+v): %v, want %v", ipnet, isIP6(ipnet), tc.want)
		}
	}
}

func TestNodeMultiNetworkChanged(t *testing.T) {
	oldNode := &v1.Node{}
	oldNode.Annotations = map[string]string{
		networkv1.NodeNetworkAnnotationKey:     "abc",
		networkv1.MultiNetworkAnnotationKey:    "123",
		networkv1.NorthInterfacesAnnotationKey: "def",
	}
	oldNode.Status.Capacity = v1.ResourceList{
		v1.ResourceName(networkv1.NetworkResourceKeyPrefix + "fake-net.IP"): *resource.NewQuantity(1, resource.DecimalSI),
	}
	tests := []struct {
		desc           string
		newNodeMutator func(*v1.Node)
		want           bool
	}{
		{
			desc:           "no change",
			newNodeMutator: nil,
			want:           false,
		},
		{
			desc: "IP resource changed",
			newNodeMutator: func(n *v1.Node) {
				n.Status.Capacity[v1.ResourceName(networkv1.NetworkResourceKeyPrefix+"fake-net.IP")] = *resource.NewQuantity(0, resource.DecimalSI)
			},
			want: true,
		},
		{
			desc: "north-interface changed",
			newNodeMutator: func(n *v1.Node) {
				n.Annotations[networkv1.NorthInterfacesAnnotationKey] = "changed"
			},
			want: true,
		},
		{
			desc: "networks changed",
			newNodeMutator: func(n *v1.Node) {
				n.Annotations[networkv1.MultiNetworkAnnotationKey] = "changed"
			},
			want: true,
		},
		{
			desc: "network-status changed",
			newNodeMutator: func(n *v1.Node) {
				n.Annotations[networkv1.NodeNetworkAnnotationKey] = "changed"
			},
			want: true,
		},
		{
			desc: "annotation cleared",
			newNodeMutator: func(n *v1.Node) {
				delete(n.Annotations, networkv1.NorthInterfacesAnnotationKey)
			},
			want: true,
		},
		{
			desc: "nil annotation",
			newNodeMutator: func(n *v1.Node) {
				n.Annotations = nil
			},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			newNode := oldNode.DeepCopy()
			if tc.newNodeMutator != nil {
				tc.newNodeMutator(newNode)
			}

			got := nodeMultiNetworkChanged(oldNode, newNode)
			if got != tc.want {
				t.Fatalf("nodeMultiNetworkChanged(%+v, %+v) return %t, but want %t", oldNode, newNode, got, tc.want)
			}
		})
	}
}
