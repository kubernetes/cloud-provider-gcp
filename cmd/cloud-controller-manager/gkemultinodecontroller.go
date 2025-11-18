package main

import (
	"context"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodemanager"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

// startGkeServiceControllerWrapper is used to take cloud config as input and start the GKE service controller
func startGkeMultiNodeControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		return startGkeMultiNodeController(ctx, initContext, controllerContext, completedConfig, cloud)
	}
}

func startGkeMultiNodeController(ctx context.Context, initContext app.ControllerInitContext, controlexContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) (controller.Interface, bool, error) {
	nodeMgrCtrl, err := nodemanager.NewNodeManagerController(
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		controlexContext.InformerFactory,
		completedConfig, controlexContext, cloud)
	if err != nil {
		// This error shouldn't fail. It lives like this as a legacy.
		klog.Errorf("Failed to start service controller: %v", err)
		return nil, false, nil
	}

	go nodeMgrCtrl.Run(ctx.Done())

	return nil, true, nil
}
