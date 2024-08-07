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
	"context"
	"fmt"
	"math/rand"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// IsGoogleAPIHTTPErrorCode returns true if the error is a google api
// error matching the corresponding HTTP error code.
func IsGoogleAPIHTTPErrorCode(err error, code int) bool {
	apiErr, ok := err.(*googleapi.Error)
	return ok && apiErr.Code == code
}

// GetInstanceTags gets tags from GCE instance with given name.
func GetInstanceTags(cloudConfig framework.CloudConfig, instanceName string) *compute.Tags {
	gceCloud := cloudConfig.Provider.(*Provider).gceCloud
	res, err := gceCloud.ComputeServices().GA.Instances.Get(cloudConfig.ProjectID, cloudConfig.Zone,
		instanceName).Do()
	if err != nil {
		framework.Failf("Failed to get instance tags for %v: %v", instanceName, err)
	}
	return res.Tags
}

// SetInstanceTags sets tags on GCE instance with given name.
func SetInstanceTags(cloudConfig framework.CloudConfig, instanceName, zone string, tags []string) []string {
	gceCloud := cloudConfig.Provider.(*Provider).gceCloud
	// Re-get instance everytime because we need the latest fingerprint for updating metadata
	resTags := GetInstanceTags(cloudConfig, instanceName)
	_, err := gceCloud.ComputeServices().GA.Instances.SetTags(
		cloudConfig.ProjectID, zone, instanceName,
		&compute.Tags{Fingerprint: resTags.Fingerprint, Items: tags}).Do()
	if err != nil {
		framework.Failf("failed to set instance tags: %v", err)
	}
	framework.Logf("Sent request to set tags %v on instance: %v", tags, instanceName)
	return resTags.Items
}

// GetClusterID returns cluster ID
func GetClusterID(ctx context.Context, c clientset.Interface) (string, error) {
	cm, err := c.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, gcecloud.UIDConfigMapName, metav1.GetOptions{})
	if err != nil || cm == nil {
		return "", fmt.Errorf("error getting cluster ID: %w", err)
	}
	clusterID, clusterIDExists := cm.Data[gcecloud.UIDCluster]
	providerID, providerIDExists := cm.Data[gcecloud.UIDProvider]
	if !clusterIDExists {
		return "", fmt.Errorf("cluster ID not set")
	}
	if providerIDExists {
		return providerID, nil
	}
	return clusterID, nil
}
