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

	cloudprovider "k8s.io/cloud-provider"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/controllers/node"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

func startCloudNodeControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		return startCloudNodeController(ctx, initContext, controllerContext, completedConfig, cloud)
	}
}

func startCloudNodeController(ctx context.Context, initContext app.ControllerInitContext, controllerContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) (controller.Interface, bool, error) {
	// Wrap the informer to filter nodes
	filteringInformer := &utilnode.GCEFilteringNodeInformer{NodeInformer: completedConfig.SharedInformers.Core().V1().Nodes()}

	// Start the CloudNodeController
	nodeController, err := node.NewCloudNodeController(
		filteringInformer,
		// cloud node controller uses existing cluster role from node-controller
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		cloud,
		completedConfig.ComponentConfig.NodeStatusUpdateFrequency.Duration,
		completedConfig.ComponentConfig.NodeController.ConcurrentNodeSyncs,
		completedConfig.ComponentConfig.NodeController.ConcurrentNodeStatusUpdates,
	)
	if err != nil {
		klog.Warningf("failed to start cloud node controller: %s", err)
		return nil, false, nil
	}

	go nodeController.Run(ctx.Done(), controllerContext.ControllerManagerMetrics)

	return nil, true, nil
}
