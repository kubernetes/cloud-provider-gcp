/*
Copyright 2018 The Kubernetes Authors.

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

// This file holds the code related with the sample nodeipamcontroller
// which demonstrates how cloud providers add external controllers to cloud-controller-manager
// This file  is copied from k8s.io/kubernetes/cmd/cloud-controller-manager/nodeipamcontroller.go@v1.22

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	cloudprovider "k8s.io/cloud-provider"
	nodeipamcontrolleroptions "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options"
	nodeipamcontroller "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
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

type nodeIPAMController struct {
	nodeIPAMControllerConfiguration nodeipamconfig.NodeIPAMControllerConfiguration
	nodeIPAMControllerOptions       nodeipamcontrolleroptions.NodeIPAMControllerOptions
}

func (nodeIpamController *nodeIPAMController) startNodeIpamControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	errors := nodeIpamController.nodeIPAMControllerOptions.Validate()
	if len(errors) > 0 {
		klog.Fatal("NodeIPAM controller values are not properly set.")
	}
	nodeIpamController.nodeIPAMControllerOptions.ApplyTo(&nodeIpamController.nodeIPAMControllerConfiguration)

	return func(ctx context.Context, controllerContext genericcontrollermanager.ControllerContext) (controller.Interface, bool, error) {
		return startNodeIpamController(completedConfig, nodeIpamController.nodeIPAMControllerConfiguration, controllerContext, cloud)
	}
}

func startNodeIpamController(ccmConfig *cloudcontrollerconfig.CompletedConfig, nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration, ctx genericcontrollermanager.ControllerContext, cloud cloudprovider.Interface) (controller.Interface, bool, error) {
	var serviceCIDR *net.IPNet
	var secondaryServiceCIDR *net.IPNet
	var clusterCIDRs []*net.IPNet
	var nodeCIDRMaskSizes []int

	// should we start nodeIPAM
	if !ccmConfig.ComponentConfig.KubeCloudShared.AllocateNodeCIDRs {
		return nil, false, fmt.Errorf("the AllocateNodeCIDRs is not enabled")
	}

	// failure: bad cidrs in config
	cidrs, dualStack, err := processCIDRs(ccmConfig.ComponentConfig.KubeCloudShared.ClusterCIDR)
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
	if len(strings.TrimSpace(nodeIPAMConfig.ServiceCIDR)) != 0 {
		_, serviceCIDR, err = net.ParseCIDR(nodeIPAMConfig.ServiceCIDR)
		if err != nil {
			klog.Warningf("Unsuccessful parsing of service CIDR %v: %v", nodeIPAMConfig.ServiceCIDR, err)
		}
	}

	if len(strings.TrimSpace(nodeIPAMConfig.SecondaryServiceCIDR)) != 0 {
		_, secondaryServiceCIDR, err = net.ParseCIDR(nodeIPAMConfig.SecondaryServiceCIDR)
		if err != nil {
			klog.Warningf("Unsuccessful parsing of service CIDR %v: %v", nodeIPAMConfig.SecondaryServiceCIDR, err)
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

	kubeConfig := ccmConfig.Complete().Kubeconfig
	kubeConfig.ContentType = "application/json" // required to serialize Networks to json
	networkClient, err := networkclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, false, err
	}
	nodeTopologyClient, err := nodetopologyclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, false, err
	}
	nwInfFactory := networkinformers.NewSharedInformerFactory(networkClient, 30*time.Second)
	nwInformer := nwInfFactory.Networking().V1().Networks()
	gnpInformer := nwInfFactory.Networking().V1().GKENetworkParamSets()
	nodeIpamController, err := nodeipamcontroller.NewNodeIpamController(
		ccmConfig.SharedInformers.Core().V1().Nodes(),
		cloud,
		ccmConfig.ClientBuilder.ClientOrDie("node-controller"),
		nwInformer,
		gnpInformer,
		nodeTopologyClient,
		nodeIPAMConfig.EnableMultiSubnetCluster,
		clusterCIDRs,
		serviceCIDR,
		secondaryServiceCIDR,
		nodeCIDRMaskSizes,
		ipam.CIDRAllocatorType(ccmConfig.ComponentConfig.KubeCloudShared.CIDRAllocatorType),
	)
	if err != nil {
		return nil, false, err
	}
	nwInfFactory.Start(ctx.Stop)
	go nodeIpamController.Run(ctx.Stop, ctx.ControllerManagerMetrics)
	return nil, true, nil
}

// processCIDRs is a helper function that works on a comma separated cidrs and returns
// a list of typed cidrs
// a flag if cidrs represents a dual stack
// error if failed to parse any of the cidrs
func processCIDRs(cidrsList string) ([]*net.IPNet, bool, error) {
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
