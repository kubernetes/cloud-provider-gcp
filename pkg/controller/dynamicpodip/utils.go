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

// transientANEError is returned by a controller's syncNode when one or more
// AliasNetworkEndpoints on the node are in a transient (non-ACTIVE) state
// such as CREATING or DELETING. Callers requeue the key with backoff instead
// of treating it as a controller error.
type transientANEError struct {
	nodeName string
}

func (e *transientANEError) Error() string {
	return fmt.Sprintf("node %q has ANEs in transient state, deferring sync", e.nodeName)
}

// resolveGCENetworkURL returns the GCE resource URL that identifies, on this
// node, the interface carrying the Kubernetes network netName. The result is
// what UpdateInstanceAliasIPRanges matches against: the interface's Network
// URL, or its NetworkAttachment URL for PSC interfaces.
//
// If netName resolves to a Network + GKENetworkParamSet, the node's interfaces
// are searched for one matching the GNP's NetworkAttachment or (VPC,
// VPCSubnet), and that interface's own URL is returned so that Shared VPC and
// short-vs-full-name differences between the GNP and GCE never matter.
//
// The primary network ("default" or empty) additionally falls back to the
// cluster's NetworkURL() when it has no Network/GNP (Adaptive Cluster IPAM
// without multi-networking) or when no interface matches its GNP. The latter
// happens on nodes in a node pool subnet other than the cluster default
// subnet: the default GNP names only the cluster subnet, but nic0 is still the
// default network's interface.
func resolveGCENetworkURL(gceCloud *gce.Cloud, resolver *NetworkResolver, netName string, ifaces []*networkInterface) (string, error) {
	if gceCloud == nil {
		return "", fmt.Errorf("GCE cloud provider is nil")
	}
	if netName == "" {
		netName = networkv1.DefaultPodNetworkName
	}

	params, known := resolver.LookupNetworkParams(netName)
	if known && !params.Ready {
		return "", fmt.Errorf("network %q is not Ready", netName)
	}
	if known {
		if iface := findMatchingInterface(params, ifaces); iface != nil {
			if params.NetworkAttachment != "" {
				return iface.NetworkAttachment, nil
			}
			return iface.Network, nil
		}
	}

	if netName == networkv1.DefaultPodNetworkName {
		url := gceCloud.NetworkURL()
		if url == "" {
			return "", fmt.Errorf("cluster network URL is not configured on GCE cloud provider")
		}
		return url, nil
	}
	if known {
		return "", fmt.Errorf("no interface on node matches network %q (vpc=%q, subnet=%q, attachment=%q)",
			netName, params.VPC, params.VPCSubnet, params.NetworkAttachment)
	}
	return "", fmt.Errorf("unsupported network %q: no matching Network and GKENetworkParamSet found", netName)
}

// findMatchingInterface returns the first interface matching params on
// NetworkAttachment, or on (VPC, VPCSubnet) when params has no attachment.
// Resource names are compared by their last path component, as the
// multi-network CIDR allocator does.
func findMatchingInterface(params NetworkParams, ifaces []*networkInterface) *networkInterface {
	if params.NetworkAttachment != "" {
		want := canonicalSubnetwork(params.NetworkAttachment)
		for _, iface := range ifaces {
			if iface != nil && iface.NetworkAttachment != "" && canonicalSubnetwork(iface.NetworkAttachment) == want {
				return iface
			}
		}
		return nil
	}
	vpc, subnet := canonicalSubnetwork(params.VPC), canonicalSubnetwork(params.VPCSubnet)
	if vpc == "" || subnet == "" {
		return nil
	}
	for _, iface := range ifaces {
		if iface != nil && canonicalSubnetwork(iface.Network) == vpc && canonicalSubnetwork(iface.Subnetwork) == subnet {
			return iface
		}
	}
	return nil
}

// resolveKubernetesNetworkName returns the Kubernetes network that an alias IP
// range on iface belongs to, given the range's SubnetworkRangeName.
//
// Resolution mirrors the multi-network CIDR allocator:
//  1. A PSC interface (NetworkAttachment set) belongs to the Network whose GNP
//     names that attachment.
//  2. Otherwise the range belongs to the Network whose GNP covers
//     (iface.Network, iface.Subnetwork) and lists subnetworkRangeName in
//     PodIPv4Ranges. This is what lets several Networks share one NIC, and an
//     additional Network share nic0 with the default network.
//  3. Anything left on nic0 that no Network claims is the default network.
//     This covers Adaptive Cluster IPAM without multi-networking, where no
//     Network CRs exist, and default-network ranges whose names are not (yet)
//     in the default GNP.
//
// A range claimed by a Network that is not Ready is an error rather than
// falling through to "default": it must not be counted or published anywhere
// until the Network is routable. Likewise a range on a secondary interface
// that no Network claims. forEachAliasRange skips both. A range with no
// SubnetworkRangeName comes from the subnet's primary range, which no Network
// can claim.
func resolveKubernetesNetworkName(resolver *NetworkResolver, iface *networkInterface, subnetworkRangeName string) (string, error) {
	if iface == nil || iface.Name == "" {
		return "", fmt.Errorf("unsupported interface with empty name")
	}

	var netName string
	var ready, claimed bool
	if iface.NetworkAttachment != "" {
		netName, ready, claimed = resolver.networkByAttachment(iface.NetworkAttachment)
	} else {
		netName, ready, claimed = resolver.networkByVPCSubnetRange(iface.Network, iface.Subnetwork, subnetworkRangeName)
	}
	if claimed {
		if !ready {
			return "", fmt.Errorf("network %q is not Ready", netName)
		}
		return netName, nil
	}

	if iface.Name == "nic0" {
		return networkv1.DefaultPodNetworkName, nil
	}
	return "", fmt.Errorf("no Kubernetes Network claims interface %q (network=%q, subnetwork=%q, attachment=%q, rangeName=%q)",
		iface.Name, iface.Network, iface.Subnetwork, iface.NetworkAttachment, subnetworkRangeName)
}

// canonicalSubnetwork returns the subnetwork resource name (the last path
// component) or an empty string if empty.
func canonicalSubnetwork(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return lastComponent(s)
}

// lastComponent returns the last component of a URL or path, i.e. anything
// after the last slash. If there is no slash, returns the whole string.
func lastComponent(s string) string {
	lastSlash := strings.LastIndex(s, "/")
	if lastSlash != -1 {
		s = s[lastSlash+1:]
	}
	return s
}
