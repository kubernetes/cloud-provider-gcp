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

package main

import (
	"context"
	"fmt"

	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	dynamicpodip "k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

func startDynamicPodIPControllerWrapper(
	initContext app.ControllerInitContext,
	completedConfig *cloudcontrollerconfig.CompletedConfig,
	cloud cloudprovider.Interface,
) app.InitFunc {
	return func(ctx context.Context, controllerContext genericcontrollermanager.ControllerContext) (controller.Interface, bool, error) {
		return startDynamicPodIPController(completedConfig, ctx, cloud)
	}
}

func startDynamicPodIPController(
	ccmConfig *cloudcontrollerconfig.CompletedConfig,
	ctx context.Context,
	cloud cloudprovider.Interface,
) (controller.Interface, bool, error) {
	klog.Info("Starting Dynamic Pod IP Controller")

	gceCloud, ok := cloud.(*gce.Cloud)
	if !ok {
		return nil, false, fmt.Errorf("dynamic pod IP controller requires GCE cloud provider, but got %T", cloud)
	}

	kubeConfig := ccmConfig.Complete().Kubeconfig
	kubeConfig.ContentType = "application/json" // required to serialize custom resources if needed

	nncClient, err := nncclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create NodeNetworkConfig clientset: %w", err)
	}

	_, ctrl, started, err := dynamicpodip.StartControllers(
		ctx,
		dynamicpodip.Options{
			PopulateNodeNetworkConfig:    populateNodeNetworkConfig,
			EnableDynamicPodIPController: enableDynamicPodIPController,
		},
		ccmConfig.ClientBuilder.ClientOrDie("dynamic-pod-ip-controller"),
		nncClient,
		ccmConfig.SharedInformers.Core().V1().Nodes(), // Pass the Node Informer from CCM config
		gceCloud,
	)
	return ctrl, started, err
}
