//go:build !providerless

/*
Copyright 2026 Red Hat, Inc.

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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Test constants matching real OSD cluster topology from OCPBUGS-78471.
// node-instance-prefix and external-instance-groups-prefix are the same
// value (the infra name), and all node names are FQDNs.
const testInfraName = "test-cluster-a1b2c"

func fqdnName(short string) string {
	return fmt.Sprintf("%s.c.%s.internal", short, "test-project")
}

func TestFilterNodeObjectFromName(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		nodes    []*v1.Node
		names    []string
		expected []string
	}{
		"FQDN node names match GCE short names": {
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: fqdnName(testInfraName + "-worker-a-wnjp7")}},
				{ObjectMeta: metav1.ObjectMeta{Name: fqdnName(testInfraName + "-worker-b-s48dq")}},
			},
			names: []string{
				testInfraName + "-worker-a-wnjp7",
				testInfraName + "-worker-b-s48dq",
			},
			expected: []string{
				fqdnName(testInfraName + "-worker-a-wnjp7"),
				fqdnName(testInfraName + "-worker-b-s48dq"),
			},
		},
		"short node names still match": {
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: testInfraName + "-worker-a-wnjp7"}},
				{ObjectMeta: metav1.ObjectMeta{Name: testInfraName + "-worker-b-s48dq"}},
			},
			names:    []string{testInfraName + "-worker-a-wnjp7"},
			expected: []string{testInfraName + "-worker-a-wnjp7"},
		},
		"mixed FQDN and short names": {
			nodes: []*v1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: fqdnName(testInfraName + "-worker-a-wnjp7")}},
				{ObjectMeta: metav1.ObjectMeta{Name: testInfraName + "-worker-b-s48dq"}},
				{ObjectMeta: metav1.ObjectMeta{Name: fqdnName(testInfraName + "-infra-a-zztd5")}},
			},
			names: []string{
				testInfraName + "-worker-a-wnjp7",
				testInfraName + "-worker-b-s48dq",
			},
			expected: []string{
				fqdnName(testInfraName + "-worker-a-wnjp7"),
				testInfraName + "-worker-b-s48dq",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			filtered := filterNodeObjectFromName(tc.nodes, tc.names)
			require.Len(t, filtered, len(tc.expected))
			for i, node := range filtered {
				assert.Equal(t, tc.expected[i], node.Name)
			}
		})
	}
}

func TestEvaluateExternalInstanceGroup(t *testing.T) {
	t.Parallel()
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)
	gce.nodeInstancePrefix = testInfraName
	zone := vals.ZoneName

	// CAPG creates one master IG per zone. During install, the bootstrap node
	// lands in the same IG as the master in that zone (e.g. both master-0 and
	// bootstrap in <infra>-master-<zone>).
	masterIGName := testInfraName + "-master-" + zone

	// IG contains only the master. shouldReuse=true via hasAll.
	err = gce.CreateInstanceGroup(&compute.InstanceGroup{Name: masterIGName}, zone)
	require.NoError(t, err)
	err = gce.AddInstancesToInstanceGroup(masterIGName, zone, []*compute.InstanceReference{
		{Instance: fmt.Sprintf("zones/%s/instances/%s-master-0", zone, testInfraName)},
	})
	require.NoError(t, err)
	masterIG, err := gce.GetInstanceGroup(masterIGName, zone)
	require.NoError(t, err)

	gceHostNames := sets.NewString(
		testInfraName+"-master-0",
		testInfraName+"-worker-a-wnjp7",
		testInfraName+"-infra-a-zztd5",
	)
	shouldReuse, instanceNames, err := gce.evaluateExternalInstanceGroup(masterIG, zone, gceHostNames)
	require.NoError(t, err)
	assert.True(t, shouldReuse, "should reuse when all IG instances are in zone node list")
	assert.True(t, instanceNames.Has(testInfraName+"-master-0"))

	// During CAPG install (OCPBUGS-35256): bootstrap node is in the same master
	// IG as master-0 (same zone). Bootstrap is not a k8s node so hasAll=false,
	// but all instances share the infra prefix so allHavePrefix=true.
	err = gce.AddInstancesToInstanceGroup(masterIGName, zone, []*compute.InstanceReference{
		{Instance: fmt.Sprintf("zones/%s/instances/%s-bootstrap", zone, testInfraName)},
	})
	require.NoError(t, err)

	shouldReuse, _, err = gce.evaluateExternalInstanceGroup(masterIG, zone, gceHostNames)
	require.NoError(t, err)
	assert.True(t, shouldReuse, "should reuse master IG even with bootstrap node present")
}

func TestFilterNodesWithExistingExternalInstanceGroups_FQDNNodes(t *testing.T) {
	t.Parallel()
	vals := DefaultTestClusterValues()
	gce, err := fakeGCECloud(vals)
	require.NoError(t, err)

	// Both prefixes are the infra name, matching real cloud config
	gce.nodeInstancePrefix = testInfraName
	gce.externalInstanceGroupsPrefix = testInfraName

	zoneA := vals.ZoneName          // us-central1-b
	zoneB := vals.SecondaryZoneName // us-central1-c

	// GCE instances (short names) across two zones — masters, workers, infra
	zoneAInstances := []string{
		testInfraName + "-master-0",
		testInfraName + "-worker-a-wnjp7",
		testInfraName + "-infra-a-zztd5",
	}
	zoneBInstances := []string{
		testInfraName + "-master-1",
		testInfraName + "-worker-b-s48dq",
		testInfraName + "-infra-b-2bn6x",
	}
	for _, name := range zoneAInstances {
		err = gce.InsertInstance(gce.ProjectID(), zoneA, &compute.Instance{
			Name: name,
			Tags: &compute.Tags{Items: []string{name}},
			Zone: zoneA,
		})
		require.NoError(t, err)
	}
	for _, name := range zoneBInstances {
		err = gce.InsertInstance(gce.ProjectID(), zoneB, &compute.Instance{
			Name: name,
			Tags: &compute.Tags{Items: []string{name}},
			Zone: zoneB,
		})
		require.NoError(t, err)
	}

	// External master IGs per zone (created by installer, match externalInstanceGroupsPrefix)
	for _, z := range []struct {
		zone   string
		master string
	}{
		{zoneA, testInfraName + "-master-0"},
		{zoneB, testInfraName + "-master-1"},
	} {
		igName := testInfraName + "-master-" + z.zone
		err = gce.CreateInstanceGroup(&compute.InstanceGroup{Name: igName}, z.zone)
		require.NoError(t, err)
		err = gce.AddInstancesToInstanceGroup(igName, z.zone, []*compute.InstanceReference{
			{Instance: fmt.Sprintf("zones/%s/instances/%s", z.zone, z.master)},
		})
		require.NoError(t, err)
	}

	// Nodes with FQDN names across both zones — masters, workers, infra
	nodes := []*v1.Node{
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-master-0"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneA},
		}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-worker-a-wnjp7"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneA},
		}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-infra-a-zztd5"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneA},
		}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-master-1"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneB},
		}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-worker-b-s48dq"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneB},
		}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:   fqdnName(testInfraName + "-infra-b-2bn6x"),
			Labels: map[string]string{v1.LabelTopologyZone: zoneB},
		}},
	}

	lbIGName := "k8s-ig--test-lb"
	filteredNodes, existingIGLinks, err := gce.filterNodesWithExistingExternalInstanceGroups(lbIGName, nodes)
	require.NoError(t, err)

	// Masters filtered out (covered by external IGs), workers + infra remain
	assert.Len(t, existingIGLinks, 2, "one master IG per zone should be reused")
	assert.Len(t, filteredNodes, 4, "workers and infra nodes should remain for internal IG creation")

	filteredNames := sets.NewString()
	for _, n := range filteredNodes {
		filteredNames.Insert(n.Name)
	}
	assert.True(t, filteredNames.Has(fqdnName(testInfraName+"-worker-a-wnjp7")))
	assert.True(t, filteredNames.Has(fqdnName(testInfraName+"-worker-b-s48dq")))
	assert.True(t, filteredNames.Has(fqdnName(testInfraName+"-infra-a-zztd5")))
	assert.True(t, filteredNames.Has(fqdnName(testInfraName+"-infra-b-2bn6x")))
	assert.False(t, filteredNames.Has(fqdnName(testInfraName+"-master-0")))
	assert.False(t, filteredNames.Has(fqdnName(testInfraName+"-master-1")))
}
