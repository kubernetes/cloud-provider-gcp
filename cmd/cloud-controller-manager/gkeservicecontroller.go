package main

import (
	"context"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options"
	gkeservicecontroller "k8s.io/cloud-provider-gcp/pkg/controller/service"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

type gkeServiceController struct {
	gkeServiceControllerOptions options.GkeServiceControllerOptions
}

// startGkeServiceControllerWrapper is used to take cloud config as input and start the GKE service controller
func (gkeServiceController *gkeServiceController) startGkeServiceControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	if gkeServiceController.gkeServiceControllerOptions.EnableGkeServiceController {
		return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
			return startGkeServiceController(ctx, initContext, controllerContext, completedConfig, cloud)
		}
	}
	return app.StartServiceControllerWrapper(initContext, completedConfig, cloud)
}

func startGkeServiceController(ctx context.Context, initContext app.ControllerInitContext, controlexContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) (controller.Interface, bool, error) {
	// Start the service controller
	serviceController, err := gkeservicecontroller.New(
		cloud,
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		completedConfig.SharedInformers.Core().V1().Services(),
		completedConfig.SharedInformers.Core().V1().Nodes(),
		completedConfig.ComponentConfig.KubeCloudShared.ClusterName,
		utilfeature.DefaultFeatureGate,
	)
	if err != nil {
		// This error shouldn't fail. It lives like this as a legacy.
		klog.Errorf("Failed to start service controller: %v", err)
		return nil, false, nil
	}

	go serviceController.Run(ctx, int(completedConfig.ComponentConfig.ServiceController.ConcurrentServiceSyncs), controlexContext.ControllerManagerMetrics)

	return nil, true, nil
}
