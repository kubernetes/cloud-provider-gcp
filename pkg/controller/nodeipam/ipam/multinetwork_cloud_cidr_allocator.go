package ipam

import (
	"fmt"
	"strings"

	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/klog/v2"
)

// PerformMultiNetworkCIDRAllocation allots pod CIDRs for all the networks that a node is connected to. It handles IPv6 only for default-network for now.
func (ca *cloudCIDRAllocator) PerformMultiNetworkCIDRAllocation(node *v1.Node, interfaces []*compute.NetworkInterface) (defaultNwCIDRs []string, northInterfaces networkv1.NorthInterfacesAnnotation, additionalNodeNetworks networkv1.MultiNetworkAnnotation, err error) {
	k8sNetworksList, err := ca.networksLister.List(labels.Everything())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error fetching networks: %v", err)
	}
	networks := make([]*networkv1.Network, 0)
	// ignore networks that are under deletion.
	for _, network := range k8sNetworksList {
		if network.DeletionTimestamp.IsZero() {
			networks = append(networks, network)
		}
	}
	// Fetch the GKENetworkParams for every k8s-network object.
	// Match the fetched GKENetworkParams object with the interfaces on the node
	// to build the per-network north-interface and node-network annotations useful for IPAM.
	for _, inf := range interfaces {
		rangeNameAliasIPMap := map[string]*compute.AliasIpRange{}
		for _, ipRange := range inf.AliasIpRanges {
			rangeNameAliasIPMap[ipRange.SubnetworkRangeName] = ipRange
		}
		for _, network := range networks {
			klog.V(4).Infof("allotting pod cidrs for network %s", network.Name)
			gnp, err := ca.gnpLister.Get(network.Spec.ParametersRef.Name)
			if err != nil {
				return nil, nil, nil, err
			}
			if resourceName(inf.Network) != resourceName(gnp.Spec.VPC) || resourceName(inf.Subnetwork) != resourceName(gnp.Spec.VPCSubnet) {
				continue
			}
			klog.V(2).Infof("interface %s matched, proceeding to find a secondary range", inf.Name)
			// TODO: Handle IPv6 in future.
			var secondaryRangeNames []string
			if gnp.Spec.PodIPv4Ranges != nil {
				secondaryRangeNames = gnp.Spec.PodIPv4Ranges.RangeNames
			}
			// In case of host networking, the node interfaces do not have the secondary ranges. We still need to update the
			// north-interface information on the node.
			if len(secondaryRangeNames) == 0 && !networkv1.IsDefaultNetwork(network.Name) {
				northInterfaces = append(northInterfaces, networkv1.NorthInterface{Network: network.Name, IpAddress: inf.NetworkIP})
			}
			// Each secondary range in a subnet corresponds to a pod-network. AliasIPRanges list on a node interface consists of IP ranges that belong to multiple secondary ranges (pod-networks).
			// Match the secondary range names of interface and GKENetworkParams and set the right IpCidrRange for current network.
			for _, secondaryRangeName := range secondaryRangeNames {
				ipRange, ok := rangeNameAliasIPMap[secondaryRangeName]
				if !ok {
					continue
				}
				klog.V(2).Infof("found an allocatable secondary range for the interface on network")
				if networkv1.IsDefaultNetwork(network.Name) {
					defaultNwCIDRs = append(defaultNwCIDRs, ipRange.IpCidrRange)
					ipv6Addr := ca.cloud.GetIPV6Address(inf)
					if ipv6Addr != nil {
						defaultNwCIDRs = append(defaultNwCIDRs, ipv6Addr.String())
					}
				} else {
					northInterfaces = append(northInterfaces, networkv1.NorthInterface{Network: network.Name, IpAddress: inf.NetworkIP})
					additionalNodeNetworks = append(additionalNodeNetworks, networkv1.NodeNetwork{Name: network.Name, Scope: "host-local", Cidrs: []string{ipRange.IpCidrRange}})
				}
				break
			}
		}
	}
	return defaultNwCIDRs, northInterfaces, additionalNodeNetworks, nil
}

func resourceName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func (ca *cloudCIDRAllocator) NetworkToNodes(network *networkv1.Network) error {
	k8sNodesList, err := ca.nodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error fetching nodes: %v", err)
	}
	// Need to reschedule all Nodes, since we do not know which one holds the
	// new Network.
	for _, node := range k8sNodesList {
		if network != nil {
			// filter out nodes that are not part of network
			if node.Annotations == nil {
				// skip node w/o any annotation, not possible for it to have any MN
				continue
			}
			northIntfAnn, ok := node.Annotations[networkv1.NorthInterfacesAnnotationKey]
			if !ok {
				// skip node w/o "north-interfaces" annotation, no MN
				continue
			}
			northIntf, err := networkv1.ParseNorthInterfacesAnnotation(northIntfAnn)
			// if err!=nil means there is some format issue with the annotation, lets
			// re-generate it for that node
			if err == nil {
				found := false
				for _, ele := range northIntf {
					if ele.Network == network.Name {
						found = true
						break
					}
				}
				if !found {
					// node is not part of this network
					continue
				}
			}
		}
		_ = ca.AllocateOrOccupyCIDR(node)
	}
	return nil
}
