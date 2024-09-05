package main

import (
	"context"
	"fmt"
	"net"
	"time"

	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	cloudprovider "k8s.io/cloud-provider"
	gkenetworkparamsetcontroller "k8s.io/cloud-provider-gcp/pkg/controller/gkenetworkparamset"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
)

const jsonContentType = "application/json"

func startGkeNetworkParamSetControllerWrapper(initCtx app.ControllerInitContext, config *cloudcontrollerconfig.CompletedConfig, c cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerCtx genericcontrollermanager.ControllerContext) (controller.Interface, bool, error) {
		return startGkeNetworkParamsController(config, controllerCtx, c)
	}
}

func startGkeNetworkParamsController(ccmConfig *cloudcontrollerconfig.CompletedConfig, controllerCtx genericcontrollermanager.ControllerContext, cloud cloudprovider.Interface) (controller.Interface, bool, error) {

	gceCloud, ok := cloud.(*gce.Cloud)
	if !ok {
		err := fmt.Errorf("GkeNetworkParamsController does not support %v provider", cloud.ProviderName())
		return nil, false, err
	}

	kubeConfig := ccmConfig.Complete().Kubeconfig
	kubeConfig.ContentType = jsonContentType //required to serialize GKENetworkParamSet to json

	networkClient, err := networkclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, false, err
	}

	// the CloudAllocator needs cluster cidrs for default GNP
	clusterCIDRs := []*net.IPNet{}
	if ipam.CIDRAllocatorType(ccmConfig.ComponentConfig.KubeCloudShared.CIDRAllocatorType) == ipam.CloudAllocatorType {
		clusterCIDRs, err = validClusterCIDR(ccmConfig.ComponentConfig.KubeCloudShared.ClusterCIDR)
		if err != nil {
			return nil, false, err
		}
	}

	nwInfFactory := networkinformers.NewSharedInformerFactory(networkClient, 30*time.Second)
	nwInformer := nwInfFactory.Networking().V1().Networks()
	gnpInformer := nwInfFactory.Networking().V1().GKENetworkParamSets()

	gkeNetworkParamsetController := gkenetworkparamsetcontroller.NewGKENetworkParamSetController(
		controllerCtx.InformerFactory.Core().V1().Nodes(),
		networkClient,
		gnpInformer,
		nwInformer,
		gceCloud,
		nwInfFactory,
		clusterCIDRs,
	)

	go gkeNetworkParamsetController.Run(1, controllerCtx.Stop, controllerCtx.ControllerManagerMetrics)
	return nil, true, nil
}

// validClusterCIDR process CIDR form config and validates the cluster CIDR
// with stack type and returns a list of typed cidrs and error
func validClusterCIDR(clusterCIDRFromFlag string) ([]*net.IPNet, error) {
	// failure: bad cidrs in config
	clusterCIDRs, dualStack, err := processCIDRs(clusterCIDRFromFlag)
	if err != nil {
		return nil, err
	}

	// failure: more than one cidr but they are not configured as dual stack
	if len(clusterCIDRs) > 1 && !dualStack {
		return nil, fmt.Errorf("len of ClusterCIDRs==%v and they are not configured as dual stack (at least one from each IPFamily", len(clusterCIDRs))
	}

	// failure: more than cidrs is not allowed even with dual stack
	if len(clusterCIDRs) > 2 {
		return nil, fmt.Errorf("len of clusters is:%v > more than max allowed of 2", len(clusterCIDRs))
	}
	return clusterCIDRs, nil
}
