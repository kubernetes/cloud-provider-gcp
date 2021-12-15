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
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	alpha "google.golang.org/api/compute/v0.alpha"
	ga "google.golang.org/api/compute/v1"
	"k8s.io/api/core/v1"
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
			expectedErr: fmt.Errorf("failed to get instance ID from cloud provider: instance not found"),
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
	alphaInstanceMap := make(map[string]*alpha.Instance)
	// n1 is instance with internal IPv6 address
	alphaInstance := &alpha.Instance{
		Name: "n1",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				NetworkIP:   "10.1.1.1",
				StackType:   "IPV4_IPV6",
				Ipv6Address: "2001:2d00::0:1",
			},
		},
	}
	alphaInstanceMap["n1"] = alphaInstance
	instance := &ga.Instance{
		Name: "n1",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				NetworkIP: "10.1.1.1",
			},
		},
	}
	instanceMap["n1"] = instance

	// n2 is instance with external IPv6 address
	alphaInstance = &alpha.Instance{
		Name: "n2",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				NetworkIP:      "10.1.1.2",
				StackType:      "IPV4_IPV6",
				Ipv6AccessType: "EXTERNAL",
				Ipv6AccessConfigs: []*alpha.AccessConfig{
					{ExternalIpv6: "2001:1900::0:2"},
				},
				AccessConfigs: []*alpha.AccessConfig{
					{NatIP: "20.1.1.2"},
				},
			},
		},
	}
	alphaInstanceMap["n2"] = alphaInstance
	instance = &ga.Instance{
		Name: "n2",
		Zone: "us-central1-b",
		NetworkInterfaces: []*ga.NetworkInterface{
			{
				NetworkIP: "10.1.1.2",
				AccessConfigs: []*ga.AccessConfig{
					{NatIP: "20.1.1.2"},
				},
			},
		},
	}
	instanceMap["n2"] = instance

	// n3 is instance not present in the alphaInstanceMap
	instance = &ga.Instance{
		Name: "n3",
		Zone: "us-central1-b",
	}
	instanceMap["n3"] = instance

	// n4 is instance with invalid network interfaces
	alphaInstance = &alpha.Instance{
		Name: "n4",
		Zone: "us-central1-b",
	}
	alphaInstanceMap["n4"] = alphaInstance
	instance = &ga.Instance{
		Name: "n4",
		Zone: "us-central1-b",
	}
	instanceMap["n4"] = instance

	// n5 is a single stack instance
	alphaInstance = &alpha.Instance{
		Name: "n5",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				NetworkIP: "10.1.1.5",
				StackType: "IPV4",
				AccessConfigs: []*alpha.AccessConfig{
					{NatIP: "20.1.1.5"},
				},
			},
		},
	}
	alphaInstanceMap["n5"] = alphaInstance
	instance = &ga.Instance{
		Name: "n5",
		Zone: "us-central1-b",
	}
	instanceMap["n5"] = instance

	mockGCE := gce.c.(*cloud.MockGCE)
	mai := mockGCE.AlphaInstances().(*cloud.MockAlphaInstances)
	mai.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockAlphaInstances) (bool, *alpha.Instance, error) {
		ret, ok := alphaInstanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("alpha instance not found")
		}
		return true, ret, nil
	}
	mi := mockGCE.Instances().(*cloud.MockInstances)
	mi.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockInstances) (bool, *ga.Instance, error) {
		ret, ok := instanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("instance not found")
		}
		return true, ret, nil
	}

	testcases := []struct {
		name      string
		nodeName  string
		dualStack bool
		wantErr   string
		wantAddrs []v1.NodeAddress
	}{
		{
			name:     "internal single stack instance",
			nodeName: "n1",
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.1"},
			},
		},
		{
			name:      "internal dual stack instance",
			nodeName:  "n1",
			dualStack: true,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.1"},
				{Type: v1.NodeInternalIP, Address: "2001:2d00::0:1"},
			},
		},
		{
			name:     "instance not found",
			nodeName: "x1",
			wantErr:  "instance not found",
		},
		{
			name:      "alpha instance not found",
			nodeName:  "n3",
			dualStack: true,
			wantErr:   "alpha instance not found",
		},
		{
			name:     "external single stack instance",
			nodeName: "n2",
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.2"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.2"},
			},
		},
		{
			name:      "external dual stack instance",
			nodeName:  "n2",
			dualStack: true,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.2"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.2"},
				{Type: v1.NodeInternalIP, Address: "2001:1900::0:2"},
			},
		},
		{
			name:     "network interface not found",
			nodeName: "n4",
			wantErr:  "could not find network interface",
		},
		{
			name:      "single stack instance",
			nodeName:  "n5",
			dualStack: true,
			wantAddrs: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.1.1.5"},
				{Type: v1.NodeExternalIP, Address: "20.1.1.5"},
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			if test.dualStack {
				gce.stackType = NetworkStackDualStack
			} else {
				gce.stackType = NetworkStackIPV4
			}
			gotAddrs, err := gce.NodeAddresses(context.TODO(), types.NodeName(test.nodeName))
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

	alphaInstanceMap := make(map[string]*alpha.Instance)
	// n1 is instance with internal IPv6 address
	alphaInstance := &alpha.Instance{
		Name: "n1",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				AliasIpRanges: []*alpha.AliasIpRange{
					{IpCidrRange: "10.11.1.0/24"},
				},
				NetworkIP:   "10.1.1.1",
				StackType:   "IPV4_IPV6",
				Ipv6Address: "2001:2d00::1:0:0",
			},
		},
	}
	alphaInstanceMap["n1"] = alphaInstance

	// n2 is instance with external IPv6 address
	alphaInstance = &alpha.Instance{
		Name: "n2",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				AliasIpRanges: []*alpha.AliasIpRange{
					{IpCidrRange: "10.11.2.0/24"},
				},
				NetworkIP:      "10.1.1.2",
				StackType:      "IPV4_IPV6",
				Ipv6AccessType: "EXTERNAL",
				Ipv6AccessConfigs: []*alpha.AccessConfig{
					{ExternalIpv6: "2001:1900::2:0:0"},
				},
				AccessConfigs: []*alpha.AccessConfig{
					{NatIP: "20.1.1.2"},
				},
			},
		},
	}
	alphaInstanceMap["n2"] = alphaInstance

	// n4 is instance with invalid network interfaces
	alphaInstance = &alpha.Instance{
		Name: "n4",
		Zone: "us-central1-b",
	}
	alphaInstanceMap["n4"] = alphaInstance

	// n5 is a single stack instance
	alphaInstance = &alpha.Instance{
		Name: "n5",
		Zone: "us-central1-b",
		NetworkInterfaces: []*alpha.NetworkInterface{
			{
				AliasIpRanges: []*alpha.AliasIpRange{
					{IpCidrRange: "10.11.5.0/24"},
				},
				NetworkIP: "10.1.1.5",
				StackType: "IPV4",
				AccessConfigs: []*alpha.AccessConfig{
					{NatIP: "20.1.1.5"},
				},
			},
		},
	}
	alphaInstanceMap["n5"] = alphaInstance

	mockGCE := gce.c.(*cloud.MockGCE)
	mai := mockGCE.AlphaInstances().(*cloud.MockAlphaInstances)
	mai.GetHook = func(ctx context.Context, key *meta.Key, m *cloud.MockAlphaInstances) (bool, *alpha.Instance, error) {
		ret, ok := alphaInstanceMap[key.Name]
		if !ok {
			return true, nil, fmt.Errorf("alpha instance not found")
		}
		return true, ret, nil
	}

	testcases := []struct {
		name       string
		providerID string
		dualStack  bool
		wantErr    string
		wantCIDRs  []string
	}{
		{
			name:       "internal single stack instance",
			providerID: "gce://p1/us-central1-b/n1",
			dualStack:  true,
			wantCIDRs: []string{
				"10.11.1.0/24",
				"2001:2d00::1:0:0/112",
			},
		},
		{
			name:       "instance not found",
			providerID: "gce://p1/us-central1-b/x1",
			dualStack:  true,
			wantErr:    "alpha instance not found",
		},
		{
			name:       "internal single stack instance",
			providerID: "gce://p1/us-central1-b/n2",
			dualStack:  true,
			wantCIDRs: []string{
				"10.11.2.0/24",
				"2001:1900::2:0:0/112",
			},
		},
		{
			name:       "network interface not found",
			providerID: "gce://p1/us-central1-b/n4",
			dualStack:  true,
			wantErr:    "",
		},
		{
			name:       "single stack instance",
			providerID: "gce://p1/us-central1-b/n5",
			dualStack:  true,
			wantErr:    "IPV6 address not found",
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			if test.dualStack {
				gce.stackType = NetworkStackDualStack
			} else {
				gce.stackType = NetworkStackIPV4
			}
			gotCIDRs, err := gce.AliasRangesByProviderID(test.providerID)
			if err != nil && (test.wantErr == "" || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("gce.AliasRangesByProviderID. Want err: %v, got: %v", test.wantErr, err)
			} else if err == nil && test.wantErr != "" {
				t.Errorf("gce.AliasRangesByProviderID. Want err: %v, got: %v, gotCIDRs: %v", test.wantErr, err, gotCIDRs)
			}
			assert.Equal(t, test.wantCIDRs, gotCIDRs)
		})
	}
}
