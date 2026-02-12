package nodeipam

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	networkinformer "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	cloudprovider "k8s.io/cloud-provider"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	controllersmetrics "k8s.io/component-base/metrics/prometheus/controllers"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
)

const (
	// defaultNodeMaskCIDRIPv4 is default mask size for IPv4 node cidr
	defaultNodeMaskCIDRIPv4 = 24
	// defaultNodeMaskCIDRIPv6 is default mask size for IPv6 node cidr
	defaultNodeMaskCIDRIPv6 = 64
)

// StartNodeIpamController starts the NodeIPAM controller.
// It returns the controller interface, a boolean indicating if it started, and an error if any.
func StartNodeIpamController(
	ctx context.Context,
	nodeInformer coreinformers.NodeInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	clusterCIDR string,
	allocateNodeCIDRs bool,
	serviceCIDRString string,
	secondaryServiceCIDRString string,
	nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration,
	nwInformer networkinformer.NetworkInformer,
	gnpInformer networkinformer.GKENetworkParamSetInformer,
	nodeTopologyClient nodetopologyclientset.Interface,
	cidrAllocatorType ipam.CIDRAllocatorType,
	controllerManagerMetrics *controllersmetrics.ControllerManagerMetrics,
) (controller.Interface, bool, error) {
	var serviceCIDR *net.IPNet
	var secondaryServiceCIDR *net.IPNet
	var clusterCIDRs []*net.IPNet
	var nodeCIDRMaskSizes []int

	// should we start nodeIPAM
	if !allocateNodeCIDRs {
		return nil, false, fmt.Errorf("the AllocateNodeCIDRs is not enabled")
	}

	// failure: bad cidrs in config
	cidrs, dualStack, err := ProcessCIDRs(clusterCIDR)
	if err != nil {
		return nil, false, err
	}
	clusterCIDRs = cidrs

	// failure: more than one cidr but they are not configured as dual stack
	if len(clusterCIDRs) > 1 && !dualStack {
		return nil, false, fmt.Errorf("len of ClusterCIDRs==%v and they are not configured as dual stack (at least one from each IPFamily", len(clusterCIDRs))
	}

	// failure: more than 2 cidrs is not allowed even with dual stack
	if len(clusterCIDRs) > 2 {
		return nil, false, fmt.Errorf("len of clusters cidrs is:%v > more than max allowed of 2", len(clusterCIDRs))
	}

	// service cidr processing
	if len(strings.TrimSpace(serviceCIDRString)) != 0 {
		_, serviceCIDR, err = net.ParseCIDR(serviceCIDRString)
		if err != nil {
			klog.Warningf("Unsuccessful parsing of service CIDR %v: %v", serviceCIDRString, err)
		}
	}

	if len(strings.TrimSpace(secondaryServiceCIDRString)) != 0 {
		_, secondaryServiceCIDR, err = net.ParseCIDR(secondaryServiceCIDRString)
		if err != nil {
			klog.Warningf("Unsuccessful parsing of service CIDR %v: %v", secondaryServiceCIDRString, err)
		}
	}

	// the following checks are triggered if both serviceCIDR and secondaryServiceCIDR are provided
	if serviceCIDR != nil && secondaryServiceCIDR != nil {
		// should be dual stack (from different IPFamilies)
		dualstackServiceCIDR, err := netutils.IsDualStackCIDRs([]*net.IPNet{serviceCIDR, secondaryServiceCIDR})
		if err != nil {
			return nil, false, fmt.Errorf("failed to perform dualstack check on serviceCIDR and secondaryServiceCIDR error:%v", err)
		}
		if !dualstackServiceCIDR {
			return nil, false, fmt.Errorf("serviceCIDR and secondaryServiceCIDR are not dualstack (from different IPfamiles)")
		}
	}

	// get list of node cidr mask sizes
	nodeCIDRMaskSizes, err = setNodeCIDRMaskSizes(nodeIPAMConfig, clusterCIDRs)
	if err != nil {
		return nil, false, err
	}

	nodeIpamController, err := NewNodeIpamController(
		nodeInformer,
		cloud,
		kubeClient,
		nwInformer,
		gnpInformer,
		nodeTopologyClient,
		nodeIPAMConfig.EnableMultiSubnetCluster,
		nodeIPAMConfig.EnableMultiNetworking,
		clusterCIDRs,
		serviceCIDR,
		secondaryServiceCIDR,
		nodeCIDRMaskSizes,
		cidrAllocatorType,
	)
	if err != nil {
		return nil, false, err
	}

	go nodeIpamController.Run(ctx.Done(), controllerManagerMetrics)

	return nil, true, nil
}

// ProcessCIDRs is a helper function that works on a comma separated cidrs and returns
// a list of typed cidrs
// a flag if cidrs represents a dual stack
// error if failed to parse any of the cidrs
func ProcessCIDRs(cidrsList string) ([]*net.IPNet, bool, error) {
	cidrsSplit := strings.Split(strings.TrimSpace(cidrsList), ",")

	cidrs, err := netutils.ParseCIDRs(cidrsSplit)
	if err != nil {
		return nil, false, err
	}

	// if cidrs has an error then the previous call will fail
	// safe to ignore error checking on next call
	dualstack, _ := netutils.IsDualStackCIDRs(cidrs)

	return cidrs, dualstack, nil
}

// setNodeCIDRMaskSizes returns the IPv4 and IPv6 node cidr mask sizes to the value provided
// for --node-cidr-mask-size-ipv4 and --node-cidr-mask-size-ipv6 respectively. If value not provided,
// then it will return default IPv4 and IPv6 cidr mask sizes.
func setNodeCIDRMaskSizes(cfg nodeipamconfig.NodeIPAMControllerConfiguration, clusterCIDRs []*net.IPNet) ([]int, error) {

	sortedSizes := func(maskSizeIPv4, maskSizeIPv6 int) []int {
		nodeMaskCIDRs := make([]int, len(clusterCIDRs))

		for idx, clusterCIDR := range clusterCIDRs {
			if netutils.IsIPv6CIDR(clusterCIDR) {
				nodeMaskCIDRs[idx] = maskSizeIPv6
			} else {
				nodeMaskCIDRs[idx] = maskSizeIPv4
			}
		}
		return nodeMaskCIDRs
	}

	// --node-cidr-mask-size flag is incompatible with dual stack clusters.
	ipv4Mask, ipv6Mask := defaultNodeMaskCIDRIPv4, defaultNodeMaskCIDRIPv6
	isDualstack := len(clusterCIDRs) > 1

	// case one: cluster is dualstack (i.e, more than one cidr)
	if isDualstack {
		// if --node-cidr-mask-size then fail, user must configure the correct dual-stack mask sizes (or use default)
		if cfg.NodeCIDRMaskSize != 0 {
			return nil, errors.New("usage of --node-cidr-mask-size is not allowed with dual-stack clusters")
		}

		if cfg.NodeCIDRMaskSizeIPv4 != 0 {
			ipv4Mask = int(cfg.NodeCIDRMaskSizeIPv4)
		}
		if cfg.NodeCIDRMaskSizeIPv6 != 0 {
			ipv6Mask = int(cfg.NodeCIDRMaskSizeIPv6)
		}
		return sortedSizes(ipv4Mask, ipv6Mask), nil
	}

	maskConfigured := cfg.NodeCIDRMaskSize != 0
	maskV4Configured := cfg.NodeCIDRMaskSizeIPv4 != 0
	maskV6Configured := cfg.NodeCIDRMaskSizeIPv6 != 0
	isSingleStackIPv6 := netutils.IsIPv6CIDR(clusterCIDRs[0])

	// original flag is set
	if maskConfigured {
		// original mask flag is still the main reference.
		if maskV4Configured || maskV6Configured {
			return nil, errors.New("usage of --node-cidr-mask-size-ipv4 and --node-cidr-mask-size-ipv6 is not allowed if --node-cidr-mask-size is set. For dual-stack clusters please unset it and use IPFamily specific flags")
		}

		mask := int(cfg.NodeCIDRMaskSize)
		return sortedSizes(mask, mask), nil
	}

	if maskV4Configured {
		if isSingleStackIPv6 {
			klog.Info("--node-cidr-mask-size-ipv4 should not be used for a single-stack IPv6 cluster")
		}

		ipv4Mask = int(cfg.NodeCIDRMaskSizeIPv4)
	}

	// !maskV4Configured && !maskConfigured && maskV6Configured
	if maskV6Configured {
		if !isSingleStackIPv6 {
			klog.Info("--node-cidr-mask-size-ipv6 should not be used for a single-stack IPv4 cluster")
		}

		ipv6Mask = int(cfg.NodeCIDRMaskSizeIPv6)
	}
	return sortedSizes(ipv4Mask, ipv6Mask), nil
}
