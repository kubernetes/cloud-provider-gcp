/*
Copyright 2024 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"math/rand"
	"strings"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	gcecloud "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/kubernetes/test/e2e/framework"
)

// Run when the "gce" provider is registered in "init()".
func factory() (framework.ProviderInterface, error) {
	framework.Logf("Fetching cloud provider for %q\r", framework.TestContext.Provider)
	zone := framework.TestContext.CloudConfig.Zone
	region := framework.TestContext.CloudConfig.Region
	allowedZones := framework.TestContext.CloudConfig.Zones

	// ensure users don't specify a zone outside of the requested zones
	if len(zone) > 0 && len(allowedZones) > 0 {
		var found bool
		for _, allowedZone := range allowedZones {
			if zone == allowedZone {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("the provided zone %q must be included in the list of allowed zones %v", zone, allowedZones)
		}
	}

	var err error
	if region == "" {
		region, err = gcecloud.GetGCERegion(zone)
		if err != nil {
			return nil, fmt.Errorf("error parsing GCE/GKE region from zone %q: %w", zone, err)
		}
	}
	managedZones := []string{} // Manage all zones in the region
	if !framework.TestContext.CloudConfig.MultiZone {
		managedZones = []string{zone}
	}
	if len(allowedZones) > 0 {
		managedZones = allowedZones
	}

	gceCloud, err := gcecloud.CreateGCECloud(&gcecloud.CloudConfig{
		APIEndpoint:        framework.TestContext.CloudConfig.APIEndpoint,
		ProjectID:          framework.TestContext.CloudConfig.ProjectID,
		Region:             region,
		Zone:               zone,
		ManagedZones:       managedZones,
		NetworkName:        "", // TODO: Change this to use framework.TestContext.CloudConfig.Network?
		SubnetworkName:     "",
		NodeTags:           nil,
		NodeInstancePrefix: "",
		TokenSource:        nil,
		UseMetadataServer:  false,
		AlphaFeatureGate:   gcecloud.NewAlphaFeatureGate([]string{}),
	})

	if err != nil {
		return nil, fmt.Errorf("Error building GCE/GKE provider: %w", err)
	}

	// Arbitrarily pick one of the zones we have nodes in, looking at prepopulated zones first.
	if framework.TestContext.CloudConfig.Zone == "" && len(managedZones) > 0 {
		framework.TestContext.CloudConfig.Zone = managedZones[rand.Intn(len(managedZones))]
	}
	if framework.TestContext.CloudConfig.Zone == "" && framework.TestContext.CloudConfig.MultiZone {
		zones, err := gceCloud.GetAllZonesFromCloudProvider()
		if err != nil {
			return nil, err
		}

		framework.TestContext.CloudConfig.Zone, _ = zones.PopAny()
	}

	return NewProvider(gceCloud), nil
}

// Provider is a structure to handle GCE clouds for e2e testing
type Provider struct {
	framework.NullProvider
	gceCloud *gcecloud.Cloud
}

// NewProvider returns a cloud provider interface for GCE
func NewProvider(gceCloud *gcecloud.Cloud) framework.ProviderInterface {
	return &Provider{
		gceCloud: gceCloud,
	}
}

// GetGCECloud returns GCE cloud provider
func GetGCECloud() (*gcecloud.Cloud, error) {
	p, ok := framework.TestContext.CloudConfig.Provider.(*Provider)
	if !ok {
		return nil, fmt.Errorf("failed to convert CloudConfig.Provider to GCE provider: %#v", framework.TestContext.CloudConfig.Provider)
	}
	return p.gceCloud, nil
}

// RecreateNodes recreates the given nodes in a managed instance group.
func RecreateNodes(c clientset.Interface, nodes []v1.Node) error {
	// Build mapping from zone to nodes in that zone.
	nodeNamesByZone := make(map[string][]string)
	for i := range nodes {
		node := &nodes[i]

		if zone, ok := node.Labels[v1.LabelFailureDomainBetaZone]; ok {
			nodeNamesByZone[zone] = append(nodeNamesByZone[zone], node.Name)
			continue
		}

		if zone, ok := node.Labels[v1.LabelTopologyZone]; ok {
			nodeNamesByZone[zone] = append(nodeNamesByZone[zone], node.Name)
			continue
		}

		defaultZone := framework.TestContext.CloudConfig.Zone
		nodeNamesByZone[defaultZone] = append(nodeNamesByZone[defaultZone], node.Name)
	}

	// Find the sole managed instance group name
	var instanceGroup string
	if strings.Contains(framework.TestContext.CloudConfig.NodeInstanceGroup, ",") {
		return fmt.Errorf("Test does not support cluster setup with more than one managed instance group: %s", framework.TestContext.CloudConfig.NodeInstanceGroup)
	}
	instanceGroup = framework.TestContext.CloudConfig.NodeInstanceGroup

	// Recreate the nodes.
	for zone, nodeNames := range nodeNamesByZone {
		args := []string{
			"compute",
			fmt.Sprintf("--project=%s", framework.TestContext.CloudConfig.ProjectID),
			"instance-groups",
			"managed",
			"recreate-instances",
			instanceGroup,
		}

		args = append(args, fmt.Sprintf("--instances=%s", strings.Join(nodeNames, ",")))
		args = append(args, fmt.Sprintf("--zone=%s", zone))
		framework.Logf("Recreating instance group %s.", instanceGroup)
		stdout, stderr, err := framework.RunCmd("gcloud", args...)
		if err != nil {
			return fmt.Errorf("error recreating nodes: %s\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
	}
	return nil
}
