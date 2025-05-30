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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/mock"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	servicehelper "k8s.io/cloud-provider/service/helpers"
)

func createInternalLoadBalancer(gce *Cloud, svc *v1.Service, existingFwdRule *compute.ForwardingRule, nodeNames []string, clusterName, clusterID, zoneName string) (*v1.LoadBalancerStatus, error) {
	nodes, err := createAndInsertNodes(gce, nodeNames, zoneName)
	if err != nil {
		return nil, err
	}

	return gce.ensureInternalLoadBalancer(
		clusterName,
		clusterID,
		svc,
		existingFwdRule,
		nodes,
	)
}

func TestEnsureInternalBackendServiceUpdates(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	igName := makeInstanceGroupName(vals.ClusterID)
	igLinks, err := gce.ensureInternalInstanceGroups(igName, nodes)
	require.NoError(t, err)

	sharedBackend := shareBackendService(svc)
	bsName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)
	err = gce.ensureInternalBackendService(bsName, "description", svc.Spec.SessionAffinity, cloud.SchemeInternal, "TCP", igLinks, "")
	require.NoError(t, err)

	// Update the Internal Backend Service with a new ServiceAffinity
	err = gce.ensureInternalBackendService(bsName, "description", v1.ServiceAffinityNone, cloud.SchemeInternal, "TCP", igLinks, "")
	require.NoError(t, err)

	bs, err := gce.GetRegionBackendService(bsName, gce.region)
	assert.NoError(t, err)
	assert.Equal(t, bs.SessionAffinity, strings.ToUpper(string(v1.ServiceAffinityNone)))
}

func TestEnsureInternalBackendServiceGroups(t *testing.T) {
	t.Parallel()

	for desc, tc := range map[string]struct {
		mockModifier func(*cloud.MockGCE)
	}{
		"Basic workflow": {},
		"GetRegionBackendService failed": {
			mockModifier: func(c *cloud.MockGCE) {
				c.MockRegionBackendServices.GetHook = mock.GetRegionBackendServicesErrHook
			},
		},
		"UpdateRegionBackendServices failed": {
			mockModifier: func(c *cloud.MockGCE) {
				c.MockRegionBackendServices.UpdateHook = mock.UpdateRegionBackendServicesErrHook
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()
			nodeNames := []string{"test-node-1"}

			gce, err := fakeGCECloud(vals)
			require.NoError(t, err)

			svc := fakeLoadbalancerService(string(LBTypeInternal))
			lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
			nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			require.NoError(t, err)
			igName := makeInstanceGroupName(vals.ClusterID)
			igLinks, err := gce.ensureInternalInstanceGroups(igName, nodes)
			require.NoError(t, err)

			sharedBackend := shareBackendService(svc)
			bsName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)

			err = gce.ensureInternalBackendService(bsName, "description", svc.Spec.SessionAffinity, cloud.SchemeInternal, "TCP", igLinks, "")
			require.NoError(t, err)

			// Update the BackendService with new InstanceGroups
			if tc.mockModifier != nil {
				tc.mockModifier(gce.c.(*cloud.MockGCE))
			}
			newIGLinks := []string{"new-test-ig-1", "new-test-ig-2"}
			err = gce.ensureInternalBackendServiceGroups(bsName, newIGLinks)
			if tc.mockModifier != nil {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			bs, err := gce.GetRegionBackendService(bsName, gce.region)
			assert.NoError(t, err)

			// Check that the Backends reflect the new InstanceGroups
			backends := backendsFromGroupLinks(newIGLinks)
			assert.Equal(t, bs.Backends, backends)
		})
	}
}

func TestEnsureInternalInstanceGroupsLimit(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{}
	for i := 0; i < maxInstancesPerInstanceGroup+5; i++ {
		nodeNames = append(nodeNames, fmt.Sprintf("node-%d", i))
	}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	igName := makeInstanceGroupName(vals.ClusterID)
	_, err = gce.ensureInternalInstanceGroups(igName, nodes)
	require.NoError(t, err)
	instances, err := gce.ListInstancesInInstanceGroup(igName, vals.ZoneName, allInstances)
	require.NoError(t, err)
	assert.Equal(t, maxInstancesPerInstanceGroup, len(instances))
}

func TestEnsureMultipleInstanceGroups(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureSkipIGsManagement})

	nodes, err := createAndInsertNodes(gce, []string{"n1"}, vals.ZoneName)
	require.NoError(t, err)

	baseName := makeInstanceGroupName(vals.ClusterID)
	clusterIGs := []string{baseName, baseName + "-1", baseName + "-2", baseName + "-3"}
	for _, igName := range append(clusterIGs, "zz-another-ig", "k8s-ig--cluster2-id") {
		ig := &compute.InstanceGroup{Name: igName}
		err := gce.CreateInstanceGroup(ig, vals.ZoneName)
		require.NoError(t, err)
	}

	igsFromCloud, err := gce.ensureInternalInstanceGroups(baseName, nodes)
	require.NoError(t, err)
	assert.Len(t, igsFromCloud, len(clusterIGs), "Incorrect number of Instance Groups")
	sort.Strings(igsFromCloud)
	for i, igName := range clusterIGs {
		assert.True(t, strings.HasSuffix(igsFromCloud[i], igName))
	}
}

func TestEnsureInstanceGroupFromDefaultNetworkMultiSubnetClusterMode(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	vals.SubnetworkURL = "https://www.googleapis.com/compute/v1/projects/project/regions/us-central1/subnetworks/defaultSubnet"
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, []string{"n1", "n2", "n3", "n4", "n5"}, vals.ZoneName)
	require.NoError(t, err)
	// node with a matching subnet
	nodes[0].Labels[labelGKESubnetworkName] = "defaultSubnet"
	// node with a label of a non-matching subnet
	nodes[1].Labels[labelGKESubnetworkName] = "anotherSubnet"
	// node with no label but with PodCIDR
	nodes[2].Spec.PodCIDR = "10.0.5.0/24"
	// node[3] has no label nor PodCIDR
	nodes[3].Spec.PodCIDR = ""
	// node[4] has a label that contains an empty string (this indicates the default network).
	nodes[4].Spec.PodCIDR = ""
	nodes[4].Labels[labelGKESubnetworkName] = ""

	baseName := makeInstanceGroupName(vals.ClusterID)

	igsFromCloud, err := gce.ensureInternalInstanceGroups(baseName, nodes)
	require.NoError(t, err)

	url, err := cloud.ParseResourceURL(igsFromCloud[0])
	require.NoError(t, err)
	instances, err := gce.ListInstancesInInstanceGroup(url.Key.Name, url.Key.Zone, "ALL")
	require.NoError(t, err)
	assert.Len(t, instances, 4, "Incorrect number of Instances in the group")
	var instanceURLs []string
	for _, inst := range instances {
		instanceURLs = append(instanceURLs, inst.Instance)
	}
	if !hasInstanceForNode(instances, nodes[0]) {
		t.Errorf("expected n1 to be in instances but it contained %+v", instanceURLs)
	}
	if hasInstanceForNode(instances, nodes[1]) {
		t.Errorf("expected n2 to NOT be in instances but it was included %+v", instanceURLs)
	}
	if !hasInstanceForNode(instances, nodes[2]) {
		t.Errorf("expected n3 to be in instances but it contained %+v", instanceURLs)
	}
	if !hasInstanceForNode(instances, nodes[3]) {
		t.Errorf("expected n4 to be in instances but it contained %+v", instanceURLs)
	}
	if !hasInstanceForNode(instances, nodes[4]) {
		t.Errorf("expected n5 to be in instances but it contained %+v", instanceURLs)
	}
}

// Test that the node filtering will not be done if the cluster subnetwork value is not set properly.
func TestEnsureInstanceGroupFromDefaultNetworkMultiSubnetClusterModeIfSubnetworkValueIsInvalid(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	vals.SubnetworkURL = "invalid"
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, []string{"n1", "n2"}, vals.ZoneName)
	require.NoError(t, err)
	// node with a matching subnet
	nodes[0].Labels[labelGKESubnetworkName] = "defaultSubnet"
	// node with a label of a non-matching subnet
	nodes[1].Labels[labelGKESubnetworkName] = "anotherSubnet"

	baseName := makeInstanceGroupName(vals.ClusterID)

	igsFromCloud, err := gce.ensureInternalInstanceGroups(baseName, nodes)
	require.NoError(t, err)

	url, err := cloud.ParseResourceURL(igsFromCloud[0])
	require.NoError(t, err)
	instances, err := gce.ListInstancesInInstanceGroup(url.Key.Name, url.Key.Zone, "ALL")
	require.NoError(t, err)
	assert.Len(t, instances, 2, "Incorrect number of Instances in the group")
	var instanceURLs []string
	for _, inst := range instances {
		instanceURLs = append(instanceURLs, inst.Instance)
	}
	if !hasInstanceForNode(instances, nodes[0]) {
		t.Errorf("expected n1 to be in instances but it contained %+v", instanceURLs)
	}
	if !hasInstanceForNode(instances, nodes[1]) {
		t.Errorf("expected n2 to be in instances but it contained %+v", instanceURLs)
	}
}

func hasInstanceForNode(instances []*compute.InstanceWithNamedPorts, node *v1.Node) bool {
	for _, instance := range instances {
		if strings.HasSuffix(instance.Instance, node.Name) {
			return true
		}
	}
	return false
}

func TestRemoveNodesInNonDefaultNetworks(t *testing.T) {
	t.Parallel()

	testInput := []struct {
		node                    *v1.Node
		shouldBeInDefaultSubnet bool
	}{
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "defaultSubnetNodeWithLabel",
					Labels: map[string]string{labelGKESubnetworkName: "defaultSubnet"},
				},
			},
			shouldBeInDefaultSubnet: true,
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "nonDefaultSubnetNode",
					Labels: map[string]string{labelGKESubnetworkName: "secondarySubnet"},
				},
			},
			shouldBeInDefaultSubnet: false,
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "defaultSubnetNodeWithEmptyLabel",
					Labels: map[string]string{labelGKESubnetworkName: ""},
				},
			},
			shouldBeInDefaultSubnet: true,
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "defaultSubnetNodeWithoutLabel",
				},
				Spec: v1.NodeSpec{PodCIDR: "10.0.0.0/28"},
			},
			shouldBeInDefaultSubnet: true,
		},
		{
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nodeInUnknownSubnet",
				},
			},
			shouldBeInDefaultSubnet: true,
		},
	}
	var nodes []*v1.Node
	for _, testNode := range testInput {
		nodes = append(nodes, testNode.node)
	}

	onlyDefaultSubnetNodes := removeNodesInNonDefaultNetworks(nodes, "defaultSubnet")

	defaultSubnetNodesSet := make(map[string]struct{})
	for _, node := range onlyDefaultSubnetNodes {
		defaultSubnetNodesSet[node.Name] = struct{}{}
	}
	for _, testNode := range testInput {
		_, hasNode := defaultSubnetNodesSet[testNode.node.Name]
		if testNode.shouldBeInDefaultSubnet != hasNode {
			t.Errorf("Node %s should not be in the default subnet but it was present in %v", testNode.node.Name, defaultSubnetNodesSet)
		}
	}
}

func TestEnsureInternalLoadBalancer(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResources(t, gce, svc, vals, nodeNames)
}

func TestEnsureInternalLoadBalancerDeprecatedAnnotation(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	svc := fakeLoadBalancerServiceDeprecatedAnnotation(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("Failed to create service %s, err %v", svc.Name, err)
	}
	status, err := gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResources(t, gce, svc, vals, nodeNames)

	// Now add the latest annotation and change scheme to external
	svc.Annotations[ServiceAnnotationLoadBalancerType] = ""
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	require.NoError(t, err)

	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, false)
	assertExternalLbResources(t, gce, svc, vals, nodeNames)

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, NetLBFinalizerV1) {
		t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
	}
	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertExternalLbResourcesDeleted(t, gce, svc, vals, true)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestEnsureInternalLoadBalancerWithExistingResources(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	// Create the expected resources necessary for an Internal Load Balancer
	nm := types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	sharedHealthCheck := !servicehelper.RequestsOnlyLocalTraffic(svc)
	hcName := makeHealthCheckName(lbName, vals.ClusterID, sharedHealthCheck)
	hcPath, hcPort := GetNodesHealthCheckPath(), GetNodesHealthCheckPort()
	existingHC := newInternalLBHealthCheck(hcName, nm, sharedHealthCheck, hcPath, hcPort)
	err = gce.CreateHealthCheck(existingHC)
	require.NoError(t, err)

	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	igName := makeInstanceGroupName(vals.ClusterID)
	igLinks, err := gce.ensureInternalInstanceGroups(igName, nodes)
	require.NoError(t, err)

	sharedBackend := shareBackendService(svc)
	bsDescription := makeBackendServiceDescription(nm, sharedBackend)
	bsName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)
	err = gce.ensureInternalBackendService(bsName, bsDescription, svc.Spec.SessionAffinity, cloud.SchemeInternal, "TCP", igLinks, existingHC.SelfLink)
	require.NoError(t, err)

	_, err = createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)
}

func TestEnsureInternalLoadBalancerClearPreviousResources(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	// Create a ForwardingRule that's missing an IP address
	existingFwdRule := &compute.ForwardingRule{
		Name:                lbName,
		IPAddress:           "",
		Ports:               []string{"123"},
		IPProtocol:          "TCP",
		LoadBalancingScheme: string(cloud.SchemeInternal),
	}
	gce.CreateRegionForwardingRule(existingFwdRule, gce.region)

	// Create a Firewall that's missing a Description
	existingFirewall := &compute.Firewall{
		Name:    lbName,
		Network: gce.networkURL,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"123"},
			},
		},
	}
	gce.CreateFirewall(existingFirewall)

	sharedHealthCheck := !servicehelper.RequestsOnlyLocalTraffic(svc)
	hcName := makeHealthCheckName(lbName, vals.ClusterID, sharedHealthCheck)
	hcPath, hcPort := GetNodesHealthCheckPath(), GetNodesHealthCheckPort()
	nm := types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}

	// Create a healthcheck with an incorrect threshold
	existingHC := newInternalLBHealthCheck(hcName, nm, sharedHealthCheck, hcPath, hcPort)
	existingHC.CheckIntervalSec = gceHcCheckIntervalSeconds - 1
	gce.CreateHealthCheck(existingHC)

	// Create a backend Service that's missing Description and Backends
	sharedBackend := shareBackendService(svc)
	backendServiceName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)
	existingBS := &compute.BackendService{
		Name:                lbName,
		Protocol:            "TCP",
		HealthChecks:        []string{existingHC.SelfLink},
		SessionAffinity:     translateAffinityType(svc.Spec.SessionAffinity),
		LoadBalancingScheme: string(cloud.SchemeInternal),
	}

	gce.CreateRegionBackendService(existingBS, gce.region)
	existingFwdRule.BackendService = cloud.SelfLink(meta.VersionGA, vals.ProjectID, "backendServices", meta.RegionalKey(existingBS.Name, gce.region))

	_, err = createInternalLoadBalancer(gce, svc, existingFwdRule, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	// Expect new resources with the correct attributes to be created
	rule, _ := gce.GetRegionForwardingRule(lbName, gce.region)
	assert.NotEqual(t, existingFwdRule, rule)

	firewall, err := gce.GetFirewall(MakeFirewallName(lbName))
	require.NoError(t, err)
	assert.NotEqual(t, firewall, existingFirewall)

	healthcheck, err := gce.GetHealthCheck(hcName)
	require.NoError(t, err)
	assert.NotEqual(t, healthcheck, existingHC)

	bs, err := gce.GetRegionBackendService(backendServiceName, gce.region)
	require.NoError(t, err)
	assert.NotEqual(t, bs, existingBS)
}

func TestEnsureInternalLoadBalancerHealthCheckConfigurable(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	sharedHealthCheck := !servicehelper.RequestsOnlyLocalTraffic(svc)
	hcName := makeHealthCheckName(lbName, vals.ClusterID, sharedHealthCheck)
	hcPath, hcPort := GetNodesHealthCheckPath(), GetNodesHealthCheckPort()
	nm := types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}

	// Create a healthcheck with an incorrect threshold
	existingHC := newInternalLBHealthCheck(hcName, nm, sharedHealthCheck, hcPath, hcPort)
	existingHC.CheckIntervalSec = gceHcCheckIntervalSeconds * 10
	gce.CreateHealthCheck(existingHC)

	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	healthcheck, err := gce.GetHealthCheck(hcName)
	require.NoError(t, err)
	assert.Equal(t, healthcheck, existingHC)
}

func TestUpdateInternalLoadBalancerBackendServices(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeName := "test-node-1"

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	// BackendService exists prior to updateInternalLoadBalancer call, but has
	// incorrect (missing) attributes.
	// ensureInternalBackendServiceGroups is called and creates the correct
	// BackendService
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	sharedBackend := shareBackendService(svc)
	backendServiceName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)
	existingBS := &compute.BackendService{
		Name:                backendServiceName,
		Protocol:            "TCP",
		SessionAffinity:     translateAffinityType(svc.Spec.SessionAffinity),
		LoadBalancingScheme: string(cloud.SchemeInternal),
	}

	gce.CreateRegionBackendService(existingBS, gce.region)

	nodes, err := createAndInsertNodes(gce, []string{nodeName}, vals.ZoneName)
	require.NoError(t, err)

	err = gce.updateInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nodes)
	assert.NoError(t, err)

	bs, err := gce.GetRegionBackendService(backendServiceName, gce.region)
	require.NoError(t, err)

	// Check that the new BackendService has the correct attributes
	urlBase := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s", vals.ProjectID)

	assert.NotEqual(t, existingBS, bs)
	assert.Equal(
		t,
		bs.SelfLink,
		fmt.Sprintf("%s/regions/%s/backendServices/%s", urlBase, vals.Region, bs.Name),
	)
	assert.Equal(t, bs.Description, `{"kubernetes.io/service-name":"/`+svc.Name+`"}`)
	assert.Equal(
		t,
		bs.HealthChecks,
		[]string{fmt.Sprintf("%s/global/healthChecks/k8s-%s-node", urlBase, vals.ClusterID)},
	)
}

func TestUpdateInternalLoadBalancerNodes(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	node1Name := []string{"test-node-1"}

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	nodes, err := createAndInsertNodes(gce, node1Name, vals.ZoneName)
	require.NoError(t, err)

	_, err = gce.ensureInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	assert.NoError(t, err)

	// Replace the node in initial zone; add new node in a new zone.
	node2Name, node3Name := "test-node-2", "test-node-3"
	newNodesZoneA, err := createAndInsertNodes(gce, []string{node2Name}, vals.ZoneName)
	require.NoError(t, err)
	newNodesZoneB, err := createAndInsertNodes(gce, []string{node3Name}, vals.SecondaryZoneName)
	require.NoError(t, err)

	nodes = append(newNodesZoneA, newNodesZoneB...)
	err = gce.updateInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nodes)
	assert.NoError(t, err)

	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	sharedBackend := shareBackendService(svc)
	backendServiceName := makeBackendServiceName(lbName, vals.ClusterID, sharedBackend, cloud.SchemeInternal, "TCP", svc.Spec.SessionAffinity)
	bs, err := gce.GetRegionBackendService(backendServiceName, gce.region)
	require.NoError(t, err)
	assert.Equal(t, 2, len(bs.Backends), "Want two backends referencing two instances groups")

	for _, zone := range []string{vals.ZoneName, vals.SecondaryZoneName} {
		var found bool
		for _, be := range bs.Backends {
			if strings.Contains(be.Group, zone) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected list of backends to have zone %q", zone)
	}

	// Expect initial zone to have test-node-2
	igName := makeInstanceGroupName(vals.ClusterID)
	instances, err := gce.ListInstancesInInstanceGroup(igName, vals.ZoneName, "ALL")
	require.NoError(t, err)
	assert.Equal(t, 1, len(instances))
	assert.Contains(
		t,
		instances[0].Instance,
		fmt.Sprintf("%s/zones/%s/instances/%s", vals.ProjectID, vals.ZoneName, node2Name),
	)

	// Expect initial zone to have test-node-3
	instances, err = gce.ListInstancesInInstanceGroup(igName, vals.SecondaryZoneName, "ALL")
	require.NoError(t, err)
	assert.Equal(t, 1, len(instances))
	assert.Contains(
		t,
		instances[0].Instance,
		fmt.Sprintf("%s/zones/%s/instances/%s", vals.ProjectID, vals.SecondaryZoneName, node3Name),
	)
}

func TestUpdateInternalLoadBalancerNodesWithEmptyZone(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	nodeName := "test-node-1"
	node1Name := []string{nodeName}

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	nodes, err := createAndInsertNodes(gce, node1Name, vals.ZoneName)
	require.NoError(t, err)

	_, err = gce.ensureInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nil, nodes)
	assert.NoError(t, err)

	// Ensure Node has been added to instance group
	igName := makeInstanceGroupName(vals.ClusterID)
	instances, err := gce.ListInstancesInInstanceGroup(igName, vals.ZoneName, "ALL")
	require.NoError(t, err)
	assert.Equal(t, 1, len(instances))
	assert.Contains(
		t,
		instances[0].Instance,
		fmt.Sprintf("%s/zones/%s/instances/%s", vals.ProjectID, vals.ZoneName, nodeName),
	)

	// Remove Zone from node
	nodes[0].Labels[v1.LabelTopologyZone] = "" // empty zone

	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	existingFwdRule := &compute.ForwardingRule{
		Name:                lbName,
		IPAddress:           "",
		Ports:               []string{"123"},
		IPProtocol:          "TCP",
		LoadBalancingScheme: string(cloud.SchemeInternal),
		Description:         fmt.Sprintf(`{"kubernetes.io/service-name":"%s"}`, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}.String()),
	}

	_, err = gce.ensureInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, existingFwdRule, nodes)
	assert.NoError(t, err)

	// Expect load balancer to not have deleted node test-node-1
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	exist, err := gce.InstanceExists(context.TODO(), node)
	require.NoError(t, err)
	assert.Equal(t, exist, true)
}

func TestEnsureInternalLoadBalancerDeleted(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	assert.NoError(t, err)

	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestSkipInstanceGroupDeletion(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureSkipIGsManagement})
	err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	assert.NoError(t, err)

	igName := makeInstanceGroupName(vals.ClusterID)
	ig, err := gce.GetInstanceGroup(igName, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotNil(t, ig, "Instance group should not be deleted when flag 'NetLB_RBS' is present")
}

func TestEnsureInternalLoadBalancerDeletedTwiceDoesNotError(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	assert.NoError(t, err)

	// Deleting the loadbalancer and resources again should not cause an error.
	err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
	assert.NoError(t, err)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestEnsureInternalLoadBalancerWithSpecialHealthCheck(t *testing.T) {
	vals := DefaultTestClusterValues()
	nodeName := "test-node-1"
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	healthCheckNodePort := int32(10101)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc.Spec.HealthCheckNodePort = healthCheckNodePort
	svc.Spec.Type = v1.ServiceTypeLoadBalancer
	svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, []string{nodeName}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)

	loadBalancerName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	hc, err := gce.GetHealthCheck(loadBalancerName)
	assert.NoError(t, err)
	assert.NotNil(t, hc)
	assert.Equal(t, int64(healthCheckNodePort), hc.HttpHealthCheck.Port)
}

func TestClearPreviousInternalResources(t *testing.T) {
	// Configure testing environment.
	vals := DefaultTestClusterValues()
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	loadBalancerName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	nm := types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}
	c := gce.c.(*cloud.MockGCE)
	require.NoError(t, err)

	hc1, err := gce.ensureInternalHealthCheck("hc1", nm, false, "healthz", 12345)
	require.NoError(t, err)

	hc2, err := gce.ensureInternalHealthCheck("hc2", nm, false, "healthz", 12346)
	require.NoError(t, err)

	err = gce.ensureInternalBackendService(svc.ObjectMeta.Name, "", svc.Spec.SessionAffinity, cloud.SchemeInternal, v1.ProtocolTCP, []string{}, "")
	require.NoError(t, err)
	backendSvc, err := gce.GetRegionBackendService(svc.ObjectMeta.Name, gce.region)
	require.NoError(t, err)
	backendSvc.HealthChecks = []string{hc1.SelfLink, hc2.SelfLink}

	c.MockRegionBackendServices.DeleteHook = mock.DeleteRegionBackendServicesErrHook
	c.MockHealthChecks.DeleteHook = mock.DeleteHealthChecksInternalErrHook
	gce.clearPreviousInternalResources(svc, loadBalancerName, backendSvc, "expectedBSName", "expectedHCName")

	backendSvc, err = gce.GetRegionBackendService(svc.ObjectMeta.Name, gce.region)
	assert.NoError(t, err)
	assert.NotNil(t, backendSvc, "BackendService should not be deleted when api is mocked out.")
	hc1, err = gce.GetHealthCheck("hc1")
	assert.NoError(t, err)
	assert.NotNil(t, hc1, "HealthCheck should not be deleted when there are more than one healthcheck attached.")
	hc2, err = gce.GetHealthCheck("hc2")
	assert.NoError(t, err)
	assert.NotNil(t, hc2, "HealthCheck should not be deleted when there are more than one healthcheck attached.")

	c.MockRegionBackendServices.DeleteHook = mock.DeleteRegionBackendServicesInUseErrHook
	backendSvc.HealthChecks = []string{hc1.SelfLink}
	gce.clearPreviousInternalResources(svc, loadBalancerName, backendSvc, "expectedBSName", "expectedHCName")

	hc1, err = gce.GetHealthCheck("hc1")
	assert.NoError(t, err)
	assert.NotNil(t, hc1, "HealthCheck should not be deleted when api is mocked out.")

	c.MockHealthChecks.DeleteHook = mock.DeleteHealthChecksInuseErrHook
	gce.clearPreviousInternalResources(svc, loadBalancerName, backendSvc, "expectedBSName", "expectedHCName")

	hc1, err = gce.GetHealthCheck("hc1")
	assert.NoError(t, err)
	assert.NotNil(t, hc1, "HealthCheck should not be deleted when api is mocked out.")

	c.MockRegionBackendServices.DeleteHook = nil
	c.MockHealthChecks.DeleteHook = nil
	gce.clearPreviousInternalResources(svc, loadBalancerName, backendSvc, "expectedBSName", "expectedHCName")

	backendSvc, err = gce.GetRegionBackendService(svc.ObjectMeta.Name, gce.region)
	assert.Error(t, err)
	assert.Nil(t, backendSvc, "BackendService should be deleted.")
	hc1, err = gce.GetHealthCheck("hc1")
	assert.Error(t, err)
	assert.Nil(t, hc1, "HealthCheck should be deleted.")
}

func TestEnsureInternalFirewallDeletesLegacyFirewall(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	vals := DefaultTestClusterValues()
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	fwName := MakeFirewallName(lbName)

	c := gce.c.(*cloud.MockGCE)
	c.MockFirewalls.InsertHook = nil
	c.MockFirewalls.UpdateHook = nil

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	destinationIP := "10.1.2.3"
	sourceRange := []string{"10.0.0.0/20"}
	// Manually create a firewall rule with the legacy name - lbName
	gce.ensureInternalFirewall(
		svc,
		lbName,
		"firewall with legacy name",
		destinationIP,
		sourceRange,
		[]string{"123"},
		v1.ProtocolTCP,
		nodes,
		"")
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring legacy firewall %s for svc %+v", err, lbName, svc)
	}

	// Now ensure the firewall again with the correct name to simulate a sync after updating to new code.
	err = gce.ensureInternalFirewall(
		svc,
		fwName,
		"firewall with new name",
		destinationIP,
		sourceRange,
		[]string{"123", "456"},
		v1.ProtocolTCP,
		nodes,
		lbName)
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring firewall %s for svc %+v", err, fwName, svc)
	}

	existingFirewall, err := gce.GetFirewall(fwName)
	require.NoError(t, err)
	require.NotNil(t, existingFirewall)
	// Existing firewall will not be deleted yet since this was the first sync with the new rule created.
	existingLegacyFirewall, err := gce.GetFirewall(lbName)
	require.NoError(t, err)
	require.NotNil(t, existingLegacyFirewall)

	// Now ensure the firewall again to simulate a second sync where the old rule will be deleted.
	err = gce.ensureInternalFirewall(
		svc,
		fwName,
		"firewall with new name",
		destinationIP,
		sourceRange,
		[]string{"123", "456", "789"},
		v1.ProtocolTCP,
		nodes,
		lbName)
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring firewall %s for svc %+v", err, fwName, svc)
	}

	existingFirewall, err = gce.GetFirewall(fwName)
	require.NoError(t, err)
	require.NotNil(t, existingFirewall)
	existingLegacyFirewall, err = gce.GetFirewall(lbName)
	require.Error(t, err)
	require.Nil(t, existingLegacyFirewall)

}

func TestEnsureInternalFirewallSucceedsOnXPN(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	vals := DefaultTestClusterValues()
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	fwName := MakeFirewallName(lbName)

	c := gce.c.(*cloud.MockGCE)
	c.MockFirewalls.InsertHook = mock.InsertFirewallsUnauthorizedErrHook
	c.MockFirewalls.PatchHook = mock.UpdateFirewallsUnauthorizedErrHook
	gce.onXPN = true
	require.True(t, gce.OnXPN())

	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	destinationIP := "10.1.2.3"
	sourceRange := []string{"10.0.0.0/20"}
	gce.ensureInternalFirewall(
		svc,
		fwName,
		"A sad little firewall",
		destinationIP,
		sourceRange,
		[]string{"123"},
		v1.ProtocolTCP,
		nodes,
		lbName)
	require.Nil(t, err, "Should success when XPN is on.")

	checkEvent(t, recorder, FirewallChangeMsg, true)

	// Create a firewall.
	c.MockFirewalls.InsertHook = nil
	c.MockFirewalls.PatchHook = nil
	gce.onXPN = false

	gce.ensureInternalFirewall(
		svc,
		fwName,
		"A sad little firewall",
		destinationIP,
		sourceRange,
		[]string{"123"},
		v1.ProtocolTCP,
		nodes,
		lbName)
	require.NoError(t, err)
	existingFirewall, err := gce.GetFirewall(fwName)
	require.NoError(t, err)
	require.NotNil(t, existingFirewall)

	gce.onXPN = true
	c.MockFirewalls.InsertHook = mock.InsertFirewallsUnauthorizedErrHook
	c.MockFirewalls.PatchHook = mock.UpdateFirewallsUnauthorizedErrHook

	// Try to update the firewall just created.
	gce.ensureInternalFirewall(
		svc,
		fwName,
		"A happy little firewall",
		destinationIP,
		sourceRange,
		[]string{"123"},
		v1.ProtocolTCP,
		nodes,
		lbName)
	require.Nil(t, err, "Should success when XPN is on.")

	checkEvent(t, recorder, FirewallChangeMsg, true)
}

func TestEnsureLoadBalancerDeletedSucceedsOnXPN(t *testing.T) {
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	c := gce.c.(*cloud.MockGCE)
	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	c.MockFirewalls.DeleteHook = mock.DeleteFirewallsUnauthorizedErrHook
	gce.onXPN = true

	err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, fakeLoadbalancerService(string(LBTypeInternal)))
	assert.NoError(t, err)
	checkEvent(t, recorder, FirewallChangeMsg, true)
}

func TestEnsureInternalInstanceGroupsDeleted(t *testing.T) {
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	c := gce.c.(*cloud.MockGCE)
	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder
	require.NoError(t, err)

	igName := makeInstanceGroupName(vals.ClusterID)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = createInternalLoadBalancer(gce, svc, nil, []string{"test-node-1"}, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.NoError(t, err)

	c.MockZones.ListHook = mock.ListZonesInternalErrHook

	err = gce.ensureInternalLoadBalancerDeleted(igName, vals.ClusterID, svc)
	assert.Error(t, err, mock.InternalServerError)
	ig, err := gce.GetInstanceGroup(igName, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotNil(t, ig)

	c.MockZones.ListHook = nil
	c.MockInstanceGroups.DeleteHook = mock.DeleteInstanceGroupInternalErrHook

	err = gce.ensureInternalInstanceGroupsDeleted(igName)
	assert.Error(t, err, mock.InternalServerError)
	ig, err = gce.GetInstanceGroup(igName, vals.ZoneName)
	assert.NoError(t, err)
	assert.NotNil(t, ig)

	c.MockInstanceGroups.DeleteHook = nil
	err = gce.ensureInternalInstanceGroupsDeleted(igName)
	assert.NoError(t, err)
	ig, err = gce.GetInstanceGroup(igName, vals.ZoneName)
	assert.Error(t, err)
	assert.Nil(t, ig)
}

type EnsureILBParams struct {
	clusterName     string
	clusterID       string
	service         *v1.Service
	existingFwdRule *compute.ForwardingRule
	nodes           []*v1.Node
}

// newEnsureILBParams is the constructor of EnsureILBParams.
func newEnsureILBParams(nodes []*v1.Node) *EnsureILBParams {
	vals := DefaultTestClusterValues()
	return &EnsureILBParams{
		vals.ClusterName,
		vals.ClusterID,
		fakeLoadbalancerService(string(LBTypeInternal)),
		nil,
		nodes,
	}
}

// TestEnsureInternalLoadBalancerErrors tests the function
// ensureInternalLoadBalancer, making sure the system won't panic when
// exceptions raised by gce.
func TestEnsureInternalLoadBalancerErrors(t *testing.T) {
	vals := DefaultTestClusterValues()
	var params *EnsureILBParams

	for desc, tc := range map[string]struct {
		adjustParams func(*EnsureILBParams)
		injectMock   func(*cloud.MockGCE)
	}{
		"Create internal instance groups failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockInstanceGroups.GetHook = mock.GetInstanceGroupInternalErrHook
			},
		},
		"Invalid existing forwarding rules given": {
			adjustParams: func(params *EnsureILBParams) {
				params.existingFwdRule = &compute.ForwardingRule{BackendService: "badBackendService"}
			},
			injectMock: func(c *cloud.MockGCE) {
				c.MockRegionBackendServices.GetHook = mock.GetRegionBackendServicesErrHook
			},
		},
		"EnsureInternalBackendService failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockRegionBackendServices.GetHook = mock.GetRegionBackendServicesErrHook
			},
		},
		"Create internal health check failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockHealthChecks.GetHook = mock.GetHealthChecksInternalErrHook
			},
		},
		"Create firewall failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockFirewalls.InsertHook = mock.InsertFirewallsUnauthorizedErrHook
			},
		},
		"Create region forwarding rule failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockForwardingRules.InsertHook = mock.InsertForwardingRulesInternalErrHook
			},
		},
		"Get region forwarding rule failed": {
			injectMock: func(c *cloud.MockGCE) {
				c.MockForwardingRules.GetHook = mock.GetForwardingRulesInternalErrHook
			},
		},
		"Delete region forwarding rule failed": {
			adjustParams: func(params *EnsureILBParams) {
				params.existingFwdRule = &compute.ForwardingRule{BackendService: "badBackendService"}
			},
			injectMock: func(c *cloud.MockGCE) {
				c.MockForwardingRules.DeleteHook = mock.DeleteForwardingRuleErrHook
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			gce, err := fakeGCECloud(DefaultTestClusterValues())
			require.NoError(t, err)
			nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
			require.NoError(t, err)
			params = newEnsureILBParams(nodes)
			if tc.adjustParams != nil {
				tc.adjustParams(params)
			}
			if tc.injectMock != nil {
				tc.injectMock(gce.c.(*cloud.MockGCE))
			}
			_, err = gce.client.CoreV1().Services(params.service.Namespace).Create(context.TODO(), params.service, metav1.CreateOptions{})
			require.NoError(t, err)
			status, err := gce.ensureInternalLoadBalancer(
				params.clusterName,
				params.clusterID,
				params.service,
				params.existingFwdRule,
				params.nodes,
			)
			assert.Error(t, err, "Should return an error when "+desc)
			assert.Nil(t, status, "Should not return a status when "+desc)

			// ensure that the temporarily reserved IP address is released upon sync errors
			ip, err := gce.GetRegionAddress(gce.GetLoadBalancerName(context.TODO(), params.clusterName, params.service), gce.region)
			require.Error(t, err)
			assert.Nil(t, ip)
		})
	}
}

func TestMergeHealthChecks(t *testing.T) {
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
			wantHC := newInternalLBHealthCheck("hc", types.NamespacedName{Name: "svc", Namespace: "default"}, false, "/", 12345)
			hc := &compute.HealthCheck{
				CheckIntervalSec:   tc.checkIntervalSec,
				TimeoutSec:         tc.timeoutSec,
				HealthyThreshold:   tc.healthyThreshold,
				UnhealthyThreshold: tc.unhealthyThreshold,
			}
			mergeHealthChecks(hc, wantHC)
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

func TestCompareHealthChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc        string
		modifier    func(*compute.HealthCheck)
		wantChanged bool
	}{
		{"unchanged", nil, false},
		{"nil HttpHealthCheck", func(hc *compute.HealthCheck) { hc.HttpHealthCheck = nil }, true},
		{"desc does not match", func(hc *compute.HealthCheck) { hc.Description = "bad-desc" }, true},
		{"port does not match", func(hc *compute.HealthCheck) { hc.HttpHealthCheck.Port = 54321 }, true},
		{"requestPath does not match", func(hc *compute.HealthCheck) { hc.HttpHealthCheck.RequestPath = "/anotherone" }, true},
		{"interval needs update", func(hc *compute.HealthCheck) { hc.CheckIntervalSec = gceHcCheckIntervalSeconds - 1 }, true},
		{"timeout needs update", func(hc *compute.HealthCheck) { hc.TimeoutSec = gceHcTimeoutSeconds - 1 }, true},
		{"healthy threshold needs update", func(hc *compute.HealthCheck) { hc.HealthyThreshold = gceHcHealthyThreshold - 1 }, true},
		{"unhealthy threshold needs update", func(hc *compute.HealthCheck) { hc.UnhealthyThreshold = gceHcUnhealthyThreshold - 1 }, true},
		{"interval does not need update", func(hc *compute.HealthCheck) { hc.CheckIntervalSec = gceHcCheckIntervalSeconds + 1 }, false},
		{"timeout does not need update", func(hc *compute.HealthCheck) { hc.TimeoutSec = gceHcTimeoutSeconds + 1 }, false},
		{"healthy threshold does not need update", func(hc *compute.HealthCheck) { hc.HealthyThreshold = gceHcHealthyThreshold + 1 }, false},
		{"unhealthy threshold does not need update", func(hc *compute.HealthCheck) { hc.UnhealthyThreshold = gceHcUnhealthyThreshold + 1 }, false},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			hc := newInternalLBHealthCheck("hc", types.NamespacedName{Name: "svc", Namespace: "default"}, false, "/", 12345)
			wantHC := newInternalLBHealthCheck("hc", types.NamespacedName{Name: "svc", Namespace: "default"}, false, "/", 12345)
			if tc.modifier != nil {
				tc.modifier(hc)
			}
			if gotChanged := needToUpdateHealthChecks(hc, wantHC); gotChanged != tc.wantChanged {
				t.Errorf("needToUpdateHealthChecks(%#v, %#v) = %t; want changed = %t", hc, wantHC, gotChanged, tc.wantChanged)
			}
		})
	}
}

// Test creation of InternalLoadBalancer with ILB Subsets featuregate enabled.
func TestEnsureInternalLoadBalancerSubsetting(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc                 string
		finalizers           []string
		createForwardingRule bool
		expectErrorMsg       string
	}{
		{
			desc:           "New service creation fails with Implemented Elsewhere",
			expectErrorMsg: cloudprovider.ImplementedElsewhere.Error(),
		},
		{
			desc:                 "Service with existing ForwardingRule is processed",
			createForwardingRule: true,
		},
		{
			desc:       "Service with v1 finalizer is processed",
			finalizers: []string{ILBFinalizerV1},
		},
		{
			desc:           "Service with v2 finalizer is skipped",
			finalizers:     []string{ILBFinalizerV2},
			expectErrorMsg: cloudprovider.ImplementedElsewhere.Error(),
		},
		{
			desc:                 "Service with v2 finalizer and existing ForwardingRule is processed",
			finalizers:           []string{ILBFinalizerV2},
			createForwardingRule: true,
		},
		{
			desc:       "Service with v1 and v2 finalizers is processed",
			finalizers: []string{ILBFinalizerV1, ILBFinalizerV2},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vals := DefaultTestClusterValues()
			gce, err := fakeGCECloud(vals)
			require.NoError(t, err)
			gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureILBSubsets})
			recorder := record.NewFakeRecorder(1024)
			gce.eventRecorder = recorder

			nodeNames := []string{"test-node-1"}
			_, err = createAndInsertNodes(gce, nodeNames, vals.ZoneName)
			require.NoError(t, err)
			svc := fakeLoadbalancerService(string(LBTypeInternal))
			svc.Finalizers = tc.finalizers
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
			require.NoError(t, err)
			var existingFwdRule *compute.ForwardingRule
			if tc.createForwardingRule {
				// Create a ForwardingRule with the expected name
				existingFwdRule = &compute.ForwardingRule{
					Name:                gce.GetLoadBalancerName(context.TODO(), "", svc),
					IPAddress:           "5.6.7.8",
					Ports:               []string{"123"},
					IPProtocol:          "TCP",
					LoadBalancingScheme: string(cloud.SchemeInternal),
				}
				gce.CreateRegionForwardingRule(existingFwdRule, gce.region)
			}
			gotErrorMsg := ""
			status, err := createInternalLoadBalancer(gce, svc, existingFwdRule, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
			if err != nil {
				gotErrorMsg = err.Error()
			}
			if gotErrorMsg != tc.expectErrorMsg {
				t.Errorf("createInternalLoadBalancer() = %q, want error %q", err, tc.expectErrorMsg)
			}
			if err != nil {
				assert.Empty(t, status)
				assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
			} else {
				svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.NotEmpty(t, status.Ingress)
				assertInternalLbResources(t, gce, svc, vals, nodeNames)
				// Ensure that cleanup is successful, if applicable.
				err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
				assert.NoError(t, err)
				assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
			}
		})
	}
}

// TestEnsureInternalLoadBalancerDeletedSubsetting verifies that updates and deletion of existing ILB resources
// continue to work, even if ILBSubsets feature is enabled.
func TestEnsureInternalLoadBalancerDeletedSubsetting(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
	}
	// Enable FeatureGate
	gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureILBSubsets})
	// mock scenario where user updates the service to use a different IP, this should be processed here.
	svc.Spec.LoadBalancerIP = "1.2.3.4"
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	require.NoError(t, err)
	err = gce.UpdateLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	assert.NoError(t, err)
	// ensure service is still managed by this controller
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
	}
	// ensure that the status has the new IP
	assert.Equal(t, svc.Spec.LoadBalancerIP, "1.2.3.4")
	// Invoked when service is deleted.
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	assert.NoError(t, err)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

// TestEnsureInternalLoadBalancerUpdateSubsetting verifies that updates of existing ILB instance groups
// continue to work, even if ILBSubsets feature is enabled.
func TestEnsureInternalLoadBalancerUpdateSubsetting(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	assert.NoError(t, err)
	recorder := record.NewFakeRecorder(1024)
	gce.eventRecorder = recorder

	nodeNames := []string{"test-node-1"}
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	assert.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)

	assert.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
	}
	// Enable FeatureGate after service has been created.
	gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureILBSubsets})
	// mock scenario where user adds more nodes, this should be updated in the ILB.
	nodeNames = []string{"test-node-1", "test-node-2"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	assert.NoError(t, err)
	err = gce.UpdateLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	assert.NoError(t, err)
	// Ensure that the backend service/Instance group has both nodes.
	igName := makeInstanceGroupName(vals.ClusterID)
	instances, err := gce.ListInstancesInInstanceGroup(igName, vals.ZoneName, allInstances)
	assert.NoError(t, err)
	var instanceNames []string
	for _, inst := range instances {
		resourceID, err := cloud.ParseResourceURL(inst.Instance)
		if err != nil || resourceID == nil || resourceID.Key == nil {
			t.Errorf("Failed to parse instance url - %q, error - %v", inst.Instance, err)
			continue
		}
		instanceNames = append(instanceNames, resourceID.Key.Name)
	}
	if !equalStringSets(instanceNames, nodeNames) {
		t.Errorf("Got instances - %v, want %v", instanceNames, nodeNames)
	}
	// Invoked when service is deleted.
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	assert.NoError(t, err)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestEnsureInternalLoadBalancerGlobalAccess(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)

	// Change service to include the global access annotation
	svc.Annotations[ServiceAnnotationILBAllowGlobalAccess] = "true"
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if !fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected false value for AllowGlobalAccess")
	}
	// remove the annotation
	delete(svc.Annotations, ServiceAnnotationILBAllowGlobalAccess)
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected true value for AllowGlobalAccess")
	}
	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestEnsureInternalLoadBalancerDisableGlobalAccess(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	svc.Annotations[ServiceAnnotationILBAllowGlobalAccess] = "true"
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if !fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected false value for AllowGlobalAccess")
	}

	// disable global access - setting the annotation to false or removing annotation will disable it
	svc.Annotations[ServiceAnnotationILBAllowGlobalAccess] = "false"
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected true value for AllowGlobalAccess")
	}

	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestGlobalAccessChangeScheme(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	// Change service to include the global access annotation
	svc.Annotations[ServiceAnnotationILBAllowGlobalAccess] = "true"

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	require.NoError(t, err)

	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if !fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected false value for AllowGlobalAccess")
	}
	// change the scheme to externalLoadBalancer
	delete(svc.Annotations, ServiceAnnotationLoadBalancerType)

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
	require.NoError(t, err)

	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	// Firewall is deleted when the service is deleted
	assertInternalLbResourcesDeleted(t, gce, svc, vals, false)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.AllowGlobalAccess {
		t.Errorf("Unexpected true value for AllowGlobalAccess")
	}

	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, NetLBFinalizerV1) {
		t.Fatalf("Expected finalizer '%s' not found in Finalizer list - %v", NetLBFinalizerV1, svc.Finalizers)
	}
	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertExternalLbResourcesDeleted(t, gce, svc, vals, true)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestUnmarshalEmptyAPIVersion(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	existingFwdRule := &compute.ForwardingRule{
		Name:                lbName,
		IPAddress:           "",
		Ports:               []string{"123"},
		IPProtocol:          "TCP",
		LoadBalancingScheme: string(cloud.SchemeInternal),
		Description:         fmt.Sprintf(`{"kubernetes.io/service-name":"%s"}`, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}.String()),
	}
	var version meta.Version
	version, err = getFwdRuleAPIVersion(existingFwdRule)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if version != meta.VersionGA {
		t.Errorf("Unexpected version %s", version)
	}
}

func TestForwardingRulesEqual(t *testing.T) {
	t.Parallel()

	fwdRules := []*compute.ForwardingRule{
		{
			Name:                "empty-ip-address-fwd-rule",
			IPAddress:           "",
			Ports:               []string{"123"},
			IPProtocol:          "TCP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			BackendService:      "http://www.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
		{
			Name:                "tcp-fwd-rule",
			IPAddress:           "10.0.0.0",
			Ports:               []string{"123"},
			IPProtocol:          "TCP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			BackendService:      "http://www.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
		{
			Name:                "udp-fwd-rule",
			IPAddress:           "10.0.0.0",
			Ports:               []string{"123"},
			IPProtocol:          "UDP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			BackendService:      "http://www.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
		{
			Name:                "global-access-fwd-rule",
			IPAddress:           "10.0.0.0",
			Ports:               []string{"123"},
			IPProtocol:          "TCP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			AllowGlobalAccess:   true,
			BackendService:      "http://www.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
		{
			Name:                "global-access-fwd-rule",
			IPAddress:           "10.0.0.0",
			Ports:               []string{"123"},
			IPProtocol:          "TCP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			AllowGlobalAccess:   true,
			BackendService:      "http://compute.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
		{
			Name:                "udp-fwd-rule-allports",
			IPAddress:           "10.0.0.0",
			Ports:               []string{"123"},
			AllPorts:            true,
			IPProtocol:          "UDP",
			LoadBalancingScheme: string(cloud.SchemeInternal),
			BackendService:      "http://www.googleapis.com/projects/test/regions/us-central1/backendServices/bs1",
		},
	}

	for _, tc := range []struct {
		desc       string
		oldFwdRule *compute.ForwardingRule
		newFwdRule *compute.ForwardingRule
		expect     bool
	}{
		{
			desc:       "empty ip address matches any ip",
			oldFwdRule: fwdRules[0],
			newFwdRule: fwdRules[1],
			expect:     true,
		},
		{
			desc:       "global access enabled",
			oldFwdRule: fwdRules[1],
			newFwdRule: fwdRules[3],
			expect:     false,
		},
		{
			desc:       "IP protocol changed",
			oldFwdRule: fwdRules[1],
			newFwdRule: fwdRules[2],
			expect:     false,
		},
		{
			desc:       "same forwarding rule",
			oldFwdRule: fwdRules[3],
			newFwdRule: fwdRules[3],
			expect:     true,
		},
		{
			desc:       "same forwarding rule, different basepath",
			oldFwdRule: fwdRules[3],
			newFwdRule: fwdRules[4],
			expect:     true,
		},
		{
			desc:       "same forwarding rule, one uses AllPorts",
			oldFwdRule: fwdRules[2],
			newFwdRule: fwdRules[5],
			expect:     false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := forwardingRulesEqual(tc.oldFwdRule, tc.newFwdRule)
			if got != tc.expect {
				t.Errorf("forwardingRulesEqual(_, _) = %t, want %t", got, tc.expect)
			}
		})
	}
}

func TestEnsureInternalLoadBalancerCustomSubnet(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)

	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetBetaRegionForwardingRule(lbName, gce.region)
	if err != nil || fwdRule == nil {
		t.Errorf("Unexpected error %v", err)
	}
	if fwdRule.Subnetwork != "" {
		t.Errorf("Unexpected subnet value %s in ILB ForwardingRule", fwdRule.Subnetwork)
	}

	// Change service to include the global access annotation and request static ip
	requestedIP := "4.5.6.7"
	svc.Annotations[ServiceAnnotationILBSubnet] = "test-subnet"
	svc.Spec.LoadBalancerIP = requestedIP
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	if status.Ingress[0].IP != requestedIP {
		t.Errorf("Reserved IP %s not propagated, Got %s", requestedIP, status.Ingress[0].IP)
	}
	fwdRule, err = gce.GetBetaRegionForwardingRule(lbName, gce.region)
	if err != nil || fwdRule == nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !strings.HasSuffix(fwdRule.Subnetwork, "test-subnet") {
		t.Errorf("Unexpected subnet value %s in ILB ForwardingRule.", fwdRule.Subnetwork)
	}

	// Change to a different subnet
	svc.Annotations[ServiceAnnotationILBSubnet] = "another-subnet"
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	if status.Ingress[0].IP != requestedIP {
		t.Errorf("Reserved IP %s not propagated, Got %s", requestedIP, status.Ingress[0].IP)
	}
	fwdRule, err = gce.GetBetaRegionForwardingRule(lbName, gce.region)
	if err != nil || fwdRule == nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !strings.HasSuffix(fwdRule.Subnetwork, "another-subnet") {
		t.Errorf("Unexpected subnet value %s in ILB ForwardingRule.", fwdRule.Subnetwork)
	}
	// remove the annotation - ILB should revert to default subnet.
	delete(svc.Annotations, ServiceAnnotationILBSubnet)
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetBetaRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if fwdRule.Subnetwork != "" {
		t.Errorf("Unexpected subnet value %s in ILB ForwardingRule.", fwdRule.Subnetwork)
	}
	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestGetPortRanges(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		Desc   string
		Input  []int
		Result []string
	}{
		{Desc: "All Unique", Input: []int{8, 66, 23, 13, 89}, Result: []string{"8", "13", "23", "66", "89"}},
		{Desc: "All Unique Sorted", Input: []int{1, 7, 9, 16, 26}, Result: []string{"1", "7", "9", "16", "26"}},
		{Desc: "Ranges", Input: []int{56, 78, 67, 79, 21, 80, 12}, Result: []string{"12", "21", "56", "67", "78-80"}},
		{Desc: "Ranges Sorted", Input: []int{5, 7, 90, 1002, 1003, 1004, 1005, 2501}, Result: []string{"5", "7", "90", "1002-1005", "2501"}},
		{Desc: "Ranges Duplicates", Input: []int{15, 37, 900, 2002, 2003, 2003, 2004, 2004}, Result: []string{"15", "37", "900", "2002-2004"}},
		{Desc: "Duplicates", Input: []int{10, 10, 10, 10, 10}, Result: []string{"10"}},
		{Desc: "Only ranges", Input: []int{18, 19, 20, 21, 22, 55, 56, 77, 78, 79, 3504, 3505, 3506}, Result: []string{"18-22", "55-56", "77-79", "3504-3506"}},
		{Desc: "Single Range", Input: []int{6000, 6001, 6002, 6003, 6004, 6005}, Result: []string{"6000-6005"}},
		{Desc: "One value", Input: []int{12}, Result: []string{"12"}},
		{Desc: "Empty", Input: []int{}, Result: nil},
	} {
		result := getPortRanges(tc.Input)
		if !reflect.DeepEqual(result, tc.Result) {
			t.Errorf("Expected %v, got %v for test case %s", tc.Result, result, tc.Desc)
		}
	}
}

func TestEnsureInternalFirewallPortRanges(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	vals := DefaultTestClusterValues()
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	fwName := MakeFirewallName(lbName)
	tc := struct {
		Input  []int
		Result []string
	}{
		Input: []int{15, 37, 900, 2002, 2003, 2003, 2004, 2004}, Result: []string{"15", "37", "900", "2002-2004"},
	}
	c := gce.c.(*cloud.MockGCE)
	c.MockFirewalls.InsertHook = nil
	c.MockFirewalls.UpdateHook = nil

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)
	destinationIP := "10.1.2.3"
	sourceRange := []string{"10.0.0.0/20"}
	// Manually create a firewall rule with the legacy name - lbName
	err = gce.ensureInternalFirewall(
		svc,
		fwName,
		"firewall with legacy name",
		destinationIP,
		sourceRange,
		getPortRanges(tc.Input),
		v1.ProtocolTCP,
		nodes,
		"")
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring legacy firewall %s for svc %+v", err, lbName, svc)
	}
	existingFirewall, err := gce.GetFirewall(fwName)
	if err != nil || existingFirewall == nil || len(existingFirewall.Allowed) == 0 {
		t.Errorf("Unexpected error %v when looking up firewall %s, Got firewall %+v", err, fwName, existingFirewall)
	}
	existingPorts := existingFirewall.Allowed[0].Ports
	if !reflect.DeepEqual(existingPorts, tc.Result) {
		t.Errorf("Expected firewall rule with ports %v,got %v", tc.Result, existingPorts)
	}
}

func TestEnsureInternalFirewallDestinations(t *testing.T) {
	gce, err := fakeGCECloud(DefaultTestClusterValues())
	require.NoError(t, err)
	vals := DefaultTestClusterValues()
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	fwName := MakeFirewallName(lbName)

	nodes, err := createAndInsertNodes(gce, []string{"test-node-1"}, vals.ZoneName)
	require.NoError(t, err)

	destinationIP := "10.1.2.3"
	sourceRange := []string{"10.0.0.0/20"}

	err = gce.ensureInternalFirewall(
		svc,
		fwName,
		"firewall with legacy name",
		destinationIP,
		sourceRange,
		[]string{"8080"},
		v1.ProtocolTCP,
		nodes,
		"")
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring firewall %s for svc %+v", err, fwName, svc)
	}
	existingFirewall, err := gce.GetFirewall(fwName)
	if err != nil || existingFirewall == nil || len(existingFirewall.Allowed) == 0 {
		t.Errorf("Unexpected error %v when looking up firewall %s, Got firewall %+v", err, fwName, existingFirewall)
	}

	newDestinationIP := "20.1.2.3"

	err = gce.ensureInternalFirewall(
		svc,
		fwName,
		"firewall with legacy name",
		newDestinationIP,
		sourceRange,
		[]string{"8080"},
		v1.ProtocolTCP,
		nodes,
		"")
	if err != nil {
		t.Errorf("Unexpected error %v when ensuring firewall %s for svc %+v", err, fwName, svc)
	}

	updatedFirewall, err := gce.GetFirewall(fwName)
	if err != nil || updatedFirewall == nil || len(updatedFirewall.Allowed) == 0 {
		t.Errorf("Unexpected error %v when looking up firewall %s, Got firewall %+v", err, fwName, existingFirewall)
	}

	if reflect.DeepEqual(existingFirewall.DestinationRanges, updatedFirewall.DestinationRanges) {
		t.Errorf("DestinationRanges is not updated. existingFirewall.DestinationRanges: %v, updatedFirewall.DestinationRanges: %v", existingFirewall.DestinationRanges, updatedFirewall.DestinationRanges)
	}

}

func TestEnsureInternalLoadBalancerFinalizer(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResources(t, gce, svc, vals, nodeNames)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
	}

	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	require.NoError(t, err)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Finalizer '%s' not deleted as part of ILB delete", ILBFinalizerV1)
	}
}

// TestEnsureInternalLoadBalancerSkipped checks that the EnsureInternalLoadBalancer function skips creation of
// resources when the input service has a V2 finalizer.
func TestEnsureLoadBalancerSkipped(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	nodeNames := []string{"test-node-1"}
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	// Add the V2 finalizer
	svc.Finalizers = append(svc.Finalizers, ILBFinalizerV2)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	assert.EqualError(t, err, cloudprovider.ImplementedElsewhere.Error())
	// No loadbalancer resources will be created due to the ILB Feature Gate
	assert.Empty(t, status)
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

// TestEnsureLoadBalancerPartialDelete simulates a partial delete and checks whether deletion completes after a second
// attempt.
func TestEnsureLoadBalancerPartialDelete(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	nodeNames := []string{"test-node-1"}

	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	require.NoError(t, err)
	assert.NotEmpty(t, status.Ingress)
	assertInternalLbResources(t, gce, svc, vals, nodeNames)
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if !hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
	}
	// Delete the forwarding rule to simulate controller getting shut down on partial cleanup
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	err = gce.DeleteRegionForwardingRule(lbName, gce.region)
	require.NoError(t, err)
	// Check output of GetLoadBalancer
	_, exists, err := gce.GetLoadBalancer(context.TODO(), vals.ClusterName, svc)
	require.NoError(t, err)
	assert.True(t, exists)
	// call EnsureDeleted again
	err = gce.EnsureLoadBalancerDeleted(context.TODO(), vals.ClusterName, svc)
	require.NoError(t, err)
	// Make sure all resources are gone
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
	// Ensure that the finalizer has been deleted
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
	require.NoError(t, err)
	if hasFinalizer(svc, ILBFinalizerV1) {
		t.Errorf("Finalizer '%s' not deleted from service - %v", ILBFinalizerV1, svc.Finalizers)
	}
	_, exists, err = gce.GetLoadBalancer(context.TODO(), vals.ClusterName, svc)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestEnsureInternalLoadBalancerModifyProtocol(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	c := gce.c.(*cloud.MockGCE)
	c.MockRegionBackendServices.UpdateHook = func(ctx context.Context, key *meta.Key, be *compute.BackendService, m *cloud.MockRegionBackendServices, options ...cloud.Option) error {
		// Same key can be used since FR will have the same name.
		fr, err := c.MockForwardingRules.Get(ctx, key)
		if err != nil && !isNotFound(err) {
			return err
		}
		if fr != nil && fr.IPProtocol != be.Protocol {
			return fmt.Errorf("Protocol mismatch between Forwarding Rule value %q and Backend service value %q", fr.IPProtocol, be.Protocol)
		}
		return mock.UpdateRegionBackendServiceHook(ctx, key, be, m)
	}
	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.IPProtocol != "TCP" {
		t.Errorf("Unexpected protocol value %s, expected TCP", fwdRule.IPProtocol)
	}

	// change the protocol to UDP
	svc.Spec.Ports[0].Protocol = v1.ProtocolUDP
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.IPProtocol != "UDP" {
		t.Errorf("Unexpected protocol value %s, expected UDP", fwdRule.IPProtocol)
	}

	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestEnsureInternalLoadBalancerAllPorts(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	nodeNames := []string{"test-node-1"}
	nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
	require.NoError(t, err)
	svc := fakeLoadbalancerService(string(LBTypeInternal))
	svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(t, err)
	lbName := gce.GetLoadBalancerName(context.TODO(), "", svc)
	status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err := gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.Ports[0] != "123" {
		t.Errorf("Unexpected port value %v, expected [123]", fwdRule.Ports)
	}

	// Change service spec to use more than 5 ports
	svc.Spec.Ports = []v1.ServicePort{
		{Name: "testport", Port: int32(8080), Protocol: "TCP"},
		{Name: "testport", Port: int32(8090), Protocol: "TCP"},
		{Name: "testport", Port: int32(8100), Protocol: "TCP"},
		{Name: "testport", Port: int32(8200), Protocol: "TCP"},
		{Name: "testport", Port: int32(8300), Protocol: "TCP"},
		{Name: "testport", Port: int32(8400), Protocol: "TCP"},
	}
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if !fwdRule.AllPorts {
		t.Errorf("Unexpected AllPorts false value, expected true, FR - %v", fwdRule)
	}
	if len(fwdRule.Ports) != 0 {
		t.Errorf("Unexpected port value %v, expected empty list", fwdRule.Ports)
	}

	// Change service spec back to use < 5 ports
	svc.Spec.Ports = []v1.ServicePort{
		{Name: "testport", Port: int32(8090), Protocol: "TCP"},
		{Name: "testport", Port: int32(8100), Protocol: "TCP"},
		{Name: "testport", Port: int32(8300), Protocol: "TCP"},
		{Name: "testport", Port: int32(8400), Protocol: "TCP"},
	}
	expectPorts := []string{"8090", "8100", "8300", "8400"}
	status, err = gce.EnsureLoadBalancer(context.Background(), vals.ClusterName, svc, nodes)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assert.NotEmpty(t, status.Ingress)
	fwdRule, err = gce.GetRegionForwardingRule(lbName, gce.region)
	if err != nil {
		t.Errorf("gce.GetRegionForwardingRule(%q, %q) = %v, want nil", lbName, gce.region, err)
	}
	if fwdRule.AllPorts {
		t.Errorf("Unexpected AllPorts true value, expected false, FR - %v", fwdRule)
	}
	if !equalStringSets(fwdRule.Ports, expectPorts) {
		t.Errorf("Unexpected port value %v, expected %v", fwdRule.Ports, expectPorts)
	}

	// Delete the service
	err = gce.EnsureLoadBalancerDeleted(context.Background(), vals.ClusterName, svc)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
}

func TestSubnetNameFromURL(t *testing.T) {
	cases := []struct {
		desc     string
		url      string
		wantName string
		wantErr  bool
	}{
		{
			desc:     "full URL",
			url:      "https://www.googleapis.com/compute/v1/projects/project/regions/us-central1/subnetworks/defaultSubnet",
			wantName: "defaultSubnet",
		},
		{
			desc:     "project path",
			url:      "projects/project/regions/us-central1/subnetworks/defaultSubnet",
			wantName: "defaultSubnet",
		},
		{
			desc:    "missing name",
			url:     "projects/project/regions/us-central1/subnetworks",
			wantErr: true,
		},
		{
			desc:    "invalid",
			url:     "invalid",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			subnetName, err := subnetNameFromURL(tc.url)
			if err != nil && !tc.wantErr {
				t.Errorf("unexpected error %v", err)
			}
			if err == nil && tc.wantErr {
				t.Errorf("wanted an error but got none")
			}
			if !tc.wantErr && subnetName != tc.wantName {
				t.Errorf("invalid name extracted from URL %s, want=%s, got=%s", tc.url, tc.wantName, subnetName)
			}
		})
	}

}

func TestEnsureInternalLoadBalancerClass(t *testing.T) {
	t.Parallel()

	vals := DefaultTestClusterValues()
	for _, tc := range []struct {
		desc                 string
		loadBalancerClass    string
		shouldProcess        bool
		gkeSubsettingEnabled bool
	}{
		{
			desc:                 "Custom loadBalancerClass should not process",
			loadBalancerClass:    "customLBClass",
			shouldProcess:        false,
			gkeSubsettingEnabled: true,
		},
		{
			desc:                 "Use legacy ILB loadBalancerClass",
			loadBalancerClass:    LegacyRegionalInternalLoadBalancerClass,
			shouldProcess:        true,
			gkeSubsettingEnabled: true,
		},
		{
			desc:                 "Use legacy NetLB loadBalancerClass",
			loadBalancerClass:    LegacyRegionalExternalLoadBalancerClass,
			shouldProcess:        false,
			gkeSubsettingEnabled: true,
		},
		{
			desc:                 "don't process Unset loadBalancerClass with Subsetting enabled",
			loadBalancerClass:    "",
			shouldProcess:        false,
			gkeSubsettingEnabled: true,
		},
		{
			desc:                 "Custom loadBalancerClass should never process",
			loadBalancerClass:    "customLBClass",
			shouldProcess:        false,
			gkeSubsettingEnabled: false,
		},
		{
			desc:                 "Use legacy ILB loadBalancerClass",
			loadBalancerClass:    LegacyRegionalInternalLoadBalancerClass,
			shouldProcess:        true,
			gkeSubsettingEnabled: false,
		},
		{
			desc:                 "Use legacy NetLB loadBalancerClass",
			loadBalancerClass:    LegacyRegionalExternalLoadBalancerClass,
			shouldProcess:        false,
			gkeSubsettingEnabled: false,
		},
		{
			desc:                 "Subsetting disabled, process unset loadBalancerClass",
			loadBalancerClass:    "",
			shouldProcess:        true,
			gkeSubsettingEnabled: false,
		},
	} {
		gce, err := fakeGCECloud(vals)
		assert.NoError(t, err)
		// Enable FeatureGate GKE Subsetting
		if tc.gkeSubsettingEnabled {
			gce.AlphaFeatureGate = NewAlphaFeatureGate([]string{AlphaFeatureILBSubsets})
		}
		recorder := record.NewFakeRecorder(1024)
		gce.eventRecorder = recorder
		nodeNames := []string{"test-node-1"}

		svc := fakeLoadbalancerServiceWithLoadBalancerClass("", tc.loadBalancerClass)
		if tc.loadBalancerClass == "" {
			svc = fakeLoadbalancerService(string(LBTypeInternal))
		}
		svc, err = gce.client.CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
		assert.NoError(t, err)

		// Create ILB
		status, err := createInternalLoadBalancer(gce, svc, nil, nodeNames, vals.ClusterName, vals.ClusterID, vals.ZoneName)
		if tc.shouldProcess {
			assert.NoError(t, err)
			assert.NotEmpty(t, status.Ingress)
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			if !hasFinalizer(svc, ILBFinalizerV1) {
				t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
			}
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
			assert.Empty(t, status)
		}

		nodeNames = []string{"test-node-1", "test-node-2"}
		nodes, err := createAndInsertNodes(gce, nodeNames, vals.ZoneName)
		assert.NoError(t, err)

		// Update ILB
		err = gce.updateInternalLoadBalancer(vals.ClusterName, vals.ClusterID, svc, nodes)
		if tc.shouldProcess {
			assert.NoError(t, err)
			svc, err = gce.client.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			if !hasFinalizer(svc, ILBFinalizerV1) {
				t.Errorf("Expected finalizer '%s' not found in Finalizer list - %v", ILBFinalizerV1, svc.Finalizers)
			}
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
			assert.Empty(t, status)
		}

		// Delete ILB
		err = gce.ensureInternalLoadBalancerDeleted(vals.ClusterName, vals.ClusterID, svc)
		if tc.shouldProcess {
			assert.NoError(t, err)
			assertInternalLbResourcesDeleted(t, gce, svc, vals, true)
		} else {
			assert.ErrorIs(t, err, cloudprovider.ImplementedElsewhere)
		}
	}
}
