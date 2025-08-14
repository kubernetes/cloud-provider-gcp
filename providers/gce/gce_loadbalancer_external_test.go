//go:build !providerless
// +build !providerless

/*
Copyright 2017 The Kubernetes Authors.

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	utilnet "k8s.io/utils/net"
)

const (
	eventMsgFirewallChange = "Firewall change required by security admin"
)

func TestEnsureStaticIP(t *testing.T) {
	t.Parallel()

	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	ipName := "some-static-ip"
	serviceName := "some-service"

	// First ensure call
	ip, existed, err := ensureStaticIP(gce, ipName, serviceName, gce.region, "", cloud.NetworkTierDefault)
	if err != nil || existed {
		t.Fatalf(`ensureStaticIP(%v, %v, %v, %v, "") = %v, %v, %v; want valid ip, false, nil`, gce, ipName, serviceName, gce.region, ip, existed, err)
	}

	// Second ensure call
	var ipPrime string
	ipPrime, existed, err = ensureStaticIP(gce, ipName, serviceName, gce.region, ip, cloud.NetworkTierDefault)
	if err != nil || !existed || ip != ipPrime {
		t.Fatalf(`ensureStaticIP(%v, %v, %v, %v, %v) = %v, %v, %v; want %v, true, nil`, gce, ipName, serviceName, gce.region, ip, ipPrime, existed, err, ip)
	}

	// Ensure call with different name
	ipName = "another-name-for-static-ip"
	ipPrime, existed, err = ensureStaticIP(gce, ipName, serviceName, gce.region, ip, cloud.NetworkTierDefault)
	if err != nil || !existed || ip != ipPrime {
		t.Fatalf(`ensureStaticIP(%v, %v, %v, %v, %v) = %v, %v, %v; want %v, true, nil`, gce, ipName, serviceName, gce.region, ip, ipPrime, existed, err, ip)
	}
}

func TestEnsureStaticIPWithTier(t *testing.T) {
	t.Parallel()

	s, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	serviceName := "some-service"

	for desc, tc := range map[string]struct {
		name     string
		netTier  cloud.NetworkTier
		expected string
	}{
		"Premium (default)": {
			name:     "foo-1",
			netTier:  cloud.NetworkTierPremium,
			expected: "PREMIUM",
		},
		"Standard": {
			name:     "foo-2",
			netTier:  cloud.NetworkTierStandard,
			expected: "STANDARD",
		},
	} {
		t.Run(desc, func(t *testing.T) {
			ip, existed, err := ensureStaticIP(s, tc.name, serviceName, s.region, "", tc.netTier)
			assert.NoError(t, err)
			assert.False(t, existed)
			assert.NotEqual(t, ip, "")
			// Get the Address from the fake address service and verify that the tier
			// is set correctly.
			Addr, err := s.GetRegionAddress(tc.name, s.region)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, Addr.NetworkTier)
		})
	}
}

func TestVerifyRequestedIP(t *testing.T) {
	t.Parallel()

	lbRef := "test-lb"

	for desc, tc := range map[string]struct {
		requestedIP     string
		fwdRuleIP       string
		netTier         cloud.NetworkTier
		addrList        []*compute.Address
		expectErr       bool
		expectUserOwned bool
	}{
		"requested IP exists": {
			requestedIP:     "1.1.1.1",
			netTier:         cloud.NetworkTierPremium,
			addrList:        []*compute.Address{{Name: "foo", Address: "1.1.1.1", NetworkTier: "PREMIUM"}},
			expectErr:       false,
			expectUserOwned: true,
		},
		"requested IP is not static, but is in use by the fwd rule": {
			requestedIP: "1.1.1.1",
			fwdRuleIP:   "1.1.1.1",
			netTier:     cloud.NetworkTierPremium,
			expectErr:   false,
		},
		"requested IP is not static and is not used by the fwd rule": {
			requestedIP: "1.1.1.1",
			fwdRuleIP:   "2.2.2.2",
			netTier:     cloud.NetworkTierPremium,
			expectErr:   true,
		},
		"no requested IP": {
			netTier:   cloud.NetworkTierPremium,
			expectErr: false,
		},
		"requested IP exists, but network tier does not match": {
			requestedIP: "1.1.1.1",
			netTier:     cloud.NetworkTierStandard,
			addrList:    []*compute.Address{{Name: "foo", Address: "1.1.1.1", NetworkTier: "PREMIUM"}},
			expectErr:   true,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			s, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)

			for _, addr := range tc.addrList {
				s.ReserveRegionAddress(addr, s.region)
			}
			isUserOwnedIP, err := verifyUserRequestedIP(s, s.region, tc.requestedIP, tc.fwdRuleIP, lbRef, tc.netTier)
			assert.Equal(t, tc.expectErr, err != nil, fmt.Sprintf("err: %v", err))
			assert.Equal(t, tc.expectUserOwned, isUserOwnedIP)
		})
	}
}

func TestMinMaxPortRange(t *testing.T) {
	for _, tc := range []struct {
		svcPorts      []v1.ServicePort
		expectedRange string
		expectError   bool
	}{
		{
			svcPorts: []v1.ServicePort{
				{Port: 1},
				{Port: 10},
				{Port: 100}},
			expectedRange: "1-100",
			expectError:   false,
		},
		{
			svcPorts: []v1.ServicePort{
				{Port: 10},
				{Port: 1},
				{Port: 50},
				{Port: 100},
				{Port: 90}},
			expectedRange: "1-100",
			expectError:   false,
		},
		{
			svcPorts: []v1.ServicePort{
				{Port: 10}},
			expectedRange: "10-10",
			expectError:   false,
		},
		{
			svcPorts: []v1.ServicePort{
				{Port: 100},
				{Port: 10}},
			expectedRange: "10-100",
			expectError:   false,
		},
		{
			svcPorts: []v1.ServicePort{
				{Port: 100},
				{Port: 50},
				{Port: 10}},
			expectedRange: "10-100",
			expectError:   false,
		},
		{
			svcPorts:      []v1.ServicePort{},
			expectedRange: "",
			expectError:   true,
		},
	} {
		portsRange, err := loadBalancerPortRange(tc.svcPorts)
		if portsRange != tc.expectedRange {
			t.Errorf("PortRange mismatch %v != %v", tc.expectedRange, portsRange)
		}
		if tc.expectError {
			assert.Error(t, err, "Should return an error, expected range "+tc.expectedRange)
		} else {
			assert.NoError(t, err, "Should not return an error, expected range "+tc.expectedRange)
		}
	}
}

func TestCreateForwardingRuleWithTier(t *testing.T) {
	t.Parallel()

	// Common variables among the tests.
	ports := []v1.ServicePort{{Name: "foo", Protocol: v1.ProtocolTCP, Port: int32(123)}}
	target := "test-target-pool"
	vals := DefaultTestClusterValues()
	serviceName := "foo-svc"

	baseLinkURL := "https://www.googleapis.com/compute/%v/projects/%v/regions/%v/forwardingRules/%v"

	for desc, tc := range map[string]struct {
		netTier      cloud.NetworkTier
		expectedRule *compute.ForwardingRule
	}{
		"Premium tier": {
			netTier: cloud.NetworkTierPremium,
			expectedRule: &compute.ForwardingRule{
				Name:        "lb-1",
				Description: `{"kubernetes.io/service-name":"foo-svc"}`,
				IPAddress:   "1.1.1.1",
				IPProtocol:  "TCP",
				PortRange:   "123-123",
				Target:      target,
				NetworkTier: "PREMIUM",
				SelfLink:    fmt.Sprintf(baseLinkURL, "v1", vals.ProjectID, vals.Region, "lb-1"),
			},
		},
		"Standard tier": {
			netTier: cloud.NetworkTierStandard,
			expectedRule: &compute.ForwardingRule{
				Name:        "lb-2",
				Description: `{"kubernetes.io/service-name":"foo-svc"}`,
				IPAddress:   "2.2.2.2",
				IPProtocol:  "TCP",
				PortRange:   "123-123",
				Target:      target,
				NetworkTier: "STANDARD",
				SelfLink:    fmt.Sprintf(baseLinkURL, "v1", vals.ProjectID, vals.Region, "lb-2"),
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			s, err := fakeGCECloud(vals)
			require.NoError(t, err)

			lbName := tc.expectedRule.Name
			ipAddr := tc.expectedRule.IPAddress

			err = createForwardingRule(s, lbName, serviceName, s.region, ipAddr, target, ports, tc.netTier, false)
			assert.NoError(t, err)

			Rule, err := s.GetRegionForwardingRule(lbName, s.region)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedRule, Rule)
		})
	}
}

func TestCreateForwardingRulePorts(t *testing.T) {
	t.Parallel()

	// Common variables among the tests.
	target := "test-target-pool"
	vals := DefaultTestClusterValues()
	serviceName := "foo-svc"
	ipAddr := "1.1.1.1"

	onePortUDP := []v1.ServicePort{
		{Name: "udp1", Protocol: v1.ProtocolUDP, Port: int32(80)},
	}

	basePortsTCP := []v1.ServicePort{
		{Name: "tcp1", Protocol: v1.ProtocolTCP, Port: int32(80)},
		{Name: "tcp2", Protocol: v1.ProtocolTCP, Port: int32(81)},
		{Name: "tcp3", Protocol: v1.ProtocolTCP, Port: int32(82)},
		{Name: "tcp4", Protocol: v1.ProtocolTCP, Port: int32(83)},
		{Name: "tcp5", Protocol: v1.ProtocolTCP, Port: int32(84)},
		{Name: "tcp6", Protocol: v1.ProtocolTCP, Port: int32(85)},
		{Name: "tcp7", Protocol: v1.ProtocolTCP, Port: int32(8080)},
	}

	fivePortsTCP := basePortsTCP[:5]
	sixPortsTCP := basePortsTCP[:6]
	wideRangePortsTCP := basePortsTCP[:]

	for _, tc := range []struct {
		desc                   string
		frName                 string
		ports                  []v1.ServicePort
		discretePortForwarding bool
		expectedPorts          []string
		expectedPortRange      string
	}{
		{
			desc:                   "Single Port, discretePorts enabled",
			frName:                 "fwd-rule1",
			ports:                  onePortUDP,
			discretePortForwarding: true,
			expectedPorts:          []string{"80"},
			expectedPortRange:      "",
		},
		{
			desc:                   "Individual Ports, discretePorts enabled",
			frName:                 "fwd-rule2",
			ports:                  fivePortsTCP,
			discretePortForwarding: true,
			expectedPorts:          []string{"80", "81", "82", "83", "84"},
			expectedPortRange:      "",
		},
		{
			desc:                   "PortRange, discretePorts enabled",
			frName:                 "fwd-rule3",
			ports:                  sixPortsTCP,
			discretePortForwarding: true,
			expectedPorts:          []string{},
			expectedPortRange:      "80-85",
		},
		{
			desc:                   "Wide PortRange, discretePorts enabled",
			frName:                 "fwd-rule4",
			ports:                  wideRangePortsTCP,
			discretePortForwarding: true,
			expectedPorts:          []string{},
			expectedPortRange:      "80-8080",
		},
		{
			desc:                   "Single Port (PortRange)",
			frName:                 "fwd-rule5",
			ports:                  onePortUDP,
			discretePortForwarding: false,
			expectedPorts:          []string{},
			expectedPortRange:      "80-80",
		},
		{
			desc:                   "5 Ports PortRange",
			frName:                 "fwd-rule6",
			ports:                  fivePortsTCP,
			discretePortForwarding: false,
			expectedPorts:          []string{},
			expectedPortRange:      "80-84",
		},
		{
			desc:                   "6 ports PortRange",
			frName:                 "fwd-rule7",
			ports:                  sixPortsTCP,
			discretePortForwarding: false,
			expectedPorts:          []string{},
			expectedPortRange:      "80-85",
		},
		{
			desc:                   "Wide PortRange",
			frName:                 "fwd-rule8",
			ports:                  wideRangePortsTCP,
			discretePortForwarding: false,
			expectedPorts:          []string{},
			expectedPortRange:      "80-8080",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gce, err := fakeGCECloud(vals)
			require.NoError(t, err)

			if tc.discretePortForwarding {
				gce.SetEnableDiscretePortForwarding(true)
			}

			frName := tc.frName
			ports := tc.ports

			err = createForwardingRule(gce, frName, serviceName, gce.region, ipAddr, target, ports, cloud.NetworkTierStandard, tc.discretePortForwarding)
			assert.NoError(t, err)

			fwdRule, err := gce.GetRegionForwardingRule(frName, gce.region)
			assert.NoError(t, err)

			assert.Equal(t, true, tc.expectedPortRange == fwdRule.PortRange)
			assert.Equal(t, true, equalStringSets(tc.expectedPorts, fwdRule.Ports))
		})
	}
}

func TestDeleteAddressWithWrongTier(t *testing.T) {
	t.Parallel()

	lbRef := "test-lb"

	s, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	for desc, tc := range map[string]struct {
		addrName     string
		netTier      cloud.NetworkTier
		addrList     []*compute.Address
		expectDelete bool
	}{
		"Network tiers (premium) match; do nothing": {
			addrName: "foo1",
			netTier:  cloud.NetworkTierPremium,
			addrList: []*compute.Address{{Name: "foo1", Address: "1.1.1.1", NetworkTier: "PREMIUM"}},
		},
		"Network tiers (standard) match; do nothing": {
			addrName: "foo2",
			netTier:  cloud.NetworkTierStandard,
			addrList: []*compute.Address{{Name: "foo2", Address: "1.1.1.2", NetworkTier: "STANDARD"}},
		},
		"Wrong network tier (standard); delete address": {
			addrName:     "foo3",
			netTier:      cloud.NetworkTierPremium,
			addrList:     []*compute.Address{{Name: "foo3", Address: "1.1.1.3", NetworkTier: "STANDARD"}},
			expectDelete: true,
		},
		"Wrong network tier (premium); delete address": {
			addrName:     "foo4",
			netTier:      cloud.NetworkTierStandard,
			addrList:     []*compute.Address{{Name: "foo4", Address: "1.1.1.4", NetworkTier: "PREMIUM"}},
			expectDelete: true,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			for _, addr := range tc.addrList {
				s.ReserveRegionAddress(addr, s.region)
			}

			// Sanity check to ensure we inject the right address.
			_, err = s.GetRegionAddress(tc.addrName, s.region)
			require.NoError(t, err)

			err = deleteAddressWithWrongTier(s, s.region, tc.addrName, lbRef, tc.netTier)
			assert.NoError(t, err)
			// Check whether the address still exists.
			_, err = s.GetRegionAddress(tc.addrName, s.region)
			if tc.expectDelete {
				assert.True(t, isNotFound(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func createExternalLoadBalancer(gce *Cloud, svc *v1.Service, nodeNames []string, clusterName, clusterID, zoneName string) (*v1.LoadBalancerStatus, error) {
	nodes, err := createAndInsertNodes(gce, nodeNames, zoneName)
	if err != nil {
		return nil, err
	}

	op := &loadBalancerSync{}
	op.actualForwardingRule = nil

	return gce.ensureExternalLoadBalancer(
		clusterName,
		clusterID,
		svc,
		op,
		nodes,
	)
}

func TestShouldNotRecreateLBWhenNetworkTiersMismatch(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	staticIP := "1.2.3.4"
	gce.ReserveRegionAddress(&compute.Address{Address: staticIP, Name: "foo", NetworkTier: cloud.NetworkTierStandard.ToGCEValue()}, vals.Region)

	for _, tc := range []struct {
		desc          string
		mutateSvc     func(service *v1.Service)
		expectNetTier string
		expectError   bool
	}{
		{
			desc: "initial LB config with standard network tier annotation",
			mutateSvc: func(service *v1.Service) {
				svc.Annotations[NetworkTierAnnotationKey] = string(NetworkTierAnnotationStandard)
			},
			expectNetTier: NetworkTierAnnotationStandard.ToGCEValue(),
		},
		{
			desc: "svc changed to empty network tier annotation",
			mutateSvc: func(service *v1.Service) {
				svc.Annotations = make(map[string]string)
			},
			expectNetTier: NetworkTierAnnotationStandard.ToGCEValue(),
		},
		{
			desc: "network tier annotation changed to premium",
			mutateSvc: func(service *v1.Service) {
				svc.Annotations[NetworkTierAnnotationKey] = string(NetworkTierAnnotationPremium)
			},
			expectNetTier: NetworkTierAnnotationPremium.ToGCEValue(),
		},
		{
			desc: " Network tiers annotation set to Standard and reserved static IP is specified",
			mutateSvc: func(service *v1.Service) {
				svc.Annotations[NetworkTierAnnotationKey] = string(NetworkTierAnnotationStandard)
				svc.Spec.LoadBalancerIP = staticIP

			},
			expectNetTier: NetworkTierAnnotationStandard.ToGCEValue(),
		},
		{
			desc: "svc changed to empty network tier annotation with static ip",
			mutateSvc: func(service *v1.Service) {
				svc.Annotations = make(map[string]string)
			},
			expectNetTier: NetworkTierAnnotationStandard.ToGCEValue(),
			expectError:   true,
		},
	} {
		tc.mutateSvc(svc)

		op := &loadBalancerSync{}
		op.actualForwardingRule = nil

		status, err := gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
		if tc.expectError {
			if err == nil {
				t.Errorf("for test case %q, expect errror != nil, but got %v", tc.desc, err)
			}
		} else {
			assert.NoError(t, err)
			assert.NotEmpty(t, status.Ingress)
		}

		lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
		fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
		assert.NoError(t, err)
		if fwdRule.NetworkTier != tc.expectNetTier {
			t.Fatalf("for test case %q, expect fwdRule.NetworkTier == %q, got %v ", tc.desc, tc.expectNetTier, fwdRule.NetworkTier)
		}
		assertExternalLbResources(t, gce, svc, vals, nodeNames)
	}
}

func TestEnsureExternalLoadBalancer(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createExternalLoadBalancer(gce, svc, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, NetLBFinalizerV1) {
		t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
	}

	assertExternalLbResources(t, gce, svc, vals, nodeNames)
}

func TestUpdateExternalLoadBalancer(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeName := "test-node-1"

	gce, err := fakeGCECloud((DefaultTestClusterValues()))
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createExternalLoadBalancer(gce, svc, []string{nodeName}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, NetLBFinalizerV1) {
		t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
	}

	newNodeName := "test-node-2"
	newNodes, err := createAndInsertNodes(gce, []string{nodeName, newNodeName}, vals.ZoneName)
	assert.NoError(t, err)

	// Add the new node, then check that it is properly added to the TargetPool
	err = gce.updateExternalLoadBalancer("", svc, newNodes)
	assert.NoError(t, err)

	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	pool, err := gce.GetTargetPool(lbName, gce.region)
	require.NoError(t, err)

	// TODO: when testify is updated to v1.2.0+, use ElementsMatch instead
	assert.Contains(
		t,
		pool.Instances,
		fmt.Sprintf("/zones/%s/instances/%s", vals.ZoneName, nodeName),
	)

	assert.Contains(
		t,
		pool.Instances,
		fmt.Sprintf("/zones/%s/instances/%s", vals.ZoneName, newNodeName),
	)

	newNodes, err = createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	assert.NoError(t, err)

	// Remove the new node by calling updateExternalLoadBalancer with a list
	// only containing the old node, and test that the TargetPool no longer
	// contains the new node.
	err = gce.updateExternalLoadBalancer(vals.ClusterName, svc, newNodes)
	assert.NoError(t, err)

	pool, err = gce.GetTargetPool(lbName, gce.region)
	require.NoError(t, err)

	assert.Equal(
		t,
		[]string{fmt.Sprintf("/zones/%s/instances/%s", vals.ZoneName, nodeName)},
		pool.Instances,
	)

	anotherNewNodeName := "test-node-3"
	newNodes, err = createAndInsertNodes(gce, []string{nodeName, newNodeName, anotherNewNodeName}, vals.ZoneName)
	assert.NoError(t, err)

	// delete one of the existing nodes, but include it in the list
	err = gce.DeleteInstance(gce.ProjectID(), vals.ZoneName, nodeName)
	require.NoError(t, err)

	// The update should ignore the reference to non-existent node "test-node-1", but update target pool with rest of the valid nodes.
	err = gce.updateExternalLoadBalancer(vals.ClusterName, svc, newNodes)
	assert.NoError(t, err)

	pool, err = gce.GetTargetPool(lbName, gce.region)
	require.NoError(t, err)

	namePrefix := fmt.Sprintf("/zones/%s/instances/", vals.ZoneName)
	assert.ElementsMatch(t, pool.Instances, []string{namePrefix + newNodeName, namePrefix + anotherNewNodeName})
}

func TestEnsureExternalLoadBalancerDeleted(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})

	require.NoError(t, err)
	_, err = createExternalLoadBalancer(gce, svc, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, NetLBFinalizerV1) {
		t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
	}

	err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	assert.NoError(t, err)

	assertExternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestLoadBalancerWrongTierResourceDeletion(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc.Annotations = map[string]string{NetworkTierAnnotationKey: "Premium"}

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	// cloud.NetworkTier defaults to Premium
	desiredTier, err := gce.getServiceNetworkTier(svc)
	require.NoError(t, err)
	assert.Equal(t, cloud.NetworkTierPremium, desiredTier)

	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	serviceName := types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}

	// create ForwardingRule and Address with the wrong tier
	err = createForwardingRule(
		gce,
		lbName,
		serviceName.String(),
		gce.region,
		"",
		gce.targetPoolURL(lbName),
		svc.Spec.Ports,
		cloud.NetworkTierStandard,
		false,
	)
	require.NoError(t, err)

	addressObj := &compute.Address{
		Name:        lbName,
		Description: serviceName.String(),
		NetworkTier: cloud.NetworkTierStandard.ToGCEValue(),
	}

	err = gce.ReserveRegionAddress(addressObj, gce.region)
	require.NoError(t, err)

	_, err = createExternalLoadBalancer(gce, svc, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NoError(t, err)

	// Expect forwarding rule tier to not be Standard
	tier, err := gce.getNetworkTierFromForwardingRule(lbName, gce.region)
	assert.NoError(t, err)
	assert.Equal(t, cloud.NetworkTierDefault.ToGCEValue(), tier)

	// Expect address to be deleted
	_, err = gce.GetRegionAddress(lbName, gce.region)
	assert.True(t, isNotFound(err))
}

func TestEnsureExternalLoadBalancerFailsIfInvalidNetworkTier(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	nodeNames := []string{"test-node-1"}

	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc.Annotations = map[string]string{NetworkTierAnnotationKey: wrongTier}

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	op := &loadBalancerSync{}
	op.actualForwardingRule = nil

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
	require.Error(t, err)
	assert.EqualError(t, err, errStrUnsupportedTier)
}

func TestEnsureExternalLoadBalancerFailsWithNoNodes(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	op := &loadBalancerSync{}
	op.actualForwardingRule = nil

	_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, []*v1.Node{})
	require.Error(t, err)
	assert.EqualError(t, err, errStrLbNoHosts)
}

func TestEnsureExternalLoadBalancerRBSAnnotation(t *testing.T) {
	t.Parallel()

	for desc, tc := range map[string]struct {
		annotations map[string]string
		wantError   *error
	}{
		"When RBS enabled": {
			annotations: map[string]string{RBSAnnotationKey: RBSEnabled},
			wantError:   &cloudprovider.ImplementedElsewhere,
		},
		"When RBS not enabled": {
			annotations: map[string]string{},
			wantError:   nil,
		},
		"When RBS annotation has wrong value": {
			annotations: map[string]string{RBSAnnotationKey: "WrongValue"},
			wantError:   nil,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()
			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud(%v) returned error %v, want nil", vals, err)
			}

			nodeNames := []string{"test-node-1"}
			nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes(_, %v, %v) returned error %v, want nil", nodeNames, vals.ZoneName, err)
			}

			svc := fakeLoadbalancerService("")
			svc.Annotations = tc.annotations

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)

			op := &loadBalancerSync{}
			op.actualForwardingRule = nil

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}

			err = gce.updateExternalLoadBalancer(vals.ClusterName, svc, nodes)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}

			err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}
		})
	}
}

func TestEnsureExternalLoadBalancerRBSFinalizer(t *testing.T) {
	t.Parallel()

	for desc, tc := range map[string]struct {
		finalizers []string
		wantError  *error
	}{
		"When has ELBRbsFinalizer V2": {
			finalizers: []string{NetLBFinalizerV2},
			wantError:  &cloudprovider.ImplementedElsewhere,
		},
		"When has ELBRbsFinalizer V3": {
			finalizers: []string{NetLBFinalizerV3},
			wantError:  &cloudprovider.ImplementedElsewhere,
		},
		"When has no finalizer": {
			finalizers: []string{},
			wantError:  nil,
		},
		"When has ELBFinalizer V1": {
			finalizers: []string{NetLBFinalizerV1},
			wantError:  nil,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()

			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud(%v) returned error %v, want nil", vals, err)
			}

			nodeNames := []string{"test-node-1"}
			nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes(_, %v, %v) returned error %v, want nil", nodeNames, vals.ZoneName, err)
			}

			svc := fakeLoadbalancerService("")
			svc.Finalizers = tc.finalizers

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)

			op := &loadBalancerSync{}
			op.actualForwardingRule = nil

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
				svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if !hasFinalizer(svc, NetLBFinalizerV1) {
					t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
				}
			}

			err = gce.updateExternalLoadBalancer(vals.ClusterName, svc, nodes)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
				svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if !hasFinalizer(svc, NetLBFinalizerV1) {
					t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
				}
			}

			err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
				svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
				require.NoError(t, err)
				assertExternalLbResourcesDeleted(t, gce, svc, vals, true)
			}
		})
	}
}

func TestDeleteExternalLoadBalancerWithFinalizer(t *testing.T) {
	t.Parallel()

	for desc, tc := range map[string]struct {
		finalizers []string
		wantError  *error
	}{
		"When has ELBRbsFinalizer V2": {
			finalizers: []string{NetLBFinalizerV2},
			wantError:  &cloudprovider.ImplementedElsewhere,
		},
		"When has ELBRbsFinalizer V3": {
			finalizers: []string{NetLBFinalizerV3},
			wantError:  &cloudprovider.ImplementedElsewhere,
		},
		"When has no finalizer": {
			finalizers: []string{},
			wantError:  nil,
		},
		"When has ELBFinalizer V1": {
			finalizers: []string{NetLBFinalizerV1},
			wantError:  nil,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()

			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud(%v) returned error %v, want nil", vals, err)
			}

			nodeNames := []string{"test-node-1"}
			nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes(_, %v, %v) returned error %v, want nil", nodeNames, vals.ZoneName, err)
			}

			svc := fakeLoadbalancerService("")

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)

			op := &loadBalancerSync{}
			op.actualForwardingRule = nil

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
			if err != nil {
				assert.NoError(t, err, "Should not return an error "+desc)
			}

			// use test finalizers for the deletion
			svc.Finalizers = tc.finalizers

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
			require.NoError(t, err)

			err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}
		})
	}
}

func TestEnsureExternalLoadBalancerExistingFwdRule(t *testing.T) {
	t.Parallel()

	for desc, tc := range map[string]struct {
		existingForwardingRule *compute.ForwardingRule
		wantError              *error
	}{
		"When has existingForwardingRule with backend service": {
			existingForwardingRule: &compute.ForwardingRule{
				BackendService: "exists",
			},
			wantError: &cloudprovider.ImplementedElsewhere,
		},
		"When has existingForwardingRule with empty backend service": {
			existingForwardingRule: &compute.ForwardingRule{
				BackendService: "",
			},
			wantError: nil,
		},
		"When has no existingForwardingRule": {
			existingForwardingRule: nil,
			wantError:              nil,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()

			gce, err := fakeGCECloud(vals)
			if err != nil {
				t.Fatalf("fakeGCECloud(%v) returned error %v, want nil", vals, err)
			}

			nodeNames := []string{"test-node-1"}
			nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			if err != nil {
				t.Fatalf("createAndInsertNodes(_, %v, %v) returned error %v, want nil", nodeNames, vals.ZoneName, err)
			}

			svc := fakeLoadbalancerService("")

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)

			op := &loadBalancerSync{}
			op.actualForwardingRule = tc.existingForwardingRule

			_, err = gce.ensureExternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, op, nodes)
			if tc.wantError != nil {
				assert.EqualError(t, err, (*tc.wantError).Error())
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}
		})
	}
}

func TestForwardingRuleNeedsUpdate(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	status, err := createExternalLoadBalancer(gce, svc, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NotNil(t, status)
	require.NoError(t, err)

	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	ipAddr := status.Ingress[0].IP

	lbIP := svc.Spec.LoadBalancerIP
	wrongPorts := []v1.ServicePort{svc.Spec.Ports[0]}
	wrongPorts[0].Port = wrongPorts[0].Port + 1

	wrongProtocolPorts := []v1.ServicePort{svc.Spec.Ports[0]}
	wrongProtocolPorts[0].Protocol = v1.ProtocolUDP

	for desc, tc := range map[string]struct {
		lbIP         string
		ports        []v1.ServicePort
		exists       bool
		needsUpdate  bool
		expectIPAddr string
		expectError  bool
	}{
		"When the loadBalancerIP does not equal the FwdRule IP address.": {
			lbIP:         "1.2.3.4",
			ports:        svc.Spec.Ports,
			exists:       true,
			needsUpdate:  true,
			expectIPAddr: ipAddr,
			expectError:  false,
		},
		"When loadBalancerPortRange returns an error.": {
			lbIP:         lbIP,
			ports:        []v1.ServicePort{},
			exists:       true,
			needsUpdate:  false,
			expectIPAddr: "",
			expectError:  true,
		},
		"When portRange not equals to the forwardingRule port range.": {
			lbIP:         lbIP,
			ports:        wrongPorts,
			exists:       true,
			needsUpdate:  true,
			expectIPAddr: ipAddr,
			expectError:  false,
		},
		"When the ports protocol does not equal the ForwardingRuel IP Protocol.": {
			lbIP:         lbIP,
			ports:        wrongProtocolPorts,
			exists:       true,
			needsUpdate:  true,
			expectIPAddr: ipAddr,
			expectError:  false,
		},
		"When basic workflow.": {
			lbIP:         lbIP,
			ports:        svc.Spec.Ports,
			exists:       true,
			needsUpdate:  false,
			expectIPAddr: ipAddr,
			expectError:  false,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			exists, needsUpdate, ipAddress, err := gce.forwardingRuleNeedsUpdate(lbName, vals.Region, tc.lbIP, tc.ports)
			assert.Equal(t, tc.exists, exists, "'exists' didn't return as expected "+desc)
			assert.Equal(t, tc.needsUpdate, needsUpdate, "'needsUpdate' didn't return as expected "+desc)
			assert.Equal(t, tc.expectIPAddr, ipAddress, "'ipAddress' didn't return as expected "+desc)
			if tc.expectError {
				assert.Error(t, err, "Should return an error "+desc)
			} else {
				assert.NoError(t, err, "Should not return an error "+desc)
			}
		})
	}
}

func TestCreateForwardingRuleNeedsUpdate(t *testing.T) {
	t.Parallel()

	// Common variables among the tests.
	target := "test-target-pool"
	vals := DefaultTestClusterValues()
	serviceName := "foo-svc"

	onePortTCP8080 := []v1.ServicePort{
		{Name: "tcp1", Protocol: v1.ProtocolTCP, Port: int32(8080)},
	}

	onePortUDP := []v1.ServicePort{
		{Name: "udp1", Protocol: v1.ProtocolUDP, Port: int32(80)},
	}

	basePortsTCP := []v1.ServicePort{
		{Name: "tcp1", Protocol: v1.ProtocolTCP, Port: int32(80)},
		{Name: "tcp2", Protocol: v1.ProtocolTCP, Port: int32(81)},
		{Name: "tcp3", Protocol: v1.ProtocolTCP, Port: int32(82)},
		{Name: "tcp4", Protocol: v1.ProtocolTCP, Port: int32(83)},
		{Name: "tcp5", Protocol: v1.ProtocolTCP, Port: int32(84)},
		{Name: "tcp6", Protocol: v1.ProtocolTCP, Port: int32(85)},
		{Name: "tcp7", Protocol: v1.ProtocolTCP, Port: int32(86)},
	}

	onePortTCP := basePortsTCP[:1]
	fivePortsTCP := basePortsTCP[:5]
	sixPortsTCP := basePortsTCP[:6]
	sevenPortsTCP := basePortsTCP[:]

	for _, tc := range []struct {
		desc                   string
		oldFwdRule             *compute.ForwardingRule
		oldPorts               []v1.ServicePort
		newlbIP                string
		newPorts               []v1.ServicePort
		discretePortForwarding bool
		needsUpdate            bool
		expectError            bool
	}{
		{
			desc: "different ip address on update",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule1",
				IPAddress: "1.1.1.1",
			},
			oldPorts:               onePortTCP,
			newlbIP:                "2.2.2.2",
			newPorts:               onePortTCP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "different protocol",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule2",
				IPAddress: "1.1.1.1",
			},
			oldPorts:               onePortTCP,
			newlbIP:                "1.1.1.1",
			newPorts:               onePortUDP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "same ports (PortRange)",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule3",
				IPAddress: "1.1.1.1",
			},
			// "80-80"
			oldPorts: onePortTCP,
			newlbIP:  "1.1.1.1",
			// "80-80"
			newPorts:               onePortTCP,
			discretePortForwarding: false,
			needsUpdate:            false,
			expectError:            false,
		},
		{
			desc: "same ports, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule4",
				IPAddress: "1.1.1.1",
			},
			// ["8080"]
			oldPorts: onePortTCP8080,
			newlbIP:  "1.1.1.1",
			// ["8080"]
			newPorts:               onePortTCP8080,
			discretePortForwarding: true,
			needsUpdate:            false,
			expectError:            false,
		},
		{
			desc: "same Port Range",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule5",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-85"
			newPorts:               sixPortsTCP,
			discretePortForwarding: false,
			needsUpdate:            false,
			expectError:            false,
		},
		{
			desc: "same Port Range, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule6",
				IPAddress: "1.1.1.1",
			},
			// "80-86"
			oldPorts: sevenPortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-86"
			newPorts:               sevenPortsTCP,
			discretePortForwarding: true,
			needsUpdate:            false,
			expectError:            false,
		},
		{
			desc: "port range mismatch",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule7",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-86"
			newPorts:               sevenPortsTCP,
			discretePortForwarding: false,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "port range mismatch, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule8",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-86"
			newPorts:               sevenPortsTCP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "ports mismatch (PortRange)",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule9",
				IPAddress: "1.1.1.1",
			},
			// "80-80"
			oldPorts: onePortTCP,
			newlbIP:  "1.1.1.1",
			// "80-84"
			newPorts:               fivePortsTCP,
			discretePortForwarding: false,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "ports mismatch, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule10",
				IPAddress: "1.1.1.1",
			},
			// ["80", "81", "82", "83", "84"]
			oldPorts: fivePortsTCP,
			newlbIP:  "1.1.1.1",
			// ["80"]
			newPorts:               onePortTCP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "PortRange to ports (PortRange)",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule11",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-84" five ports are still considered PortRange since discretePorts is disabled
			newPorts:               fivePortsTCP,
			discretePortForwarding: false,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "PortRange to ports discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule12",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// ["80", "81", "82", "83", "84"]
			newPorts:               fivePortsTCP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "PortRange to ports within existing port range discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule13",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// ["80", "85"]
			newPorts: []v1.ServicePort{
				{Name: "tcp1", Protocol: v1.ProtocolTCP, Port: int32(80)},
				{Name: "tcp2", Protocol: v1.ProtocolTCP, Port: int32(85)},
			},
			discretePortForwarding: true,
			// we don't want to unnecessarily recreate forwarding rules
			// when upgrading from port ranges to distinct ports, because recreating
			// forwarding rules is traffic impacting.
			needsUpdate: false,
			expectError: false,
		},
		{
			desc: "PortRange to ports, discretePorts enabled, port outside of PortRange",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule14",
				IPAddress: "1.1.1.1",
			},
			// "80-85"
			oldPorts: sixPortsTCP,
			newlbIP:  "1.1.1.1",
			// ["8080"]
			newPorts:               onePortTCP8080,
			discretePortForwarding: true,
			// Since port is outside of portrange we expect to recreate forwarding rule
			needsUpdate: true,
			expectError: false,
		},
		{
			desc: "ports (PortRange) to PortRange",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule15",
				IPAddress: "1.1.1.1",
			},
			// "80-84"
			oldPorts: fivePortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-85"
			newPorts:               sixPortsTCP,
			discretePortForwarding: false,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "ports to PortRange, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule16",
				IPAddress: "1.1.1.1",
			},
			// ["80", "81", "82", "83", "84"]
			oldPorts: fivePortsTCP,
			newlbIP:  "1.1.1.1",
			// "80-85"
			newPorts:               sixPortsTCP,
			discretePortForwarding: true,
			needsUpdate:            true,
			expectError:            false,
		},
		{
			desc: "update to empty ports, discretePorts enabled",
			oldFwdRule: &compute.ForwardingRule{
				Name:      "fwd-rule17",
				IPAddress: "1.1.1.1",
			},
			// ["80", "81", "82", "83", "84"]
			oldPorts:               fivePortsTCP,
			newlbIP:                "1.1.1.1",
			newPorts:               []v1.ServicePort{},
			discretePortForwarding: true,
			needsUpdate:            false,
			expectError:            true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gce, err := fakeGCECloud(vals)
			require.NoError(t, err)

			if tc.discretePortForwarding {
				gce.SetEnableDiscretePortForwarding(true)
			}

			frName := tc.oldFwdRule.Name
			ipAddr := tc.oldFwdRule.IPAddress
			ports := tc.oldPorts
			newlbIP := tc.newlbIP
			newPorts := tc.newPorts

			err = createForwardingRule(gce, frName, serviceName, gce.region, ipAddr, target, ports, cloud.NetworkTierStandard, tc.discretePortForwarding)
			assert.NoError(t, err)

			exists, needsUpdate, _, err := gce.forwardingRuleNeedsUpdate(frName, vals.Region, newlbIP, newPorts)
			assert.Equal(t, true, exists, "'exists' didn't return as expected "+tc.desc)
			assert.Equal(t, tc.needsUpdate, needsUpdate, "'needsUpdate' didn't return as expected "+tc.desc)
			if tc.expectError {
				assert.Error(t, err, "Should return an error "+tc.desc)
			} else {
				assert.NoError(t, err, "Should not return an error "+tc.desc)
			}
		})
	}
}

func TestTargetPoolAddsAndRemoveInstancesInBatches(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	addInstanceCalls := 0
	addInstanceHook := func(req *compute.TargetPoolsAddInstanceRequest) {
		addInstanceCalls++
	}
	removeInstanceCalls := 0
	removeInstanceHook := func(req *compute.TargetPoolsRemoveInstanceRequest) {
		removeInstanceCalls++
	}

	err = registerTargetPoolAddInstanceHook(gce, addInstanceHook)
	assert.NoError(t, err)
	err = registerTargetPoolRemoveInstanceHook(gce, removeInstanceHook)
	assert.NoError(t, err)

	svc := fakeLoadbalancerService("")
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	nodeName := "default-node"
	_, err = createExternalLoadBalancer(gce, svc, []string{nodeName}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	// Insert large number of nodes to test batching.
	additionalNodeNames := []string{}
	for i := 0; i < 2*maxInstancesPerTargetPoolUpdate+2; i++ {
		additionalNodeNames = append(additionalNodeNames, fmt.Sprintf("node-%d", i))
	}
	allNodes, err := createAndInsertNodes(gce, append([]string{nodeName}, additionalNodeNames...), vals.ZoneName)
	assert.NoError(t, err)
	err = gce.updateExternalLoadBalancer("", svc, allNodes)
	assert.NoError(t, err)

	assert.Equal(t, 3, addInstanceCalls)

	// Remove large number of nodes to test batching.
	allNodes, err = createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	assert.NoError(t, err)
	err = gce.updateExternalLoadBalancer("", svc, allNodes)
	assert.NoError(t, err)

	assert.Equal(t, 3, removeInstanceCalls)
}

func TestTargetPoolNeedsRecreation(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	serviceName := svc.ObjectMeta.Name
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	hostNames := nodeNames(nodes)
	hosts, err := gce.getInstancesByNames(hostNames)
	require.NoError(t, err)

	var instances []string
	for _, host := range hosts {
		instances = append(instances, host.makeComparableHostPath())
	}
	pool := &compute.TargetPool{
		Name:            lbName,
		Description:     fmt.Sprintf(`{"kubernetes.io/service-name":"%s"}`, serviceName),
		Instances:       instances,
		SessionAffinity: translateAffinityType(v1.ServiceAffinityNone),
	}
	err = gce.CreateTargetPool(pool, vals.Region)
	require.NoError(t, err)

	c := gce.c.(*cloud.MockGCE)
	c.MockTargetPools.GetHook = mock.GetTargetPoolInternalErrHook
	exists, needsRecreation, err := gce.targetPoolNeedsRecreation(lbName, vals.Region, v1.ServiceAffinityNone)
	assert.True(t, exists)
	assert.False(t, needsRecreation)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), errPrefixGetTargetPool))
	c.MockTargetPools.GetHook = nil

	exists, needsRecreation, err = gce.targetPoolNeedsRecreation(lbName, vals.Region, v1.ServiceAffinityClientIP)
	assert.True(t, exists)
	assert.True(t, needsRecreation)
	assert.NoError(t, err)

	exists, needsRecreation, err = gce.targetPoolNeedsRecreation(lbName, vals.Region, v1.ServiceAffinityNone)
	assert.True(t, exists)
	assert.False(t, needsRecreation)
	assert.NoError(t, err)
}

func TestFirewallNeedsUpdate(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	svc.Spec.Ports = []v1.ServicePort{
		{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
		{Name: "port2", Protocol: v1.ProtocolTCP, Port: int32(81), TargetPort: intstr.FromInt(81)},
		{Name: "port3", Protocol: v1.ProtocolTCP, Port: int32(82), TargetPort: intstr.FromInt(82)},
		{Name: "port4", Protocol: v1.ProtocolTCP, Port: int32(84), TargetPort: intstr.FromInt(84)},
		{Name: "port5", Protocol: v1.ProtocolTCP, Port: int32(85), TargetPort: intstr.FromInt(85)},
		{Name: "port6", Protocol: v1.ProtocolTCP, Port: int32(86), TargetPort: intstr.FromInt(86)},
		{Name: "port7", Protocol: v1.ProtocolTCP, Port: int32(88), TargetPort: intstr.FromInt(87)},
	}

	status, err := createExternalLoadBalancer(gce, svc, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NotNil(t, status)
	require.NoError(t, err)
	svcName := "/" + svc.ObjectMeta.Name

	ipAddr := status.Ingress[0].IP
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	ipnet, err := utilnet.ParseIPNets("0.0.0.0/0")
	require.NoError(t, err)

	wrongIpnet, err := utilnet.ParseIPNets("1.0.0.0/10")
	require.NoError(t, err)

	fw, err := gce.GetFirewall(MakeFirewallName(lbName))
	require.NoError(t, err)

	for desc, tc := range map[string]struct {
		lbName       string
		ipAddr       string
		ports        []v1.ServicePort
		ipnet        utilnet.IPNetSet
		fwIPProtocol string
		getHook      func(context.Context, *meta.Key, *cloud.MockFirewalls, ...cloud.Option) (bool, *compute.Firewall, error)
		sourceRange  string
		exists       bool
		needsUpdate  bool
		hasErr       bool
	}{
		"When response is a Non-400 HTTP error.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      mock.GetFirewallsUnauthorizedErrHook,
			sourceRange:  fw.SourceRanges[0],
			exists:       false,
			needsUpdate:  false,
			hasErr:       true,
		},
		"When given a wrong description.": {
			lbName:       lbName,
			ipAddr:       "",
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When IPProtocol doesn't match.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "usps",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When the ports don't match.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        []v1.ServicePort{{Protocol: v1.ProtocolTCP, Port: int32(666)}},
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When parseIPNets returns an error.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  "badSourceRange",
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When the source ranges are not equal.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        wrongIpnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When the destination ranges are not equal.": {
			lbName:       lbName,
			ipAddr:       "8.8.8.8",
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  true,
			hasErr:       false,
		},
		"When basic flow without exceptions.": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook:      nil,
			sourceRange:  fw.SourceRanges[0],
			exists:       true,
			needsUpdate:  false,
			hasErr:       false,
		},
		"Backward compatible with previous firewall setup with enumerated ports": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook: func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, *compute.Firewall, error) {
				obj, ok := m.Objects[*key]
				if !ok {
					return false, nil, nil
				}
				fw, err := copyFirewallObj(obj.Obj.(*compute.Firewall))
				if err != nil {
					return true, nil, err
				}
				// enumerate the service ports in the firewall rule
				fw.Allowed[0].Ports = []string{"80", "81", "82", "84", "85", "86", "88"}
				return true, fw, nil
			},
			sourceRange: fw.SourceRanges[0],
			exists:      true,
			needsUpdate: false,
			hasErr:      false,
		},
		"need to update previous firewall setup with enumerated ports ": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook: func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, *compute.Firewall, error) {
				obj, ok := m.Objects[*key]
				if !ok {
					return false, nil, nil
				}
				fw, err := copyFirewallObj(obj.Obj.(*compute.Firewall))
				if err != nil {
					return true, nil, err
				}
				// enumerate the service ports in the firewall rule
				fw.Allowed[0].Ports = []string{"80", "81", "82", "84", "85", "86"}
				return true, fw, nil
			},
			sourceRange: fw.SourceRanges[0],
			exists:      true,
			needsUpdate: true,
			hasErr:      false,
		},
		"need to update port-ranges ": {
			lbName:       lbName,
			ipAddr:       ipAddr,
			ports:        svc.Spec.Ports,
			ipnet:        ipnet,
			fwIPProtocol: "tcp",
			getHook: func(ctx context.Context, key *meta.Key, m *cloud.MockFirewalls, options ...cloud.Option) (bool, *compute.Firewall, error) {
				obj, ok := m.Objects[*key]
				if !ok {
					return false, nil, nil
				}
				fw, err := copyFirewallObj(obj.Obj.(*compute.Firewall))
				if err != nil {
					return true, nil, err
				}
				// enumerate the service ports in the firewall rule
				fw.Allowed[0].Ports = []string{"80-82", "86"}
				return true, fw, nil
			},
			sourceRange: fw.SourceRanges[0],
			exists:      true,
			needsUpdate: true,
			hasErr:      false,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			fw, err = gce.GetFirewall(MakeFirewallName(tc.lbName))
			fw.Allowed[0].IPProtocol = tc.fwIPProtocol
			fw, err = gce.GetFirewall(MakeFirewallName(tc.lbName))
			require.Equal(t, fw.Allowed[0].IPProtocol, tc.fwIPProtocol)

			trueSourceRange := fw.SourceRanges[0]
			fw.SourceRanges[0] = tc.sourceRange
			fw, err = gce.GetFirewall(MakeFirewallName(lbName))
			require.Equal(t, fw.SourceRanges[0], tc.sourceRange)
			require.Equal(t, fw.DestinationRanges[0], status.Ingress[0].IP)

			c := gce.c.(*cloud.MockGCE)
			c.MockFirewalls.GetHook = tc.getHook

			exists, needsUpdate, err := gce.firewallNeedsUpdate(
				tc.lbName,
				svcName,
				tc.ipAddr,
				tc.ports,
				tc.ipnet)
			assert.Equal(t, tc.exists, exists, "'exists' didn't return as expected "+desc)
			assert.Equal(t, tc.needsUpdate, needsUpdate, "'needsUpdate' didn't return as expected "+desc)
			if tc.hasErr {
				assert.Error(t, err, "Should returns an error "+desc)
			} else {
				assert.NoError(t, err, "Should not returns an error "+desc)
			}

			c.MockFirewalls.GetHook = nil

			fw.Allowed[0].IPProtocol = "tcp"
			fw.SourceRanges[0] = trueSourceRange
			fw, err = gce.GetFirewall(MakeFirewallName(tc.lbName))
			require.NoError(t, err)
			require.Equal(t, fw.Allowed[0].IPProtocol, "tcp")
			require.Equal(t, fw.SourceRanges[0], trueSourceRange)

		})
	}
}

func TestDeleteWrongNetworkTieredResourcesSucceedsWhenNotFound(t *testing.T) {
	t.Parallel()

	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	assert.Nil(t, gce.deleteWrongNetworkTieredResources("Wrong_LB_Name", "", cloud.NetworkTier("")))
}

func TestEnsureTargetPoolAndHealthCheck(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	op := &loadBalancerSync{}
	op.actualForwardingRule = nil

	status, err := gce.ensureExternalLoadBalancer(
		vals.ClusterName,
		vals.ClusterID,
		svc,
		op,
		nodes,
	)
	require.NotNil(t, status)
	require.NoError(t, err)

	hostNames := nodeNames(nodes)
	hosts, err := gce.getInstancesByNames(hostNames)
	require.NoError(t, err)
	clusterID := vals.ClusterID

	ipAddr := status.Ingress[0].IP
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	region := vals.Region

	hcToCreate := makeHTTPHealthCheck(MakeNodesHealthCheckName(clusterID), GetNodesHealthCheckPath(), GetNodesHealthCheckPort())
	hcToDelete := makeHTTPHealthCheck(MakeNodesHealthCheckName(clusterID), GetNodesHealthCheckPath(), GetNodesHealthCheckPort())

	// Apply a tag on the target pool. By verifying the change of the tag, target pool update can be ensured.
	tag := "A Tag"
	pool, err := gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	pool.CreationTimestamp = tag
	pool, err = gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	require.Equal(t, tag, pool.CreationTimestamp)
	err = gce.ensureTargetPoolAndHealthCheck(true, true, svc, lbName, clusterID, ipAddr, hosts, hcToCreate, hcToDelete)
	assert.NoError(t, err)
	pool, err = gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	assert.NotEqual(t, pool.CreationTimestamp, tag)

	pool, err = gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	assert.Equal(t, 1, len(pool.Instances))
	var manyNodeName [maxTargetPoolCreateInstances + 1]string
	for i := 0; i < maxTargetPoolCreateInstances+1; i++ {
		manyNodeName[i] = fmt.Sprintf("testnode_%d", i)
	}
	manyNodes, err := createAndInsertNodes(gce, manyNodeName[:], vals.ZoneName)
	require.NoError(t, err)
	manyHostNames := nodeNames(manyNodes)
	manyHosts, err := gce.getInstancesByNames(manyHostNames)
	require.NoError(t, err)
	err = gce.ensureTargetPoolAndHealthCheck(true, true, svc, lbName, clusterID, ipAddr, manyHosts, hcToCreate, hcToDelete)
	assert.NoError(t, err)

	pool, err = gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	assert.Equal(t, maxTargetPoolCreateInstances+1, len(pool.Instances))

	err = gce.ensureTargetPoolAndHealthCheck(true, false, svc, lbName, clusterID, ipAddr, hosts, hcToCreate, hcToDelete)
	assert.NoError(t, err)
	pool, err = gce.GetTargetPool(lbName, region)
	require.NoError(t, err)
	assert.Equal(t, 1, len(pool.Instances))
}

func TestCreateAndUpdateFirewallSucceedsOnXPN(t *testing.T) {
	t.Parallel()

	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	vals := DefaultTestClusterValues()

	c := gce.c.(*cloud.MockGCE)
	c.MockFirewalls.InsertHook = mock.InsertFirewallsUnauthorizedErrHook
	c.MockFirewalls.PatchHook = mock.UpdateFirewallsUnauthorizedErrHook
	gce.onXPN = true
	require.True(t, gce.OnXPN())

	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder

	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	hostNames := nodeNames(nodes)
	hosts, err := gce.getInstancesByNames(hostNames)
	require.NoError(t, err)
	ipnet, err := utilnet.ParseIPNets("10.0.0.0/20")
	require.NoError(t, err)
	gce.createFirewall(
		svc,
		gce.GetLoadBalancerName(context.TODO(), "", svc),
		"10.0.0.1",
		"A sad little firewall",
		ipnet,
		svc.Spec.Ports,
		hosts)
	require.NoError(t, err)

	msg := fmt.Sprintf("%s %s %s", v1.EventTypeNormal, eventReasonManualChange, eventMsgFirewallChange)
	checkEvent(t, recorder, msg, true)

	gce.updateFirewall(
		svc,
		gce.GetLoadBalancerName(context.TODO(), "", svc),
		"A sad little firewall",
		"10.0.0.1",
		ipnet,
		svc.Spec.Ports,
		hosts)
	require.NoError(t, err)

	msg = fmt.Sprintf("%s %s %s", v1.EventTypeNormal, eventReasonManualChange, eventMsgFirewallChange)
	checkEvent(t, recorder, msg, true)
}

func TestEnsureExternalLoadBalancerDeletedSucceedsOnXPN(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)

	svc := fakeLoadbalancerService("")

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = createExternalLoadBalancer(gce, svc, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NoError(t, err)

	c := gce.c.(*cloud.MockGCE)
	c.MockFirewalls.DeleteHook = mock.DeleteFirewallsUnauthorizedErrHook
	gce.onXPN = true
	require.True(t, gce.OnXPN())

	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder

	err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	require.NoError(t, err)

	msg := fmt.Sprintf("%s %s %s", v1.EventTypeNormal, eventReasonManualChange, eventMsgFirewallChange)
	checkEvent(t, recorder, msg, true)
}

type EnsureELBParams struct {
	clusterName     string
	clusterID       string
	service         *v1.Service
	existingFwdRule *compute.ForwardingRule
	nodes           []*v1.Node
}

// newEnsureELBParams is the constructor of EnsureELBParams.
func newEnsureELBParams(nodes []*v1.Node, svc *v1.Service) *EnsureELBParams {
	vals := DefaultTestClusterValues()
	return &EnsureELBParams{
		vals.ClusterName,
		vals.ClusterID,
		svc,
		nil,
		nodes,
	}
}

// TestEnsureExternalLoadBalancerErrors tests the function
// ensureExternalLoadBalancer, making sure the system won't panic when
// exceptions raised by gce.
func TestEnsureExternalLoadBalancerErrors(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	var params *EnsureELBParams

	for desc, tc := range map[string]struct {
		adjustParams func(*EnsureELBParams)
		injectMock   func(*cloud.MockGCE)
	}{
		"No hosts provided": {
			adjustParams: func(params *EnsureELBParams) {
				params.nodes = []*v1.Node{}
			},
		},
		"Invalid node provided": {
			adjustParams: func(params *EnsureELBParams) {
				params.nodes = []*v1.Node{{}}
			},
		},
		"Get forwarding rules failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockForwardingRules.GetHook = mock.GetForwardingRulesInternalErrHook
			},
		},
		"Get addresses failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockAddresses.GetHook = mock.GetAddressesInternalErrHook
			},
		},
		"Bad load balancer source range provided": {
			adjustParams: func(params *EnsureELBParams) {
				params.service.Spec.LoadBalancerSourceRanges = []string{"BadSourceRange"}
			},
		},
		"Get firewall failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockFirewalls.GetHook = mock.GetFirewallsUnauthorizedErrHook
			},
		},
		"Create firewall failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockFirewalls.InsertHook = mock.InsertFirewallsUnauthorizedErrHook
			},
		},
		"Get target pool failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockTargetPools.GetHook = mock.GetTargetPoolInternalErrHook
			},
		},
		"Get HTTP health checks failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockHttpHealthChecks.GetHook = mock.GetHTTPHealthChecksInternalErrHook
			},
		},
		"Create target pools failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockTargetPools.InsertHook = mock.InsertTargetPoolsInternalErrHook
			},
		},
		"Create forwarding rules failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockForwardingRules.InsertHook = mock.InsertForwardingRulesInternalErrHook
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			gce, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)
			nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
			require.NoError(t, err)
			svc := fakeLoadbalancerService("")

			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)

			params = newEnsureELBParams(nodes, svc)
			if tc.adjustParams != nil {
				tc.adjustParams(params)
			}
			if tc.injectMock != nil {
				tc.injectMock(gce.c.(*cloud.MockGCE))
			}

			op := &loadBalancerSync{}
			op.actualForwardingRule = params.existingFwdRule

			status, err := gce.ensureExternalLoadBalancer(
				params.clusterName,
				params.clusterID,
				params.service,
				op,
				params.nodes,
			)
			assert.Error(t, err, "Should return an error when "+desc)
			assert.Nil(t, status, "Should not return a status when "+desc)
		})
	}
}

func TestExternalLoadBalancerEnsureHttpHealthCheck(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc      string
		modifier  func(*compute.HttpHealthCheck) *compute.HttpHealthCheck
		wantEqual bool
	}{
		{"should ensure HC", func(_ *compute.HttpHealthCheck) *compute.HttpHealthCheck { return nil }, false},
		{
			"should reconcile HC interval",
			func(hc *compute.HttpHealthCheck) *compute.HttpHealthCheck {
				hc.CheckIntervalSec = gceHcCheckIntervalSeconds - 1
				return hc
			},
			false,
		},
		{
			"should allow HC to be configurable to bigger intervals",
			func(hc *compute.HttpHealthCheck) *compute.HttpHealthCheck {
				hc.CheckIntervalSec = gceHcCheckIntervalSeconds * 10
				return hc
			},
			true,
		},
		{
			"should allow HC to accept bigger intervals while applying default value to small thresholds",
			func(hc *compute.HttpHealthCheck) *compute.HttpHealthCheck {
				hc.CheckIntervalSec = gceHcCheckIntervalSeconds * 10
				hc.UnhealthyThreshold = gceHcUnhealthyThreshold - 1
				return hc
			},
			false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {

			gce, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)
			c := gce.c.(*cloud.MockGCE)
			c.MockHttpHealthChecks.UpdateHook = func(ctx context.Context, key *meta.Key, obj *compute.HttpHealthCheck, m *cloud.MockHttpHealthChecks, options ...cloud.Option) error {
				m.Objects[*key] = &cloud.MockHttpHealthChecksObj{Obj: obj}
				return nil
			}

			hcName, hcPath, hcPort := "test-hc", "/healthz", int32(12345)
			existingHC := makeHTTPHealthCheck(hcName, hcPath, hcPort)
			existingHC = tc.modifier(existingHC)
			if existingHC != nil {
				if err := gce.CreateHTTPHealthCheck(existingHC); err != nil {
					t.Fatalf("gce.CreateHttpHealthCheck(%#v) = %v; want err = nil", existingHC, err)
				}
			}
			if _, err := gce.ensureHTTPHealthCheck(hcName, hcPath, hcPort); err != nil {
				t.Fatalf("gce.ensureHttpHealthCheck(%q, %q, %v) = _, %d; want err = nil", hcName, hcPath, hcPort, err)
			}
			if hc, err := gce.GetHTTPHealthCheck(hcName); err != nil {
				t.Fatalf("gce.GetHttpHealthCheck(%q) = _, %d; want err = nil", hcName, err)
			} else {
				if tc.wantEqual {
					assert.Equal(t, hc, existingHC)
				} else {
					assert.NotEqual(t, hc, existingHC)
				}
			}
		})
	}

}

func TestMergeHttpHealthChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc                   string
		checkIntervalSec       int64
		timeoutSec             int64
		healthyThreshold       int64
		unhealthyThreshold     int64
		wantCheckIntervalSec   int64
		wantTimeoutSec         int64
		wantHealthyThreshold   int64
		wantUnhealthyThreshold int64
	}{
		{"unchanged", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"interval - too small - should reconcile", gceHcCheckIntervalSeconds - 1, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"timeout - too small - should reconcile", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds - 1, gceHcHealthyThreshold, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"healthy threshold - too small - should reconcile", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold - 1, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"unhealthy threshold - too small - should reconcile", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold - 1, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"interval - user configured - should keep", gceHcCheckIntervalSeconds + 1, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds + 1, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"timeout - user configured - should keep", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds + 1, gceHcHealthyThreshold, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds + 1, gceHcHealthyThreshold, gceHcUnhealthyThreshold},
		{"healthy threshold - user configured - should keep", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold + 1, gceHcUnhealthyThreshold, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold + 1, gceHcUnhealthyThreshold},
		{"unhealthy threshold - user configured - should keep", gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold + 1, gceHcCheckIntervalSeconds, gceHcTimeoutSeconds, gceHcHealthyThreshold, gceHcUnhealthyThreshold + 1},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			wantHC := makeHTTPHealthCheck("hc", "/", 12345)
			hc := &compute.HttpHealthCheck{
				CheckIntervalSec:   tc.checkIntervalSec,
				TimeoutSec:         tc.timeoutSec,
				HealthyThreshold:   tc.healthyThreshold,
				UnhealthyThreshold: tc.unhealthyThreshold,
			}
			mergeHTTPHealthChecks(hc, wantHC)
			if wantHC.CheckIntervalSec != tc.wantCheckIntervalSec {
				t.Errorf("wantHC.CheckIntervalSec = %d; want %d", wantHC.CheckIntervalSec, tc.checkIntervalSec)
			}
			if wantHC.TimeoutSec != tc.wantTimeoutSec {
				t.Errorf("wantHC.TimeoutSec = %d; want %d", wantHC.TimeoutSec, tc.timeoutSec)
			}
			if wantHC.HealthyThreshold != tc.wantHealthyThreshold {
				t.Errorf("wantHC.HealthyThreshold = %d; want %d", wantHC.HealthyThreshold, tc.healthyThreshold)
			}
			if wantHC.UnhealthyThreshold != tc.wantUnhealthyThreshold {
				t.Errorf("wantHC.UnhealthyThreshold = %d; want %d", wantHC.UnhealthyThreshold, tc.unhealthyThreshold)
			}
		})
	}
}

func TestNeedToUpdateHttpHealthChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc        string
		modifier    func(*compute.HttpHealthCheck)
		wantChanged bool
	}{
		{"unchanged", nil, false},
		{"desc does not match", func(hc *compute.HttpHealthCheck) { hc.Description = "bad-desc" }, true},
		{"port does not match", func(hc *compute.HttpHealthCheck) { hc.Port = 54321 }, true},
		{"requestPath does not match", func(hc *compute.HttpHealthCheck) { hc.RequestPath = "/anotherone" }, true},
		{"interval needs update", func(hc *compute.HttpHealthCheck) { hc.CheckIntervalSec = gceHcCheckIntervalSeconds - 1 }, true},
		{"timeout needs update", func(hc *compute.HttpHealthCheck) { hc.TimeoutSec = gceHcTimeoutSeconds - 1 }, true},
		{"healthy threshold needs update", func(hc *compute.HttpHealthCheck) { hc.HealthyThreshold = gceHcHealthyThreshold - 1 }, true},
		{"unhealthy threshold needs update", func(hc *compute.HttpHealthCheck) { hc.UnhealthyThreshold = gceHcUnhealthyThreshold - 1 }, true},
		{"interval does not need update", func(hc *compute.HttpHealthCheck) { hc.CheckIntervalSec = gceHcCheckIntervalSeconds + 1 }, false},
		{"timeout does not need update", func(hc *compute.HttpHealthCheck) { hc.TimeoutSec = gceHcTimeoutSeconds + 1 }, false},
		{"healthy threshold does not need update", func(hc *compute.HttpHealthCheck) { hc.HealthyThreshold = gceHcHealthyThreshold + 1 }, false},
		{"unhealthy threshold does not need update", func(hc *compute.HttpHealthCheck) { hc.UnhealthyThreshold = gceHcUnhealthyThreshold + 1 }, false},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			hc := makeHTTPHealthCheck("hc", "/", 12345)
			wantHC := makeHTTPHealthCheck("hc", "/", 12345)
			if tc.modifier != nil {
				tc.modifier(hc)
			}
			if gotChanged := needToUpdateHTTPHealthChecks(hc, wantHC); gotChanged != tc.wantChanged {
				t.Errorf("needToUpdateHTTPHealthChecks(%#v, %#v) = %t; want changed = %t", hc, wantHC, gotChanged, tc.wantChanged)
			}
		})
	}
}

func TestFirewallObject(t *testing.T) {
	t.Parallel()
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	gce.nodeTags = []string{"node-tags"}
	require.NoError(t, err)
	dstIP := "10.0.0.1"
	srcRanges := []string{"10.10.0.0/24", "10.20.0.0/24"}
	sourceRanges, _ := utilnet.ParseIPNets(srcRanges...)
	fwName := "test-fw"
	fwDesc := "test-desc"
	baseFw := compute.Firewall{
		Name:         fwName,
		Description:  fwDesc,
		Network:      gce.networkURL,
		SourceRanges: []string{},
		TargetTags:   gce.nodeTags,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"80"},
			},
		},
	}

	for _, tc := range []struct {
		desc             string
		sourceRanges     utilnet.IPNetSet
		destinationIP    string
		svcPorts         []v1.ServicePort
		expectedFirewall func(fw compute.Firewall) compute.Firewall
	}{
		{
			desc:         "empty source ranges",
			sourceRanges: utilnet.IPNetSet{},
			svcPorts: []v1.ServicePort{
				{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
			},
			expectedFirewall: func(fw compute.Firewall) compute.Firewall {
				return fw
			},
		},
		{
			desc:         "has source ranges",
			sourceRanges: sourceRanges,
			svcPorts: []v1.ServicePort{
				{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
			},
			expectedFirewall: func(fw compute.Firewall) compute.Firewall {
				fw.SourceRanges = srcRanges
				return fw
			},
		},
		{
			desc:          "has destination IP",
			sourceRanges:  utilnet.IPNetSet{},
			destinationIP: dstIP,
			svcPorts: []v1.ServicePort{
				{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
			},
			expectedFirewall: func(fw compute.Firewall) compute.Firewall {
				fw.DestinationRanges = []string{dstIP}
				return fw
			},
		},
		{
			desc:         "has multiple ports",
			sourceRanges: sourceRanges,
			svcPorts: []v1.ServicePort{
				{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
				{Name: "port2", Protocol: v1.ProtocolTCP, Port: int32(82), TargetPort: intstr.FromInt(82)},
				{Name: "port3", Protocol: v1.ProtocolTCP, Port: int32(84), TargetPort: intstr.FromInt(84)},
			},
			expectedFirewall: func(fw compute.Firewall) compute.Firewall {
				fw.Allowed = []*compute.FirewallAllowed{
					{
						IPProtocol: "tcp",
						Ports:      []string{"80", "82", "84"},
					},
				}
				fw.SourceRanges = srcRanges
				return fw
			},
		},
		{
			desc:         "has multiple ports",
			sourceRanges: sourceRanges,
			svcPorts: []v1.ServicePort{
				{Name: "port1", Protocol: v1.ProtocolTCP, Port: int32(80), TargetPort: intstr.FromInt(80)},
				{Name: "port2", Protocol: v1.ProtocolTCP, Port: int32(81), TargetPort: intstr.FromInt(81)},
				{Name: "port3", Protocol: v1.ProtocolTCP, Port: int32(82), TargetPort: intstr.FromInt(82)},
				{Name: "port4", Protocol: v1.ProtocolTCP, Port: int32(84), TargetPort: intstr.FromInt(84)},
				{Name: "port5", Protocol: v1.ProtocolTCP, Port: int32(85), TargetPort: intstr.FromInt(85)},
				{Name: "port6", Protocol: v1.ProtocolTCP, Port: int32(86), TargetPort: intstr.FromInt(86)},
				{Name: "port7", Protocol: v1.ProtocolTCP, Port: int32(88), TargetPort: intstr.FromInt(87)},
			},
			expectedFirewall: func(fw compute.Firewall) compute.Firewall {
				fw.Allowed = []*compute.FirewallAllowed{
					{
						IPProtocol: "tcp",
						Ports:      []string{"80-82", "84-86", "88"},
					},
				}
				fw.SourceRanges = srcRanges
				return fw
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ret, err := gce.firewallObject(fwName, fwDesc, tc.destinationIP, tc.sourceRanges, tc.svcPorts, nil)
			require.NoError(t, err)
			expectedFirewall := tc.expectedFirewall(baseFw)
			retSrcRanges := sets.NewString(ret.SourceRanges...)
			expectSrcRanges := sets.NewString(expectedFirewall.SourceRanges...)
			if !expectSrcRanges.Equal(retSrcRanges) {
				t.Errorf("expect firewall source ranges to be %v, but got %v", expectSrcRanges, retSrcRanges)
			}
			ret.SourceRanges = nil
			expectedFirewall.SourceRanges = nil
			if !reflect.DeepEqual(*ret, expectedFirewall) {
				t.Errorf("expect firewall to be %+v, but got %+v", expectedFirewall, ret)
			}
		})
	}
}

func copyFirewallObj(firewall *compute.Firewall) (*compute.Firewall, error) {
	// make a copy of the original obj via json marshal and unmarshal
	jsonObj, err := firewall.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var fw compute.Firewall
	err = json.Unmarshal(jsonObj, &fw)
	if err != nil {
		return nil, err
	}
	return &fw, nil
}

func TestEnsureExternalLoadBalancerClass(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	for _, tc := range []struct {
		desc              string
		loadBalancerClass string
		shouldProcess     bool
	}{
		{
			desc:              "Custom loadBalancerClass should not process",
			loadBalancerClass: "customLBClass",
			shouldProcess:     false,
		},
		{
			desc:              "Use legacy ILB loadBalancerClass",
			loadBalancerClass: LegacyRegionalInternalLoadBalancerClass,
			shouldProcess:     false,
		},
		{
			desc:              "Use legacy NetLB loadBalancerClass",
			loadBalancerClass: LegacyRegionalExternalLoadBalancerClass,
			shouldProcess:     true,
		},
		{
			desc:              "Unset loadBalancerClass",
			loadBalancerClass: "",
			shouldProcess:     true,
		},
	} {
		gce, err := fakeGCECloud(vals)
		assert.NoError(t, err)
		recorder := record.NewFakeRecorder(1024)
		gce.eventRecorder = recorder
		nodeNames := []string{"test-node-1"}

		svc := fakeLoadbalancerServiceWithLoadBalancerClass("", tc.loadBalancerClass)
		if tc.loadBalancerClass == "" {
			svc = fakeLoadbalancerService("")
		}
		svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Create NetLB
		status, err := createExternalLoadBalancer(gce, svc, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
		if tc.shouldProcess {
			assert.NoError(t, err)
			require.NotNil(t, status)
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			if hasFinalizer(svc, NetLBFinalizerV2) || hasFinalizer(svc, NetLBFinalizerV3) {
				t.Errorf("Unexpected finalizer found in Finalizer list - %v", svc.Finalizers)
			}
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
			assert.Empty(t, status)
		}

		nodeNames = []string{"test-node-1", "test-node-2"}
		nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
		assert.NoError(t, err)

		// Update NetLB
		err = gce.updateExternalLoadBalancer(vals.ClusterName, svc, nodes)
		if tc.shouldProcess {
			assert.NoError(t, err)
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			if !hasFinalizer(svc, NetLBFinalizerV1) {
				t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
			}
			if hasFinalizer(svc, NetLBFinalizerV2) || hasFinalizer(svc, NetLBFinalizerV3) {
				t.Errorf("Unexpected finalizer found in Finalizer list - %v", svc.Finalizers)
			}
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
		}

		// Delete ILB
		err = gce.ensureExternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
		if tc.shouldProcess {
			assert.NoError(t, err)
			assertExternalLbResourcesDeleted(t, gce, svc, vals, true)
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
		}
	}
}
