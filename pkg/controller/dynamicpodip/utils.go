/*
Copyright 2026 The Kubernetes Authors.

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

package dynamicpodip

import (
	"fmt"
	"strings"

	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

// ResolveNetworkURL converts a network name to a GCE network URL.
// Returns an error if netName is empty or gceCloud is nil.
func ResolveNetworkURL(gceCloud *gce.Cloud, netName string) (string, error) {
	if gceCloud == nil {
		return "", fmt.Errorf("GCE cloud provider is nil")
	}
	if netName == "" {
		return "", fmt.Errorf("network name cannot be empty")
	}
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", gceCloud.ProjectID(), netName), nil
}

// ExtractNetworkName extracts the network resource name from a full GCE network URL.
// Returns an error if networkURL is empty.
func ExtractNetworkName(networkURL string) (string, error) {
	if networkURL == "" {
		return "", fmt.Errorf("network URL cannot be empty")
	}
	parts := strings.Split(networkURL, "/")
	name := parts[len(parts)-1]
	if name == "" {
		return "", fmt.Errorf("invalid network URL %q", networkURL)
	}
	return name, nil
}
