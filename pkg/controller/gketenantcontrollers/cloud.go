/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"io"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	gcfg "gopkg.in/gcfg.v1"
	cloudprovider "k8s.io/cloud-provider"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/klog/v2"
)

// TenantCloud is an interface for creating tenant-scoped cloud providers.
type TenantCloud interface {
	CreateTenantScopedGCECloud(config *config.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error)
}

// CreateTenantScopedGCECloud creates a new cloud object customized with
// authentication from a ProviderConfig CR.
func CreateTenantScopedGCECloud(config *config.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error) {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider
	configReader, err := createConfigReader(cloudConfig.CloudConfigFile)
	if err != nil {
		return nil, err
	}

	// Parse the default config file
	configFile := &gce.ConfigFile{}
	if err := gcfg.FatalOnly(gcfg.ReadInto(configFile, configReader)); err != nil {
		klog.Errorf("Couldn't read config: %v", err)
		return nil, err
	}

	// Generate CloudConfig from ConfigFile and modify with ProviderConfig
	gceCloudConfig, err := generateCloudConfig(configFile, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate provider-scoped config: %w", err)
	}

	projectID := pc.Spec.ProjectID
	klog.Infof("Creating new scoped GCECloud for project: %s", projectID)

	cloud, err := gce.CreateGCECloud(gceCloudConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloud provider for project %s: %v", projectID, err)
	}
	klog.Infof("cloud object created for project %s: %+v", projectID, cloud)
	return cloud, nil
}

func createConfigReader(configFilePath string) (io.Reader, error) {
	if configFilePath != "" {
		data, err := os.ReadFile(configFilePath)
		if err != nil {
			klog.Errorf("Couldn't open cloud provider configuration %s: %#v", configFilePath, err)
			return nil, err
		}
		return bytes.NewReader(data), nil
	}

	klog.Warning("Cloud provider configuration file path is empty")
	return nil, fmt.Errorf("cloud provider configuration file path is empty")
}

func generateCloudConfig(configFile *gce.ConfigFile, providerConfig *v1.ProviderConfig) (*gce.CloudConfig, error) {
	if providerConfig == nil || strings.HasPrefix(providerConfig.Name, "s") {
		// For supervisor or nil config, use default cloud config logic without modification.
		return gce.GenerateCloudConfig(configFile)
	}

	gceCloudConfig, err := gce.GenerateCloudConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Override with ProviderConfig values
	gceCloudConfig.ProjectID = providerConfig.Spec.ProjectID
	// NetworkProjectID defaults to ProjectID if unset in configFile.
	// We do not override it with ProviderConfig.Spec.ProjectID to allow NetworkProjectID to differ (e.g. Shared VPC).

	// Update NetworkName
	// e.g. projects/my-project/global/networks/my-network -> my-network logic
	if providerConfig.Spec.NetworkConfig.Network != "" {
		networkName, networkURL := parseResourceURL(providerConfig.Spec.NetworkConfig.Network)
		gceCloudConfig.NetworkName = networkName
		if networkURL != "" {
			gceCloudConfig.NetworkURL = networkURL
		}
	}

	// Update SubnetworkName
	if providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork != "" {
		subnetworkName, subnetworkURL := parseResourceURL(providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork)
		gceCloudConfig.SubnetworkName = subnetworkName
		if subnetworkURL != "" {
			gceCloudConfig.SubnetworkURL = subnetworkURL
		}
	}

	// Update TokenSource
	// Reuse existing TokenURL from config file if present
	existingTokenURL := configFile.Global.TokenURL
	tokenURL, err := tokenURLForProviderConfig(existingTokenURL, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to update TokenURL: %v", err)
	}

	// Reuse/Update TokenBody
	existingTokenBody := configFile.Global.TokenBody
	tokenBody, err := updateTokenProjectNumber(existingTokenBody, int(providerConfig.Spec.ProjectNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to update TokenBody: %v", err)
	}

	if tokenURL != "" {
		if tokenURL == "nil" {
			gceCloudConfig.TokenSource = nil
		} else {
			gceCloudConfig.TokenSource = gce.NewAltTokenSource(tokenURL, tokenBody)
		}
	}

	return gceCloudConfig, nil
}

func tokenURLForProviderConfig(existingTokenURL string, providerConfig *v1.ProviderConfig) (string, error) {
	if _, ok := providerConfig.Labels[tenantLabel]; !ok {
		return existingTokenURL, nil
	}

	location := extractLocationFromTokenURL(existingTokenURL)
	tokenURLParts := strings.SplitN(existingTokenURL, "/projects/", 2)
	if len(tokenURLParts) != 2 {
		return "", fmt.Errorf("invalid token URL format")
	}
	baseURL := tokenURLParts[0]
	formatString := "%s/projects/%d/locations/%s/tenants/%s:generateTenantToken"
	tokenURL := fmt.Sprintf(formatString, baseURL, providerConfig.Spec.ProjectNumber, location, providerConfig.Name)
	klog.Infof("Token URL for %s: %s", providerConfig.Name, tokenURL)
	return tokenURL, nil
}

func extractLocationFromTokenURL(tokenURL string) string {
	parts := strings.Split(tokenURL, "/")
	for i, part := range parts {
		if part == "locations" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// updateTokenProjectNumber returns the updated JSON body as a string.
func updateTokenProjectNumber(tokenBody string, projectNumber int) (string, error) {
	if tokenBody == "" {
		return "", nil // If existing is empty, we return empty.
	}

	// Try to unmarshal.
	var bodyMap map[string]any

	jsonStr := tokenBody
	isQuoted := len(tokenBody) > 0 && tokenBody[0] == '"' && tokenBody[len(tokenBody)-1] == '"'
	if isQuoted {
		var err error
		jsonStr, err = strconv.Unquote(tokenBody)
		if err != nil {
			// If unquote fails, treat as raw string.
			jsonStr = tokenBody
			isQuoted = false
		}
	}

	if err := json.Unmarshal([]byte(jsonStr), &bodyMap); err != nil {
		return "", fmt.Errorf("error unmarshaling TokenBody: %v", err)
	}

	bodyMap["projectNumber"] = projectNumber

	newTokenBodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return "", fmt.Errorf("error marshaling TokenBody: %v", err)
	}

	return string(newTokenBodyBytes), nil
}

func getNodeLabelSelector(cr *v1.ProviderConfig) (string, error) {
	labelValue := cr.Name

	if labelValue == "" {
		return "", fmt.Errorf("ProviderConfig name cannot be empty")
	}

	return labelValue, nil
}

func parseResourceURL(resource string) (name string, url string) {
	name = resource
	if strings.Contains(resource, "/") {
		parts := strings.Split(resource, "/")
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
		url = resource
	}
	return name, url
}
