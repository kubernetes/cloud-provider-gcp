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

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	"k8s.io/cloud-provider-gcp/providers/gce"
)

// nodeNotReadyError is returned by a controller's syncNode when the Node backing a
// NodeNetworkConfig has not registered yet, or has registered but does not yet have a ProviderID.
//
// This is an expected, transient condition while a node is bootstrapping, so callers requeue the
// key with backoff instead of treating it as a controller error. Requeuing matters because neither
// controller watches Nodes: if the key were simply forgotten, nothing would ever reconcile that
// node again and its NodeNetworkConfig would stay unpopulated indefinitely.
type nodeNotReadyError struct {
	nodeName string
	reason   string
}

func (e *nodeNotReadyError) Error() string {
	return fmt.Sprintf("node %q is not ready for reconciliation: %s", e.nodeName, e.reason)
}

// resolveGCENetworkURL converts a Kubernetes logical network name into a full GCE network URL.
// Currently, only the primary network ("default" or empty) is supported, which resolves to the
// cluster's primary NetworkURL().
//
// TODO: Support Multi-Network (MN) by resolving secondary networks via Network and
// GKENetworkParamSet (GNP) informers instead of returning an error.
func resolveGCENetworkURL(gceCloud *gce.Cloud, netName string) (string, error) {
	if gceCloud == nil {
		return "", fmt.Errorf("GCE cloud provider is nil")
	}
	if netName == "" || netName == networkv1.DefaultPodNetworkName {
		url := gceCloud.NetworkURL()
		if url == "" {
			return "", fmt.Errorf("cluster network URL is not configured on GCE cloud provider")
		}
		return url, nil
	}
	return "", fmt.Errorf("unsupported network %q: only %q is currently supported", netName, networkv1.DefaultPodNetworkName)
}

// resolveKubernetesNetworkName returns the Kubernetes network name for a GCE network interface.
// Currently, only the primary interface (nic0) is supported, which maps to "default".
//
// TODO: Support Multi-Network (MN) by matching secondary interfaces (nic1+) against
// Network and GKENetworkParamSet (GNP) informers instead of returning an error.
func resolveKubernetesNetworkName(iface *networkInterface) (string, error) {
	if iface.Name == "nic0" {
		return networkv1.DefaultPodNetworkName, nil
	}
	return "", fmt.Errorf("unsupported interface %q: only primary interface (nic0) is currently supported", iface.Name)
}
