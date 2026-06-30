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
	"fmt"
	"slices"
	"strings"

	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
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
		instancesInExistingInstanceGroups := sets.New[string]()

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
		canonicalName := canonicalizeInstanceName(node.Name)
		if slices.Contains(nodeNames, canonicalName) {
			filteredNodes = append(filteredNodes, node)
		} else {
			klog.Warningf("filterNodeObjectFromName: node %q (canonical: %q) not found in GCE instance names %v", node.Name, canonicalName, nodeNames)
		}
	}
	return filteredNodes
}

// extractInstanceNamesFromGroup extracts instance names from a list of instances in an instance group.
func extractInstanceNamesFromGroup(instances []*compute.InstanceWithNamedPorts) sets.Set[string] {
	instanceNames := sets.New[string]()
	for _, ins := range instances {
		// Extract instance name from URL path (e.g., ".../instances/node-name")
		parts := strings.Split(ins.Instance, "/")
		instanceNames.Insert(parts[len(parts)-1])
	}
	return instanceNames
}

// evaluateExternalInstanceGroup determines if an external instance group can be reused.
// It returns whether the group should be reused and the set of instance names in the group.
func (g *Cloud) evaluateExternalInstanceGroup(ig *compute.InstanceGroup, zone string, gceHostNamesInZone sets.Set[string]) (shouldReuse bool, instanceNames sets.Set[string], err error) {
	// Get all instances in this external instance group
	instances, err := g.ListInstancesInInstanceGroup(ig.Name, zone, allInstances)
	if err != nil {
		return false, nil, err
	}

	// Extract instance names from the group
	instanceNames = extractInstanceNamesFromGroup(instances)

	// During bootstrap the bootstrap machine is placed in one of the master
	// instance groups. However, the bootstrap machine does not have an
	// associated Node. This will prevent us from considering this instance
	// group for reuse because the instance group contains an instance which is
	// not in the service's Node list. Consequently we will attempt to add the
	// masters to a second instance group, which will fail with
	// INSTANCE_IN_MULTIPLE_LOAD_BALANCED_IGS.
	//
	// To avoid this we explicitly exclude the bootstrap machine from
	// consideration if it is present.
	bootstrapInstanceName := fmt.Sprintf("%s-bootstrap", g.nodeInstancePrefix)
	instanceNames.Delete(bootstrapInstanceName)

	// If all instances in this external instance group are also in our zone's node list
	shouldReuse = gceHostNamesInZone.HasAll(instanceNames.UnsortedList()...) && instanceNames.Len() > 0
	klog.V(2).Infof("evaluateExternalInstanceGroup(%v): shouldReuse=%v, instances=%v", ig.Name, shouldReuse, instanceNames.UnsortedList())

	return shouldReuse, instanceNames, nil
}

// candidateExternalInstanceGroups returns instance groups with the external instance groups prefix, if defined.
func (g *Cloud) candidateExternalInstanceGroups(zone string) ([]*compute.InstanceGroup, error) {
	if g.externalInstanceGroupsPrefix == "" {
		return nil, nil
	}

	return g.ListInstanceGroupsWithPrefix(zone, g.externalInstanceGroupsPrefix)
}

// gceInstanceNamesInZone returns a set of GCE Host names from the list of nodes provided
func (g *Cloud) gceInstanceNamesInZone(zoneNodes []*v1.Node) (sets.Set[string], error) {
	// hosts is a list of GCE instances matching the zone's node names.
	hosts, err := g.getFoundInstanceByNames(nodeNames(zoneNodes))
	if err != nil {
		return nil, err
	}

	names := sets.New[string]()
	for _, h := range hosts {
		names.Insert(h.Name)
	}

	return names, nil
}
