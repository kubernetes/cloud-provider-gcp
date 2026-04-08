/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"gopkg.in/gcfg.v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers/utils"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/klog/v2"
)

var (
	// regex patterns for validation
	// Note: the network & subnetwork regex is more open than what is actually possible.
	networkURLRegex      = regexp.MustCompile(`^projects/[^/]+/global/networks/[^/]+$`)
	subnetworkURLRegex   = regexp.MustCompile(`^projects/[^/]+/regions/[^/]+/subnetworks/[^/]+$`)
	tenantTokenURLRegex  = regexp.MustCompile(`^https://(gkeauth\.googleapis\.com|[a-zA-Z0-9-]*gkeauth\.sandbox\.googleapis\.com)(?:/v[^/]+)?/projects/[0-9]+/locations/[^/]+/tenants/[^/]+:generateTenantToken$`)
	clusterTokenURLRegex = regexp.MustCompile(`^https://(gkeauth\.googleapis\.com|[a-zA-Z0-9-]*gkeauth\.sandbox\.googleapis\.com)(?:/v[^/]+)?/projects/[0-9]+/locations/[^/]+/clusters/[^/]+:generateToken$`)
)

// TenantCloud is an interface for creating tenant-scoped cloud providers.
type TenantCloud interface {
	CreateTenantScopedGCECloud(config *config.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error)
}

// CreateTenantScopedGCECloud creates a new cloud object customized with authentication and network from a ProviderConfig CR.
func CreateTenantScopedGCECloud(config *config.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error) {
	//validate provider config network and auth values
	if err := validateConfig(pc); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// create config reader from cloud config file
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider
	configReader, err := createConfigReader(cloudConfig.CloudConfigFile)
	if err != nil {
		return nil, err
	}

	// Parse the default config file
	configFile, err := parseConfigFile(configReader)
	if err != nil {
		klog.Errorf("failed to parse config: %v", err)
		return nil, err
	}

	// Generate CloudConfig from ConfigFile and modify with ProviderConfig Network, Subnetwork & AuthConfig
	gceCloudConfig, err := generateCloudConfig(configFile, pc)
	if err != nil {
		klog.Errorf("failed to generate provider-scoped config: %v", err)
		return nil, err
	}

	projectID := pc.Spec.ProjectID
	klog.Infof("Creating new scoped GCECloud for project: %s", projectID)
	cloud, err := gce.CreateGCECloud(gceCloudConfig)
	if err != nil {
		klog.Errorf("failed to initialize scoped cloud provider for project %s: %v", projectID, err)
		return nil, err
	}
	klog.Infof("cloud object created for project %s: %+v", projectID, cloud)
	return cloud, nil
}

// validate config does a regex check on network and subnetwork urls
// It also validates the token url based on whether the config is for a tenant or a supervisor
func validateConfig(pc *v1.ProviderConfig) error {
	if err := validateField("Network URL", pc.Spec.NetworkConfig.Network, networkURLRegex); err != nil {
		klog.Errorf("invalid network url: %v", err)
		return err
	}
	if err := validateField("Subnetwork URL", pc.Spec.NetworkConfig.SubnetInfo.Subnetwork, subnetworkURLRegex); err != nil {
		klog.Errorf("invalid subnetwork url: %v", err)
		return err
	}
	return validateTokenURL(pc)
}

func validateTokenURL(pc *v1.ProviderConfig) error {
	if pc.Spec.AuthConfig == nil {
		err := fmt.Errorf("invalid config: AuthConfig is nil")
		klog.Errorf("Error: %v", err)
		return err
	}
	if utils.IsSupervisor(pc) && !clusterTokenURLRegex.MatchString(pc.Spec.AuthConfig.TokenURL) {
		err := fmt.Errorf("invalid Cluster Token URL format for supervisor: %s", pc.Spec.AuthConfig.TokenURL)
		klog.Errorf("Error: %v", err)
		return err
	} else if !utils.IsSupervisor(pc) && !tenantTokenURLRegex.MatchString(pc.Spec.AuthConfig.TokenURL) {
		err := fmt.Errorf("invalid Tenant Token URL format for tenant: %s", pc.Spec.AuthConfig.TokenURL)
		klog.Errorf("Error: %v", err)
		return err
	}
	return nil
}

func createConfigReader(configFilePath string) (io.Reader, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		klog.Errorf("Couldn't open cloud provider configuration %s: %#v", configFilePath, err)
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func parseConfigFile(configReader io.Reader) (*gce.ConfigFile, error) {
	configFile := &gce.ConfigFile{}
	if err := gcfg.FatalOnly(gcfg.ReadInto(configFile, configReader)); err != nil {
		return nil, err
	}
	return configFile, nil
}

func generateCloudConfig(configFile *gce.ConfigFile, providerConfig *v1.ProviderConfig) (*gce.CloudConfig, error) {
	// For supervisor config, use default cloud config logic without modification.
	if utils.IsSupervisor(providerConfig) {
		klog.Infof("Supervisor config detected, using default cloud config logic")
		return gce.GenerateCloudConfig(configFile)
	}

	gceCloudConfig, err := gce.GenerateCloudConfig(configFile)
	if err != nil {
		klog.Errorf("failed to generate cloud config: %v", err)
		return nil, err
	}

	// Override with ProviderConfig values
	gceCloudConfig.ProjectID = providerConfig.Spec.ProjectID
	setNetworkConfig(gceCloudConfig, providerConfig.Spec.NetworkConfig.Network)
	setSubnetworkConfig(gceCloudConfig, providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork)
	gceCloudConfig.TokenSource = gce.NewAltTokenSource(providerConfig.Spec.AuthConfig.TokenURL, providerConfig.Spec.AuthConfig.TokenBody)
	return gceCloudConfig, nil
}

func setNetworkConfig(c *gce.CloudConfig, network string) error {
	resourceID, err := cloud.ParseResourceURL(network)
	if err != nil {
		klog.Errorf("failed to parse network url: %v", err)
		return err
	}
	c.NetworkName = resourceID.Key.Name
	c.NetworkURL = network
	c.NetworkProjectID = resourceID.ProjectID
	return nil
}

func setSubnetworkConfig(c *gce.CloudConfig, subnetwork string) error {
	resourceID, err := cloud.ParseResourceURL(subnetwork)
	if err != nil {
		klog.Errorf("failed to parse subnetwork url: %v", err)
		return err
	}
	c.SubnetworkName = resourceID.Key.Name
	c.SubnetworkURL = subnetwork
	return nil
}

// validateField validates a string value against a given regular expression pattern.
func validateField(name, value string, pattern *regexp.Regexp) error {
	if value != "" && !pattern.MatchString(value) {
		return fmt.Errorf("invalid %s format: %s", name, value)
	}
	return nil
}
