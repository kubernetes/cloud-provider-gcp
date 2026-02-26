/*
Copyright 2026 The Kubernetes Authors.
*/
package main

import (
	"context"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework"
	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	topologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	cloudprovider "k8s.io/cloud-provider"
	nodeipamcontrolleroptions "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options"
	"k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	"k8s.io/cloud-provider/controllers/node"
	"k8s.io/cloud-provider/controllers/nodelifecycle"
)

// startGKETenantControllerManagerWrapper is used to take cloud config as input and start the GKE TenantControllerManager controller
func startGKETenantControllerManagerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface, nodeIPAMControllerOptions nodeipamcontrolleroptions.NodeIPAMControllerOptions) app.InitFunc {
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		c, _, started, err := startGKETenantControllerManager(ctx, initContext, controllerContext, completedConfig, cloud, *nodeIPAMControllerOptions.NodeIPAMControllerConfiguration)
		return c, started, err
	}
}

func startGKETenantControllerManager(ctx context.Context, initContext app.ControllerInitContext, controlexContext controllermanagerapp.ControllerContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface, nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration) (controller.Interface, *gketenantcontrollers.ControllersStarter, bool, error) {
	if !enableGKETenantController {
		klog.Infof("GKE Tenant Controller Manager is disabled (enable with --enable-gke-tenant-controller)")
		return nil, nil, false, nil
	}

	clientConfig := completedConfig.Kubeconfig

	// Create network clients and informers
	networkClient, err := networkclientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Errorf("Failed to create network client: %v", err)
		return nil, nil, false, err
	}
	networkInformerFactory := networkinformers.NewSharedInformerFactory(networkClient, 0)
	networkInformer := networkInformerFactory.Networking().V1().Networks()
	gnpInformer := networkInformerFactory.Networking().V1().GKENetworkParamSets()

	// Create topology client
	nodeTopologyClient, err := topologyclientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Errorf("Failed to create topology client: %v", err)
		return nil, nil, false, err
	}

	// Eagerly request the main Node informer so the SharedInformerFactory starts it.
	// All informers that are needed by all of the controllers that will be started
	// on a per provider config need to be started here. This is different from the
	// other case because CCM will only run tenant controller manager, so the factory
	// is started before all the other controllers are initialized.
	_ = controlexContext.InformerFactory.Core().V1().Nodes().Informer()

	// Create dynamic client for framework
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		klog.Errorf("Failed to create dynamic client: %v", err)
		return nil, nil, false, err
	}

	// Create dynamic informer factory for ProviderConfig
	gvr := schema.GroupVersionResource{
		Group:    providerconfigv1.GroupName,
		Version:  providerconfigv1.SchemeGroupVersion.Version,
		Resource: "providerconfigs",
	}
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	providerConfigInformer := dynamicInformerFactory.ForResource(gvr).Informer()

	// Define controllers with filtered informers and tenant scoped cloud
	controllers := map[string]gketenantcontrollers.ControllerStartFunc{
		"node-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Creating OSS Cloud Node Controller for %s...", cfg.ProviderConfig.Name)
			nodeController, err := node.NewCloudNodeController(
				cfg.NodeInformer,
				cfg.KubeClient,
				cfg.Cloud,
				completedConfig.ComponentConfig.NodeStatusUpdateFrequency.Duration,
				completedConfig.ComponentConfig.NodeController.ConcurrentNodeSyncs,
			)
			if err != nil {
				return err
			}
			klog.Infof("Starting OSS Cloud Node Controller for %s (blocking)", cfg.ProviderConfig.Name)
			nodeController.Run(cfg.Context.Done(), cfg.ControllerContext.ControllerManagerMetrics)
			return nil
		},
		"node-ipam-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Starting Node IPAM Controller for %s...", cfg.ProviderConfig.Name)
			clusterCIDR, err := gketenantcontrollers.GetClusterCIDRsFromProviderConfig(cfg.ProviderConfig)
			if err != nil {
				klog.Errorf("Failed to get ClusterCIDRs from ProviderConfig: %v. Node IPAM Controller will be disabled.", err)
				return nil // Don't fail the whole start
			}

			_, started, err := nodeipam.StartNodeIpamController(
				cfg.Context,
				cfg.NodeInformer,
				cfg.KubeClient,
				cfg.Cloud,
				clusterCIDR,
				completedConfig.ComponentConfig.KubeCloudShared.AllocateNodeCIDRs,
				nodeIPAMConfig.ServiceCIDR,
				nodeIPAMConfig.SecondaryServiceCIDR,
				nodeIPAMConfig,
				networkInformer,
				gnpInformer,
				nodeTopologyClient,
				ipam.CIDRAllocatorType(completedConfig.ComponentConfig.KubeCloudShared.CIDRAllocatorType),
				cfg.ControllerContext.ControllerManagerMetrics,
			)
			if err != nil {
				return err
			}
			if !started {
				klog.Infof("Node IPAM Controller not started (disabled in config) for %s", cfg.ProviderConfig.Name)
			} else {
				klog.Infof("Node IPAM Controller started with ClusterCIDR: %s for %s", clusterCIDR, cfg.ProviderConfig.Name)
			}
			// Block until context is canceled so starter doesn't exit early
			<-cfg.Context.Done()
			return nil
		},
		"node-lifecycle-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Creating Node Lifecycle Controller for %s...", cfg.ProviderConfig.Name)
			nodeMonitorPeriod := completedConfig.ComponentConfig.KubeCloudShared.NodeMonitorPeriod.Duration
			lifecycleController, err := nodelifecycle.NewCloudNodeLifecycleController(
				cfg.NodeInformer,
				cfg.KubeClient,
				cfg.Cloud,
				nodeMonitorPeriod,
			)
			if err != nil {
				return err
			}
			klog.Infof("Starting Node Lifecycle Controller for %s...", cfg.ProviderConfig.Name)
			lifecycleController.Run(cfg.Context, cfg.ControllerContext.ControllerManagerMetrics)
			return nil
		},
	}

	// Create the starter
	starter := gketenantcontrollers.NewControllersStarter(
		completedConfig.ClientBuilder,
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		controlexContext.InformerFactory,
		completedConfig,
		controlexContext,
		controllers,
	)

	// Create the framework manager
	mgr := framework.New(
		dynamicClient,
		providerConfigInformer,
		gkeTenantControllerManagerName,
		starter,
		ctx.Done(),
	)

	// Start network informers only if multinetworking is enabled
	if nodeIPAMConfig.EnableMultiNetworking {
		networkInformerFactory.Start(ctx.Done())
	}
	// Start dynamic informers
	dynamicInformerFactory.Start(ctx.Done())

	// Run the manager
	go mgr.Run()

	return nil, starter, true, nil
}
