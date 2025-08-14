/*
Copyright 2024 Red Hat, Inc.

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
	"slices"
	"strings"

	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// filterNodesWithExistingExternalInstanceGroups filters out nodes that are already managed by
// existing external instance groups matching externalInstanceGroupsPrefix. This implements
// the OpenShift-specific logic for reusing external instance groups to work around GCP
// internal load balancer restrictions for multi-subnet clusters.
//
// Algorithm: Work around GCP internal load balancer restrictions for multi-subnet clusters.
// GCP documentation:
// - Internal LBs can load balance to VMs in same region but different subnets
// - VMs cannot be in more than one instance group, regardless of LB backend. (backend service restrictions)
// - Instance groups can "only select VMs that are in the same zone, VPC network, and subnet"
// - "All VMs in an instance group must have their primary network interface in the same VPC network"
//
// For clusters with nodes across multiple subnets, we use a two-pass approach:
// Pass 1: Find existing external instance groups (matching externalInstanceGroupsPrefix)
//
//	that contain ONLY our cluster nodes and reuse them for the backend service.
//
// Pass 2: Create internal instance groups only for remaining nodes not covered by external groups.
// This ensures compliance with GCP restrictions while enabling multi-subnet load balancing.
func (g *Cloud) filterNodesWithExistingExternalInstanceGroups(name string, nodes []*v1.Node) ([]*v1.Node, []string, error) {
	zonedNodes := splitNodesByZone(nodes)
	var existingIGLinks []string
	var filteredNodes []*v1.Node

	// instancesNeedingInternalInstanceGroups tracks instances that don't have
	// pre-existing external instance groups defined by the externalInstanceGroupsPrefix.
	// They should map to nodes 1:1.

	// This separation is necessary to comply with GCP's restriction that VMs cannot be in
	// more than one load-balanced instance group.
	instancesNeedingInternalInstanceGroups := map[string][]string{}

	for zone, nodesInZone := range zonedNodes {
		gceHostNamesInZone, err := g.gceInstanceNamesInZone(nodesInZone)
		if err != nil {
			return nil, nil, err
		}

		// Track instances that are already managed by existing external instance groups
		instancesInExistingInstanceGroups := sets.NewString()

		candidateExternalInstanceGroups, err := g.candidateExternalInstanceGroups(zone)
		if err != nil {
			return nil, nil, err
		}

		for _, ig := range candidateExternalInstanceGroups {
			if strings.EqualFold(ig.Name, name) {
				continue
			}

			shouldReuse, instanceNames, err := g.evaluateExternalInstanceGroup(ig, zone, gceHostNamesInZone)
			if err != nil {
				return nil, nil, err
			}

			if shouldReuse {
				existingIGLinks = append(existingIGLinks, ig.SelfLink)
				instancesInExistingInstanceGroups.Insert(instanceNames.UnsortedList()...)
			}
		}

		// Determine which instances (nodes) in this zone need internal instance groups created.
		// These are instances that exist in the zone but are not already managed by external instance groups.
		if remainingInstances := gceHostNamesInZone.Difference(instancesInExistingInstanceGroups).UnsortedList(); len(remainingInstances) > 0 {
			instancesNeedingInternalInstanceGroups[zone] = remainingInstances
		}
	}

	// Build the filtered node list from instances that need internal instance groups
	for zone, nodeNames := range instancesNeedingInternalInstanceGroups {
		allNodesInZone := zonedNodes[zone]
		nodesForInternalInstanceGroup := filterNodeObjectFromName(allNodesInZone, nodeNames)
		filteredNodes = append(filteredNodes, nodesForInternalInstanceGroup...)
	}

	return filteredNodes, existingIGLinks, nil
}

// filterNodeObjectFromName takes a list of *v1.Node and a list of node names,
// and returns the *v1.Node objects that match the provided names.
func filterNodeObjectFromName(nodesInZone []*v1.Node, nodeNames []string) []*v1.Node {
	filteredNodes := []*v1.Node{}

	for _, node := range nodesInZone {
		if slices.Contains(nodeNames, node.Name) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	return filteredNodes
}

// extractInstanceNamesFromGroup extracts instance names from a list of instances in an instance group.
func extractInstanceNamesFromGroup(instances []*compute.InstanceWithNamedPorts) sets.String {
	instanceNames := sets.NewString()
	for _, ins := range instances {
		// Extract instance name from URL path (e.g., ".../instances/node-name")
		parts := strings.Split(ins.Instance, "/")
		instanceNames.Insert(parts[len(parts)-1])
	}
	return instanceNames
}

// evaluateExternalInstanceGroup determines if an external instance group can be reused.
// It returns whether the group should be reused and the set of instance names in the group.
func (g *Cloud) evaluateExternalInstanceGroup(ig *compute.InstanceGroup, zone string, gceHostNamesInZone sets.String) (shouldReuse bool, instanceNames sets.String, err error) {
	// Get all instances in this external instance group
	instances, err := g.ListInstancesInInstanceGroup(ig.Name, zone, allInstances)
	if err != nil {
		return false, nil, err
	}

	// Extract instance names from the group
	instanceNames = extractInstanceNamesFromGroup(instances)

	// If all instances in this external instance group are also in our zone's node list,
	// or they all have the node instance prefix, we can reuse this instance group instead
	// of creating our own internal instance group
	shouldReuse = gceHostNamesInZone.HasAll(instanceNames.UnsortedList()...) || g.allHaveNodePrefix(instanceNames.UnsortedList())

	return shouldReuse, instanceNames, nil
}

// allHaveNodePrefix checks if all instances have the cluster's node instance prefix.
func (g *Cloud) allHaveNodePrefix(instances []string) bool {
	for _, instance := range instances {
		if !strings.HasPrefix(instance, g.nodeInstancePrefix) {
			return false
		}
	}
	return true
}

// candidateExternalInstanceGroups returns instance groups with the external instance groups prefix, if defined.
func (g *Cloud) candidateExternalInstanceGroups(zone string) ([]*compute.InstanceGroup, error) {
	if g.externalInstanceGroupsPrefix == "" {
		return nil, nil
	}

	return g.ListInstanceGroupsWithPrefix(zone, g.externalInstanceGroupsPrefix)
}

// gceInstanceNamesInZone returns a set of GCE Host names from the list of nodes provided
func (g *Cloud) gceInstanceNamesInZone(zoneNodes []*v1.Node) (sets.String, error) {
	// hosts is a list of GCE instances matching the zone's node names.
	hosts, err := g.getFoundInstanceByNames(nodeNames(zoneNodes))
	if err != nil {
		return nil, err
	}

	names := sets.NewString()
	for _, h := range hosts {
		names.Insert(h.Name)
	}

	return names, nil
}
