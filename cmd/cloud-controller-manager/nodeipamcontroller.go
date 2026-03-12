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

package main

import (
	"context"
	"time"

	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	"k8s.io/apimachinery/pkg/util/wait"
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

	// TODO: Add a flag to control to start this informer specific to required GKE functionality
	go nwInfFactory.Start(ctx.Stop)

	return nodeipamcontroller.StartNodeIpamController(
		wait.ContextForChannel(ctx.Stop),
		ccmConfig.SharedInformers.Core().V1().Nodes(),
		ccmConfig.ClientBuilder.ClientOrDie("node-controller"),
		cloud,
		ccmConfig.ComponentConfig.KubeCloudShared.ClusterCIDR,
		ccmConfig.ComponentConfig.KubeCloudShared.AllocateNodeCIDRs,
		nodeIPAMConfig.ServiceCIDR,
		nodeIPAMConfig.SecondaryServiceCIDR,
		nodeIPAMConfig,
		nwInformer,
		gnpInformer,
		nodeTopologyClient,
		ipam.CIDRAllocatorType(ccmConfig.ComponentConfig.KubeCloudShared.CIDRAllocatorType),
		ctx.ControllerManagerMetrics,
	)
}
