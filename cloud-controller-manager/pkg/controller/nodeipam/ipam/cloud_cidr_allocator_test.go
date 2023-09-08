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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/testutil"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	clSetFake "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/fake"
	networkinformers "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions"
	"k8s.io/cloud-provider-gcp/providers/gce"
	metricsUtil "k8s.io/component-base/metrics/testutil"
	netutils "k8s.io/utils/net"
)

const (
	// Default Network
	defaultGKENetworkParamsName = "DefaultGKENetworkParams"
	defaultVPCName              = "projects/testProject/global/networks/default"
	defaultVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/default"
	defaultSecondaryRangeA      = "RangeA"
	defaultSecondaryRangeB      = "RangeB"
	// Red Network
	redNetworkName          = "Red-Network"
	redGKENetworkParamsName = "RedGKENetworkParams"
	redVPCName              = "projects/testProject/global/networks/red"
	redVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/red"
	redSecondaryRangeA      = "RedRangeA"
	redSecondaryRangeB      = "RedRangeB"
	// Blue Network
	blueNetworkName          = "Blue-Network"
	blueGKENetworkParamsName = "BlueGKENetworkParams"
	blueVPCName              = "projects/testProject/global/networks/blue"
	blueVPCSubnetName        = "projects/testProject/regions/us-central1/subnetworks/blue"
	blueSecondaryRangeA      = "BlueRangeA"
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

func withinExpectedRange(got time.Duration, expected time.Duration) bool {
	return got >= expected/2 && got <= 3*expected/2
}

func TestUpdateCIDRAllocation(t *testing.T) {
	tests := []struct {
		name            string
		fakeNodeHandler *testutil.FakeNodeHandler
		networks        []*networkv1.Network
		gkeNwParams     []*networkv1.GKENetworkParamSet
		nodeChanges     func(*v1.Node)
		gceInstance     []*compute.Instance
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
			name: "[mn] [invalid] - node with cidrs in incorrect format",
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
			name: "[mn] one additional network with /32 cidr",
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

			ca := &cloudCIDRAllocator{
				client:         tc.fakeNodeHandler,
				cloud:          fakeGCE,
				recorder:       testutil.NewFakeRecorder(),
				nodeLister:     fakeNodeInformer.Lister(),
				nodesSynced:    fakeNodeInformer.Informer().HasSynced,
				networksLister: nwInformer.Lister(),
				gnpLister:      gnpInformer.Lister(),
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
