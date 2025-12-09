package main

import (
	"context"

	cloudprovider "k8s.io/cloud-provider"
	nodeipamcontrolleroptions "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodemanager"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

// startGkeMultiNodeControllerWrapper is used to take cloud config as input and start the GKE service controller
func startGkeMultiNodeControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface, nodeIPAMControllerOptions nodeipamcontrolleroptions.NodeIPAMControllerOptions) app.InitFunc {
	nodeIPAMControllerOptions.ApplyTo(nodeIPAMControllerOptions.NodeIPAMControllerConfiguration)
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		return startGkeMultiNodeController(ctx, initContext, controllerContext, completedConfig, cloud, *nodeIPAMControllerOptions.NodeIPAMControllerConfiguration)
	}
}

func startGkeMultiNodeController(ctx context.Context, initContext app.ControllerInitContext, controlexContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface, nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration) (controller.Interface, bool, error) {
	if !enableMultiProject {
		klog.Warning("MultiNodeController is disabled (enable-multi-project is false)")
		return nil, false, nil
	}
	nodeMgrCtrl, err := nodemanager.NewNodeManagerController(
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		controlexContext.InformerFactory,
		completedConfig, controlexContext, cloud, nodeIPAMConfig)
	if err != nil {
		klog.Errorf("Failed to start node manager controller: %v", err)
		return nil, false, nil
	}

	go nodeMgrCtrl.Run(ctx.Done())

	return nil, true, nil
}
