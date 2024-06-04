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

	"google.golang.org/api/googleapi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	gcecloud "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
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

// GetGCECloud returns GCE cloud provider
func GetGCECloud() (*gcecloud.Cloud, error) {
	p, ok := framework.TestContext.CloudConfig.Provider.(*Provider)
	if !ok {
		return nil, fmt.Errorf("failed to convert CloudConfig.Provider to GCE provider: %#v", framework.TestContext.CloudConfig.Provider)
	}
	return p.gceCloud, nil
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

// CreatePD creates a persistent volume
func (p *Provider) CreatePD(zone string) (string, error) {
	pdName := fmt.Sprintf("%s-%s", framework.TestContext.Prefix, string(uuid.NewUUID()))

	if zone == "" && framework.TestContext.CloudConfig.MultiZone {
		zones, err := p.gceCloud.GetAllZonesFromCloudProvider()
		if err != nil {
			return "", err
		}
		zone, _ = zones.PopAny()
	}

	tags := map[string]string{}
	if _, err := p.gceCloud.CreateDisk(pdName, gcecloud.DiskTypeStandard, zone, 2 /* sizeGb */, tags); err != nil {
		return "", err
	}
	return pdName, nil
}

// DeletePD deletes a persistent volume
func (p *Provider) DeletePD(pdName string) error {
	err := p.gceCloud.DeleteDisk(pdName)

	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok && len(gerr.Errors) > 0 && gerr.Errors[0].Reason == "notFound" {
			// PD already exists, ignore error.
			return nil
		}

		framework.Logf("error deleting PD %q: %v", pdName, err)
	}
	return err
}

// CreatePVSource creates a persistent volume source
func (p *Provider) CreatePVSource(ctx context.Context, zone, diskName string) (*v1.PersistentVolumeSource, error) {
	return &v1.PersistentVolumeSource{
		GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{
			PDName:   diskName,
			FSType:   "ext3",
			ReadOnly: false,
		},
	}, nil
}

// DeletePVSource deletes a persistent volume source
func (p *Provider) DeletePVSource(ctx context.Context, pvSource *v1.PersistentVolumeSource) error {
	return e2epv.DeletePDWithRetry(ctx, pvSource.GCEPersistentDisk.PDName)
}
