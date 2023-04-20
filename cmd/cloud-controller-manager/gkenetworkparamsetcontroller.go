package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	networkclientset "k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned"
	v1alphainformers "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1alpha1"
	gkenetworkparamsetcontroller "k8s.io/cloud-provider-gcp/pkg/controller/gkenetworkparamset"
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

	//no resync, we dont want to automatically update objects if their state changes in gcp
	gkeNetworkParamSetInformer := v1alphainformers.NewGKENetworkParamSetInformer(networkClient, 0*time.Second, cache.Indexers{})

	gkeNetworkParamsetController := gkenetworkparamsetcontroller.NewGKENetworkParamSetController(
		networkClient,
		gkeNetworkParamSetInformer,
		gceCloud,
	)

	go gkeNetworkParamSetInformer.Run(controllerCtx.Stop)

	go gkeNetworkParamsetController.Run(1, controllerCtx.Stop, controllerCtx.ControllerManagerMetrics)
	return nil, true, nil
}
