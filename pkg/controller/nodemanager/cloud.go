/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"io"

	"github.com/go-ini/ini"
	cloudprovider "k8s.io/cloud-provider"
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/klog/v2"
)

func init() {
	// Disable pretty printing for INI files, to match default format of gce.conf.
	ini.PrettyFormat = false
	ini.PrettyEqual = true
	ini.PrettySection = true
}

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
	// Read the default config content from the config reader we created.
	// Then apply provider-scoped modifications (generateConfigForProviderConfig)
	// and pass the modified INI to the gce helper to build a scoped cloud.
	defaultBytes, err := io.ReadAll(configReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read default gce config: %w", err)
	}

	defaultConfigContent := string(defaultBytes)
	modifiedConfigContent, err := generateConfigForProviderConfig(defaultConfigContent, pc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate provider-scoped config: %w", err)
	}

	projectID := pc.Spec.ProjectID
	klog.Infof("Creating new scoped GCECloud for project: %s", projectID)

	cloud, err := gce.CreateGCECloudFromReader(strings.NewReader(modifiedConfigContent))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloud provider for project %s: %v", projectID, err)
	}

	return cloud, nil
}

func createConfigReader(configFilePath string) (io.Reader, error) {
	// TODO: add logic for empty config file path if needed.
	if configFilePath != "" {
		// Read the file into memory and return an in-memory reader. This
		// avoids returning an *os.File that the caller would need to close
		// and also fixes the undeclared 'err' issue.
		data, err := os.ReadFile(configFilePath)
		if err != nil {
			klog.Errorf("Couldn't open cloud provider configuration %s: %#v", configFilePath, err)
			return nil, err
		}
		return bytes.NewReader(data), nil
	} else {
		klog.Warning("Cloud provider configuration file path is empty")
		return nil, fmt.Errorf("cloud provider configuration file path is empty")
	}
}

// TODO: modify this constant byte to string and back by changing signature of this function
func generateConfigForProviderConfig(defaultConfigContent string, providerConfig *v1.ProviderConfig) (string, error) {
	if providerConfig == nil {
		return defaultConfigContent, nil
	}

	// Load the config content into an INI file
	cfg, err := ini.LoadSources(ini.LoadOptions{
		AllowShadows: true, // This allows multiple keys with the same name
	}, []byte(defaultConfigContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse default config content: %w", err)
	}

	globalSection := cfg.Section("global")
	if globalSection == nil {
		return "", fmt.Errorf("global section not found in config")
	}

	// Update ProjectID
	projectIDKey := "project-id"
	globalSection.Key(projectIDKey).SetValue(providerConfig.Spec.ProjectID)

	// Update TokenURL
	tokenURLKey := "token-url"
	oldValue := globalSection.Key(tokenURLKey).String()
	tokenURL, err := tokenURLForProviderConfig(oldValue, providerConfig)
	if err != nil {
		return "", fmt.Errorf("failed to update TokenURL: %v", err)
	}
	globalSection.Key(tokenURLKey).SetValue(tokenURL)

	// Update TokenBody
	tokenBodyKey := "token-body"
	tokenBody := globalSection.Key(tokenBodyKey).String()
	newTokenBody, err := updateTokenProjectNumber(tokenBody, int(providerConfig.Spec.ProjectNumber))
	if err != nil {
		return "", fmt.Errorf("failed to update TokenBody: %v", err)
	}
	globalSection.Key(tokenBodyKey).SetValue(newTokenBody)

	// Update NetworkName and SubnetworkName
	networkNameKey := "network-name"
	// Network name is the last part of the network path
	// e.g. projects/my-project/global/networks/my-network -> my-network
	networkParts := strings.Split(providerConfig.Spec.NetworkConfig.Network, "/")
	networkName := providerConfig.Spec.NetworkConfig.Network
	if len(networkParts) > 1 {
		networkName = networkParts[len(networkParts)-1]
	}
	globalSection.Key(networkNameKey).SetValue(networkName)

	subnetworkNameKey := "subnetwork-name"
	// Subnetwork name is the last part of the subnetwork path
	// e.g. projects/my-project/regions/us-central1/subnetworks/my-subnetwork -> my-subnetwork
	subnetworkParts := strings.Split(providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork, "/")
	subnetworkName := providerConfig.Spec.NetworkConfig.SubnetInfo.Subnetwork
	if len(subnetworkParts) > 1 {
		subnetworkName = subnetworkParts[len(subnetworkParts)-1]
	}
	globalSection.Key(subnetworkNameKey).SetValue(subnetworkName)

	// Write the modified config content to a string with custom options
	var modifiedConfigContent bytes.Buffer
	_, err = cfg.WriteTo(&modifiedConfigContent)
	if err != nil {
		return "", fmt.Errorf("failed to write modified config content: %v", err)
	}

	return modifiedConfigContent.String(), nil
}

func tokenURLForProviderConfig(existingTokenURL string, providerConfig *v1.ProviderConfig) (string, error) {
	if _, ok := providerConfig.Labels[TenantLabel]; !ok {
		// If no owner label is set, assume token URL belongs to the default project.
		return existingTokenURL, nil
	}

	// Extract location from the old token URL
	location := extractLocationFromTokenURL(existingTokenURL)
	// Extract the baseURL before "/projects/"
	tokenURLParts := strings.SplitN(existingTokenURL, "/projects/", 2)
	if len(tokenURLParts) != 2 {
		return "", fmt.Errorf("invalid token URL format")
	}
	baseURL := tokenURLParts[0]
	// Format: {BASE_URL}/projects/{TENANT_PROJECT_NUMBER}/locations/{TENANT_LOCATION}/tenants/{TENANT_ID}:generateTenantToken"
	formatString := "%s/projects/%d/locations/%s/tenants/%s:generateTenantToken"
	tokenURL := fmt.Sprintf(formatString, baseURL, providerConfig.Spec.ProjectNumber, location, providerConfig.Name)
	return tokenURL, nil
}

func updateTokenProjectNumber(tokenBody string, projectNumber int) (string, error) {
	// Check if the token body is a quoted JSON string
	isQuoted := len(tokenBody) > 0 && tokenBody[0] == '"' && tokenBody[len(tokenBody)-1] == '"'

	var jsonStr string
	if isQuoted {
		// Unquote the JSON string
		var err error
		jsonStr, err = strconv.Unquote(tokenBody)
		if err != nil {
			return "", fmt.Errorf("error unquoting TokenBody: %v", err)
		}
	} else {
		jsonStr = tokenBody
	}

	var bodyMap map[string]any

	// Unmarshal the JSON string into a map
	if err := json.Unmarshal([]byte(jsonStr), &bodyMap); err != nil {
		return "", fmt.Errorf("error unmarshaling TokenBody: %v", err)
	}

	// Update the "projectNumber" field with the new value
	bodyMap["projectNumber"] = projectNumber

	// Marshal the map back into a JSON string
	newTokenBodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return "", fmt.Errorf("error marshaling TokenBody: %v", err)
	}

	if isQuoted {
		// Re-quote the JSON string if the original was quoted
		return strconv.Quote(string(newTokenBodyBytes)), nil
	}

	return string(newTokenBodyBytes), nil
}

// extractLocationFromTokenURL extracts the location from a GKE token URL.
// Example input: https://gkeauth.googleapis.com/v1/projects/654321/locations/us-central1/clusters/example-cluster:generateToken
// Returns: us-central1
func extractLocationFromTokenURL(tokenURL string) string {
	parts := strings.Split(tokenURL, "/")
	for i, part := range parts {
		if part == "locations" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// getNodeLabelSelector extracts the node label selector string from the CR.
// This is a placeholder for wherever you define this in your CR.
// **YOU MUST UPDATE THIS**
func getNodeLabelSelector(cr *v1.ProviderConfig) (string, error) {
	// Example: Assuming you added a label selector to your CR's spec.
	// If it's stored in an annotation, get it from there.
	//
	// if cr.Spec.NodeSelector == "" {
	//    return "", fmt.Errorf("ProviderConfig.spec.nodeSelector is empty")
	// }
	// return cr.Spec.NodeSelector, nil

	// For this example, I'll assume you have an annotation:
	// "nodemanager.cloud.google.com/node-selector": "key=value"
	selector, ok := cr.Annotations["nodemanager.cloud.google.com/node-selector"]
	if !ok || selector == "" {
		return "", fmt.Errorf("ProviderConfig %s/%s is missing annotation 'nodemanager.cloud.google.com/node-selector'", cr.Namespace, cr.Name)
	}

	// The Node Controller's informer factory expects a label.Selector,
	// not a field selector.
	if strings.Contains(selector, "!=") || strings.Contains(selector, " in ") {
		klog.Warningf("Selector '%s' may be too complex; simple 'key=value' is safest.", selector)
	}

	return selector, nil
}
