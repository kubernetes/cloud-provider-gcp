package ipam

import (
	"fmt"
	"net"
	"strings"

	compute "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
)

// performMultiNetworkCIDRAllocation receives the existing Node object and its
// GCE interfaces, and is updated with the corresponding annotations for
// MultiNetwork and NorthInterfaces and the capacity for the additional networks.
// It also returns calculated cidrs for default Network.
//
// NorthInterfacesAnnotationKey is modified on Network Ready condition changes.
// MultiNetworkAnnotationKey is modified on Node's NodeNetworkAnnotationKey changes.
func (ca *cloudCIDRAllocator) performMultiNetworkCIDRAllocation(node *v1.Node, interfaces []*compute.NetworkInterface) (defaultNwCIDRs []string, err error) {
	northInterfaces := networkv1.NorthInterfacesAnnotation{}
	additionalNodeNetworks := networkv1.MultiNetworkAnnotation{}

	k8sNetworksList, err := ca.networksLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("node=%s error fetching networks: %v", node.Name, err)
	}

	// get networks from Node's network-status annotation
	upStatusNetworks, err := getUpNetworks(node)
	if err != nil {
		return nil, err
	}

	// networks is list of networks that are Ready
	networks := make([]*networkv1.Network, 0)
	// filter networks based only on Ready condition
	// we do not filter Networks with DeletionTimestamp set, because we
	// count on the Network "delete event" for cleanup
	for _, network := range k8sNetworksList {
		if meta.IsStatusConditionTrue(network.Status.Conditions, string(networkv1.NetworkConditionStatusReady)) || networkv1.IsDefaultNetwork(network.Name) {
			networks = append(networks, network)
		}
	}

	processedNetworks := make(map[string]struct{})
	// Fetch the GKENetworkParams for every k8s-network object.
	// Match the fetched GKENetworkParams object with the interfaces on the node
	// to build the per-network north-interface and node-network annotations useful for IPAM.
	for _, inf := range interfaces {
		rangeNameAliasIPMap := map[string]*compute.AliasIpRange{}
		for _, ipRange := range inf.AliasIpRanges {
			rangeNameAliasIPMap[ipRange.SubnetworkRangeName] = ipRange
		}
		for _, network := range networks {
			if _, ok := processedNetworks[network.Name]; ok {
				// skip networks that are already matched with an interface
				continue
			}

			klog.V(4).InfoS("allotting pod CIDRs", "nodeName", node.Name, "networkName", network.Name)
			gnp, err := ca.gnpLister.Get(network.Spec.ParametersRef.Name)
			if err != nil {
				return nil, err
			}
			if resourceName(inf.Network) != resourceName(gnp.Spec.VPC) || resourceName(inf.Subnetwork) != resourceName(gnp.Spec.VPCSubnet) {
				continue
			}
			klog.V(2).InfoS("interface matched, proceeding to find a secondary range", "nodeName", node.Name, "networkInterface", inf.Name)
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
				klog.V(2).InfoS("found an allocatable secondary range for the interface on network", "nodeName", node.Name, "networkName", network.Name)
				processedNetworks[network.Name] = struct{}{}
				if networkv1.IsDefaultNetwork(network.Name) {
					defaultNwCIDRs = append(defaultNwCIDRs, ipRange.IpCidrRange)
					ipv6Addr := ca.cloud.GetIPV6Address(inf)
					if ipv6Addr != nil {
						defaultNwCIDRs = append(defaultNwCIDRs, ipv6Addr.String())
					}
				} else {
					northInterfaces = append(northInterfaces, networkv1.NorthInterface{Network: network.Name, IpAddress: inf.NetworkIP})
					if _, ok := upStatusNetworks[network.Name]; ok {
						additionalNodeNetworks = append(additionalNodeNetworks, networkv1.NodeNetwork{Name: network.Name, Scope: "host-local", Cidrs: []string{ipRange.IpCidrRange}})
					}
				}
				break
			}
		}
	}
	if err = updateAnnotations(node, northInterfaces, additionalNodeNetworks); err != nil {
		return nil, err
	}
	return defaultNwCIDRs, nil
}

func updateAnnotations(node *v1.Node, northInterfaces networkv1.NorthInterfacesAnnotation, additionalNodeNetworks networkv1.MultiNetworkAnnotation) error {
	northInterfaceAnn, err := networkv1.MarshalNorthInterfacesAnnotation(northInterfaces)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the north interfaces annotation for multi-networking", "nodeName", node.Name)
		return err
	}
	additionalNodeNwAnn, err := networkv1.MarshalAnnotation(additionalNodeNetworks)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the additional node networks annotation for multi-networking", "nodeName", node.Name)
		return err
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[networkv1.NorthInterfacesAnnotationKey] = northInterfaceAnn
	capacity, err := allocateIPCapacity(node, additionalNodeNetworks)
	if err != nil {
		return err
	}
	node.Status.Capacity = capacity
	node.Annotations[networkv1.MultiNetworkAnnotationKey] = additionalNodeNwAnn
	return nil
}

// allocateIPCapacity updates the extended IP resource capacity for every non-default network on the node.
func allocateIPCapacity(node *v1.Node, nodeNetworks networkv1.MultiNetworkAnnotation) (v1.ResourceList, error) {
	resourceList := node.Status.Capacity
	if resourceList == nil {
		resourceList = make(v1.ResourceList)
	}
	// Rebuild the IP capacity for all the networks on the node by deleting the existing IP capacities first.
	for name := range resourceList {
		if strings.HasPrefix(name.String(), networkv1.NetworkResourceKeyPrefix) {
			delete(resourceList, name)
		}
	}
	for _, nw := range nodeNetworks {
		ipCount, err := getNodeCapacity(nw)
		if err != nil {
			return nil, err
		}
		resourceList[v1.ResourceName(networkv1.NetworkResourceKeyPrefix+nw.Name+".IP")] = *resource.NewQuantity(ipCount, resource.DecimalSI)
	}
	return resourceList, nil
}

func resourceName(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func getNodeCapacity(nw networkv1.NodeNetwork) (int64, error) {
	if len(nw.Cidrs) < 1 {
		return -1, fmt.Errorf("network %s is missing CIDRs", nw.Name)
	}
	_, ipNet, err := net.ParseCIDR(nw.Cidrs[0])
	if err != nil {
		return -1, err
	}
	var ipCount int64 = 1
	size := netutils.RangeSize(ipNet)
	if size > 1 {
		// The number of IPs supported are halved and returned for overprovisioning purposes.
		ipCount = size >> 1
	}
	return ipCount, nil
}

func getUpNetworks(node *v1.Node) (map[string]struct{}, error) {
	m := make(map[string]struct{})
	if node.Annotations == nil {
		return m, nil
	}
	ann, ok := node.Annotations[networkv1.NodeNetworkAnnotationKey]
	if !ok {
		return m, nil
	}
	nodeNws, err := networkv1.ParseNodeNetworkAnnotation(ann)
	if err != nil {
		return nil, fmt.Errorf("invalid format for multi-network annotation: %v", err)
	}
	for _, n := range nodeNws {
		m[n.Name] = struct{}{}
	}
	return m, nil
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
