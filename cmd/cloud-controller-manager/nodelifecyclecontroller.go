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
	"k8s.io/cloud-provider-gcp/pkg/controller/nodelifecycle"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

func startCloudNodeLifecycleControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		return startCloudNodeLifecycleController(ctx, initContext, controllerContext, completedConfig, cloud)
	}
}

func startCloudNodeLifecycleController(ctx context.Context, initContext app.ControllerInitContext, controllerContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) (controller.Interface, bool, error) {
	// Start the cloudNodeLifecycleController
	cloudNodeLifecycleController, err := nodelifecycle.NewCloudNodeLifecycleController(
		completedConfig.SharedInformers.Core().V1().Nodes(),
		// cloud node lifecycle controller uses existing cluster role from node-controller
		completedConfig.ClientBuilder.ClientOrDie("cloud-node-lifecycle-controller"),
		cloud,
		completedConfig.ComponentConfig.KubeCloudShared.NodeMonitorPeriod.Duration,
	)
	if err != nil {
		klog.Warningf("failed to start cloud node lifecycle controller: %s", err)
		return nil, false, nil
	}

	go cloudNodeLifecycleController.Run(ctx, controllerContext.ControllerManagerMetrics)

	return nil, true, nil
}
