//go:build !providerless
// +build !providerless

/*
Copyright 2020 The Kubernetes Authors.

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

package gce

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ga "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

func TestInstanceExists(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	_, err = createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	testcases := []struct {
		name        string
		nodeName    string
		exist       bool
		expectedErr error
	}{
		{
			name:        "node exist",
			nodeName:    "test-node-1",
			exist:       true,
			expectedErr: nil,
		},
		{
			name:        "node not exist",
			nodeName:    "test-node-2",
			exist:       false,
			expectedErr: nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: test.nodeName}}
			exist, err := gce.InstanceExists(context.TODO(), node)
			assert.Equal(t, test.expectedErr, err, test.name)
			assert.Equal(t, test.exist, exist, test.name)
		})
	}
}

func TestNodeAddresses(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	instanceMap := make(map[string]*ga.Instance)
	// n1 is dual stack instance with internal IPv6 address
	instance := &ga.Instance{
		Name: "n1",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				NetworkIP:   "10.1.1.1",
				StackType:   "IPV4_IPV6",
				Ipv6Address: "2001:2d00::0:1",
			},
		},
	}
	instanceMap["n1"] = instance

	// n2 is dual stack instance with external IPv6 address
	instance = &ga.Instance{
		Name: "n2",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				NetworkIP:      "10.1.1.2",
				StackType:      "IPV4_IPV6",
				Ipv6AccessType: "EXTERNAL",
				Ipv6AccessConfigs: []*ga.AccessConfig{
					{ExternalIpv6: "2001:1900::0:2"},
				},
				AccessConfigs: []*ga.AccessConfig{
					{NatIP: "20.1.1.2"},
				},
			},
		},
	}
	instanceMap["n2"] = instance

	// n4 is instance with invalid network interfaces
	instance = &ga.Instance{
		Name: "n4",
		Zone: "us-central1-b",
	}
	instanceMap["n4"] = instance

	// n5 is a single stack IPv4 instance
	instance = &ga.Instance{
		Name: "n5",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				NetworkIP: "10.1.1.5",
				StackType: "IPV4",
				AccessConfigs: []*ga.AccessConfig{
					{NatIP: "20.1.1.5"},
				},
			},
		},
	}
	instanceMap["n5"] = instance

	// n6 is a single stack IPv6 instance with internal IPv6 address
	instance = &ga.Instance{
		Name: "n6",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				StackType:   "IPV6",
				Ipv6Address: "2001:2d00::0:1",
			},
		},
	}
	instanceMap["n6"] = instance

	// n7 is single stack IPv6 instance with external IPv6 address
	instance = &ga.Instance{
		Name: "n7",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				StackType:      "IPV6",
				Ipv6AccessType: "EXTERNAL",
				Ipv6AccessConfigs: []*ga.AccessConfig{
					{ExternalIpv6: "2001:1900::0:2"},
				},
			},
		},
	}
	instanceMap["n7"] = instance

	mockGCE := gce.c.(*cloud.MockGCE)
	mi := mockGCE.Instances().(*cloud.MockInstances)
	mi.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockInstances, options ...cloud.Option) (bool, *ga.Instance, error) {
		ret, ok := instanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("instance not found")
		}
		return true, ret, nil
	}

	testcases := []struct {
		name      string
		nodeName  string
		stackType StackType
		wantErr   string
		wantAddrs []v1.NodeAddress
	}{
		{
			name:      "internal dual stack instance with cluster stack type IPv4",
			nodeName:  "n1",
			stackType: clusterStackIPV4,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.1"},
				{Type: v1.NodeInternalIP, Address: "2001:2d00::0:1"},
			},
		},
		{
			name:      "internal dual stack instance with cluster stack type dual",
			nodeName:  "n1",
			stackType: clusterStackDualStack,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.1"},
				{Type: v1.NodeInternalIP, Address: "2001:2d00::0:1"},
			},
		},
		{
			name:      "internal dual stack instance with cluster stack type IPv6",
			nodeName:  "n1",
			stackType: clusterStackIPV6,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "2001:2d00::0:1"},
				{Type: v1.NodeInternalIP, Address: "10.1.1.1"},
			},
		},
		{
			name:      "external dual stack instance with cluster stack type IPv4",
			nodeName:  "n2",
			stackType: clusterStackIPV4,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.2"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.2"},
				{Type: v1.NodeInternalIP, Address: "2001:1900::0:2"},
			},
		},
		{
			name:      "external dual stack instance with cluster stack type dual",
			nodeName:  "n2",
			stackType: clusterStackDualStack,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.2"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.2"},
				{Type: v1.NodeInternalIP, Address: "2001:1900::0:2"},
			},
		},
		{
			name:      "external dual stack instance with cluster stack type IPv6",
			nodeName:  "n2",
			stackType: clusterStackIPV6,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "2001:1900::0:2"},
				{Type: v1.NodeInternalIP, Address: "10.1.1.2"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.2"},
			},
		},
		{
			name:      "instance not found with cluster stack type IPv4",
			nodeName:  "x1",
			stackType: clusterStackIPV4,
			wantErr:   "instance not found",
		},
		{
			name:      "instance not found with cluster stack type dual",
			nodeName:  "x1",
			stackType: clusterStackDualStack,
			wantErr:   "instance not found",
		},
		{
			name:      "instance not found with cluster stack type IPv6",
			nodeName:  "x1",
			stackType: clusterStackIPV6,
			wantErr:   "instance not found",
		},
		{
			name:      "network interface not found with cluster stack type IPv4",
			nodeName:  "n4",
			stackType: clusterStackIPV4,
			wantErr:   "could not find network interface",
		},
		{
			name:      "network interface not found with cluster stack type dual",
			nodeName:  "n4",
			stackType: clusterStackDualStack,
			wantErr:   "could not find network interface",
		},
		{
			name:      "network interface not found with cluster stack type IPv6",
			nodeName:  "n4",
			stackType: clusterStackIPV6,
			wantErr:   "could not find network interface",
		},
		{
			name:      "single stack instance with cluster stack type IPv4",
			nodeName:  "n5",
			stackType: clusterStackIPV4,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.5"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.5"},
			},
		},
		{
			name:      "single stack IPv6 instance with internal IPv6 address and cluster stack type IPv6",
			nodeName:  "n6",
			stackType: clusterStackIPV6,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "2001:2d00::0:1"},
			},
		},
		{
			name:      "single stack IPv6 instance with external IPv6 address and cluster stack type IPv6",
			nodeName:  "n7",
			stackType: clusterStackIPV6,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "2001:1900::0:2"},
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			SetFakeStackType(gce, test.stackType)

			gotAddrs, err := gce.NodeAddresses(context.Background(), types.NodeName(test.nodeName))
			if err != nil && (test.wantErr == "" || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("gce.NodeAddresses. Want err: %v, got: %v", test.wantErr, err)
				return
			} else if err == nil && test.wantErr != "" {
				t.Errorf("gce.NodeAddresses. Want err: %v, got: %v", test.wantErr, err)
			}
			assert.Equal(t, test.wantAddrs, gotAddrs)
		})
	}
}

func TestAliasRangesByProviderID(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	instanceMap := make(map[string]*ga.Instance)
	// n1 is instance with internal IPv6 address
	instance := &ga.Instance{
		Name: "n1",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				AliasIpRanges: []*ga.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24"},
				},
				NetworkIP:   "10.1.1.1",
				StackType:   "IPV4_IPV6",
				Ipv6Address: "2001:2d00::1:0:0",
			},
		},
	}
	instanceMap["n1"] = instance

	// n2 is instance with external IPv6 address
	instance = &ga.Instance{
		Name: "n2",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				AliasIpRanges: []*ga.AliasIpRange{
					{IpCidrRange: "10.11.2.0/24"},
				},
				NetworkIP:      "10.1.1.2",
				StackType:      "IPV4_IPV6",
				Ipv6AccessType: "EXTERNAL",
				Ipv6AccessConfigs: []*ga.AccessConfig{
					{ExternalIpv6: "2001:1900::2:0:0"},
				},
				AccessConfigs: []*ga.AccessConfig{
					{NatIP: "20.1.1.2"},
				},
			},
		},
	}
	instanceMap["n2"] = instance

	// n4 is instance with invalid network interfaces
	instance = &ga.Instance{
		Name: "n4",
		Zone: "us-central1-b",
	}
	instanceMap["n4"] = instance

	// n5 is a single stack instance
	instance = &ga.Instance{
		Name: "n5",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				AliasIpRanges: []*ga.AliasIpRange{
					{IpCidrRange: "10.11.5.0/24"},
				},
				NetworkIP: "10.1.1.5",
				StackType: "IPV4",
				AccessConfigs: []*ga.AccessConfig{
					{NatIP: "20.1.1.5"},
				},
			},
		},
	}
	instanceMap["n5"] = instance

	mockGCE := gce.c.(*cloud.MockGCE)
	mai := mockGCE.Instances().(*cloud.MockInstances)
	mai.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockInstances, options ...cloud.Option) (bool, *ga.Instance, error) {
		ret, ok := instanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("instance not found")
		}
		return true, ret, nil
	}

	testcases := []struct {
		name       string
		providerId string
		wantErr    string
		wantCIDRs  []string
	}{
		{
			name:       "internal single stack instance",
			providerId: "gce://p1/us-central1-b/n1",
			wantCIDRs: []string{
				"10.11.1.0/24",
				"2001:2d00::1:0:0/112",
			},
		},
		{
			name:       "instance not found",
			providerId: "gce://p1/us-central1-b/x1",
			wantErr:    "instance not found",
		},
		{
			name:       "internal single stack instance",
			providerId: "gce://p1/us-central1-b/n2",
			wantCIDRs: []string{
				"10.11.2.0/24",
				"2001:1900::2:0:0/112",
			},
		},
		{
			name:       "network interface not found",
			providerId: "gce://p1/us-central1-b/n4",
			wantErr:    "",
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			gotCIDRs, err := gce.AliasRangesByProviderID(test.providerId)
			if err != nil && (test.wantErr == "" || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("gce.AliasRangesByProviderID. Want err: %v, got: %v", test.wantErr, err)
			} else if err == nil && test.wantErr != "" {
				t.Errorf("gce.AliasRangesByProviderID. Want err: %v, got: %v, gotCIDRs: %v", test.wantErr, err, gotCIDRs)
			}
			assert.Equal(t, test.wantCIDRs, gotCIDRs)
		})
	}
}

func TestInstanceByProviderID(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	instanceMap := make(map[string]*ga.Instance)
	interfaces := []*ga.NetworkInterface{
		{
			AliasIpRanges: []*ga.AliasIpRange{
				{IpCidrRange: "10.11.1.0/24", SubnetworkRangeName: "range-A"},
			},
			NetworkIP:  "10.1.1.1",
			Network:    "network-A",
			Subnetwork: "subnetwork-A",
		},
		{
			AliasIpRanges: []*ga.AliasIpRange{
				{IpCidrRange: "20.11.1.0/24", SubnetworkRangeName: "range-B"},
			},
			NetworkIP:  "20.1.1.1",
			Network:    "network-B",
			Subnetwork: "subnetwork-B",
		},
	}
	// n1 is instance with 2 network interfaces
	instance := &ga.Instance{
		Name:              "n1",
		Zone:              "us-central1-b",
		NetworkInterfaces: interfaces,
	}
	instanceMap["n1"] = instance

	mockGCE := gce.c.(*cloud.MockGCE)
	mai := mockGCE.Instances().(*cloud.MockInstances)
	mai.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockInstances, options ...cloud.Option) (bool, *ga.Instance, error) {
		ret, ok := instanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("instance not found")
		}
		return true, ret, nil
	}

	testcases := []struct {
		name         string
		providerId   string
		wantErr      string
		wantInstance *ga.Instance
	}{
		{
			name:       "invalid provider id",
			providerId: "gce://p1/x1",
			wantErr:    "error splitting providerID",
		},
		{
			name:       "instance not found",
			providerId: "gce://p1/us-central1-b/x1",
			wantErr:    "instance not found",
		},
		{
			name:         "instance with multiple interfaces",
			providerId:   "gce://p1/us-central1-b/n1",
			wantInstance: instance,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			gotInstance, err := gce.InstanceByProviderID(test.providerId)
			if err != nil && (test.wantErr == "" || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("gce.InstanceByProviderID. Want err: %v, got: %v", test.wantErr, err)
			} else if err == nil && test.wantErr != "" {
				t.Errorf("gce.InstanceByProviderID. Want err: %v, got: %v, gotInstances: %v", test.wantErr, err, gotInstance)
			}
			assert.Equal(t, test.wantInstance, gotInstance)
		})
	}
}

func TestGetZone(t *testing.T) {
	testCases := []struct {
		nodeLabels   map[string]string
		expectedZone string
	}{
		{
			nodeLabels:   nil,
			expectedZone: emptyZone,
		},
		{
			nodeLabels:   map[string]string{v1.LabelTopologyZone: "zone-a"},
			expectedZone: "zone-a",
		},
		{
			nodeLabels:   map[string]string{v1.LabelFailureDomainBetaZone: "zone-b"},
			expectedZone: "zone-b",
		},
		{
			nodeLabels:   map[string]string{v1.LabelTopologyRegion: "us-central1"},
			expectedZone: emptyZone,
		},
	}
	for _, tc := range testCases {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: tc.nodeLabels,
			},
		}

		gotZone := getZone(node)
		if gotZone != tc.expectedZone {
			t.Errorf("Wrong labels from node labels: %v, got: %v, want: %v", tc.nodeLabels, gotZone, tc.expectedZone)
		}
	}
}

// TestProjectFromNodeProviderID verifies the behaviour of various functions under the
// influence of projectFromNodeProviderID option.
//
// Test setup involves creating two instances which have the same name and are
// in the same zone but differ in their projects. A K8s Node is created with the
// providerID pointing to the non-default project.
//   - When projectFromNodeProviderID is not enabled, the instance from the
//     default project should be returned (since the project from providerID
//     should be ignored.)
//   - When projectFromNodeProviderID is enabled, the instance from the
//     non-default project should be returned (since the project from providerID
//     would be considered.)
func TestProjectFromNodeProviderID(t *testing.T) {
	defaultValues := DefaultTestClusterValues()
	gce, err := fakeGCECloud(defaultValues)
	require.NoError(t, err)

	// instanceMap maps the instance's selfLink to the instance.
	instanceMap := map[string]*ga.Instance{}

	// Instance in the default project.
	instanceFromDefaultProject := &ga.Instance{
		SelfLink: fmt.Sprintf("projects/%v/zones/us-central1-c/instances/instance-1", defaultValues.ProjectID),
		Id:       1,
		NetworkInterfaces: []*ga.NetworkInterface{{
			NetworkIP: "1.1.1.1",
			StackType: "IPV4",
			AliasIpRanges: []*ga.AliasIpRange{
				{IpCidrRange: "10.10.10.10/24"},
			},
		}},
	}
	instanceMap[instanceFromDefaultProject.SelfLink] = instanceFromDefaultProject

	// Instance in a different project.
	nonDefaultProject := "non-default-project"
	instanceFromNonDefaultProject := &ga.Instance{
		SelfLink: fmt.Sprintf("projects/%v/zones/us-central1-c/instances/instance-1", nonDefaultProject),
		Id:       2,
		NetworkInterfaces: []*ga.NetworkInterface{{
			NetworkIP: "2.2.2.2",
			StackType: "IPV4",
			AliasIpRanges: []*ga.AliasIpRange{
				{IpCidrRange: "20.20.20.20/24"},
			},
		}},
	}
	instanceMap[instanceFromNonDefaultProject.SelfLink] = instanceFromNonDefaultProject
	forceNonDefaultProject := cloud.ForceProjectID(nonDefaultProject)

	// Setup mock response.
	mockGCE := gce.c.(*cloud.MockGCE)
	mi := mockGCE.Instances().(*cloud.MockInstances)
	mi.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockInstances, options ...cloud.Option) (bool, *ga.Instance, error) {
		projectID := defaultValues.ProjectID
		if len(options) == 1 && reflect.DeepEqual(options[0], forceNonDefaultProject) {
			projectID = nonDefaultProject
		}
		selfLink := fmt.Sprintf("projects/%v/zones/%v/instances/%v", projectID, key.Zone, key.Name)
		ret, ok := instanceMap[selfLink]
		if !ok {
			return true, nil, fmt.Errorf("instance not found")
		}
		return true, ret, nil
	}

	node := &v1.Node{
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("gce://%v/us-central1-c/instance-1", nonDefaultProject),
		},
	}

	// Invoke functions under test

	testCases := []struct {
		projectFromNodeProviderID bool
		wantInstance              *ga.Instance
	}{
		{
			projectFromNodeProviderID: false,
			wantInstance:              instanceFromDefaultProject,
		},
		{
			projectFromNodeProviderID: true,
			wantInstance:              instanceFromNonDefaultProject,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("instanceByProviderID() when projectFromNodeProviderID=%v", tc.projectFromNodeProviderID), func(t *testing.T) {
			gce.projectFromNodeProviderID = tc.projectFromNodeProviderID
			gotGCEInstance, err := gce.instanceByProviderID(node.Spec.ProviderID)
			if err != nil {
				t.Fatalf("instanceByProviderID(%v) = %v; want nil", node.Spec.ProviderID, err)
			}
			if gotGCEInstance.ID != tc.wantInstance.Id {
				t.Errorf("instanceByProviderID(%v) returned instance with ID = %v; want instance with ID = %v", node.Spec.ProviderID, gotGCEInstance.ID, instanceFromDefaultProject.Id)
			}
		})

		t.Run(fmt.Sprintf("InstanceMetadata() when projectFromNodeProviderID=%v", tc.projectFromNodeProviderID), func(t *testing.T) {
			gce.projectFromNodeProviderID = tc.projectFromNodeProviderID
			gotInstanceMetadata, err := gce.InstanceMetadata(context.TODO(), node)
			if err != nil {
				t.Fatalf("InstanceMetadata(%v) = %v; want nil", node.Spec.ProviderID, err)
			}
			gotAddress := gotInstanceMetadata.NodeAddresses[0].Address
			wantAddress := tc.wantInstance.NetworkInterfaces[0].NetworkIP
			if gotAddress != wantAddress {
				t.Errorf("InstanceMetadata(%v) returned instance with IP address = %v; want instance with IP address = %v", node.Spec.ProviderID, gotAddress, wantAddress)
			}
		})

		t.Run(fmt.Sprintf("AliasRangesByProviderID() when projectFromNodeProviderID=%v", tc.projectFromNodeProviderID), func(t *testing.T) {
			gce.projectFromNodeProviderID = tc.projectFromNodeProviderID
			gotAliasRanges, err := gce.AliasRangesByProviderID(node.Spec.ProviderID)
			if err != nil {
				t.Fatalf("AliasRangesByProviderID(%v) = %v; want nil", node.Spec.ProviderID, err)
			}
			if len(gotAliasRanges) != 1 {
				t.Fatalf("AliasRangesByProviderID(%v) returned %d ranges; want only 1", node.Spec.ProviderID, len(gotAliasRanges))
			}
			gotAliasRange := gotAliasRanges[0]
			wantAliasRange := tc.wantInstance.NetworkInterfaces[0].AliasIpRanges[0].IpCidrRange
			if gotAliasRange != wantAliasRange {
				t.Errorf("AliasRangesByProviderID(%v) returned instance with alias IP range = %v; want instance with alias IP range = %v", node.Spec.ProviderID, gotAliasRange, wantAliasRange)
			}
		})
	}
}
