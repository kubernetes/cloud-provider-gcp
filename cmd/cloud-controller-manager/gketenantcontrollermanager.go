/*
Copyright 2026 The Kubernetes Authors.
*/
package main

import (
	"context"
	"strings"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework"
	providerconfigcr "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr"
	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	topologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	nodeipamcontrolleroptions "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options"
	"k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers"
	"k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers/utils"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodelifecycle"
	"k8s.io/cloud-provider/controllers/node"
)

type gkeTenantControllerManagerConfig struct {
	ctx               context.Context
	initContext       app.ControllerInitContext
	controllerContext controllermanagerapp.ControllerContext
	completedConfig   *cloudcontrollerconfig.CompletedConfig
	cloud             cloudprovider.Interface
	nodeIPAMConfig    nodeipamconfig.NodeIPAMControllerConfiguration
}

// startGKETenantControllerManagerWrapper is used to take cloud config as input and start the GKE TenantControllerManager controller
func startGKETenantControllerManagerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface, nodeIPAMControllerOptions nodeipamcontrolleroptions.NodeIPAMControllerOptions) app.InitFunc {
	return func(ctx context.Context, controllerContext controllermanagerapp.ControllerContext) (controller.Interface, bool, error) {
		mgrCfg := gkeTenantControllerManagerConfig{
			ctx:               ctx,
			initContext:       initContext,
			controllerContext: controllerContext,
			completedConfig:   completedConfig,
			cloud:             cloud,
			nodeIPAMConfig:    *nodeIPAMControllerOptions.NodeIPAMControllerConfiguration,
		}
		c, _, started, err := startGKETenantControllerManager(mgrCfg)
		return c, started, err
	}
}

func startGKETenantControllerManager(mgrCfg gkeTenantControllerManagerConfig) (controller.Interface, *gketenantcontrollers.ControllersStarter, bool, error) {
	if !enableGKETenantController {
		klog.Infof("GKE Tenant Controller Manager is disabled (enable with --enable-gke-tenant-controller)")
		return nil, nil, false, nil
	}

	clientConfig := rest.CopyConfig(mgrCfg.completedConfig.Kubeconfig)
	clientConfig.ContentType = "application/json" // required to serialize CRDs to json

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
	_ = mgrCfg.controllerContext.InformerFactory.Core().V1().Nodes().Informer()

	// Create dynamic client for framework
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		klog.Errorf("Failed to create dynamic client: %v", err)
		return nil, nil, false, err
	}

	// Create dynamic informer for ProviderConfig using framework
	providerConfigInformer := providerconfigcr.NewInformer(dynamicClient, 0)

	// Define controllers with filtered informers and tenant scoped cloud
	controllers := map[string]gketenantcontrollers.ControllerStartFunc{
		"node-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Creating OSS Cloud Node Controller for %s...", cfg.ProviderConfig.Name)
			// Wrap the informer to filter nodes
			filteringInformer := &utilnode.GCEFilteringNodeInformer{NodeInformer: cfg.NodeInformer}
			nodeController, err := node.NewCloudNodeController(
				filteringInformer,
				cfg.KubeClient,
				cfg.Cloud,
				mgrCfg.completedConfig.ComponentConfig.NodeStatusUpdateFrequency.Duration,
				mgrCfg.completedConfig.ComponentConfig.NodeController.ConcurrentNodeSyncs,
				mgrCfg.completedConfig.ComponentConfig.NodeController.ConcurrentNodeStatusUpdates,
			)
			if err != nil {
				return err
			}
			klog.Infof("Starting OSS Cloud Node Controller for %s", cfg.ProviderConfig.Name)
			// nodeController.Run blocks until the context is cancelled, unlike the IPAM controller
			// which requires explicit blocking.
			nodeController.Run(cfg.Context.Done(), cfg.ControllerContext.ControllerManagerMetrics)
			return nil
		},
		"node-ipam-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Starting Node IPAM Controller for %s...", cfg.ProviderConfig.Name)
			cidrs := getCIDRsFromProviderConfig(cfg.ProviderConfig)

			// Disable MultiSubnetCluster for tenant controllers to prevent them from
			// overwriting the global "default" NodeTopology CR with tenant-specific subnets.
			// We only enable this feature if the current controller belongs to the supervisor.
			tenantNodeIPAMConfig := mgrCfg.nodeIPAMConfig
			if !utils.IsSupervisor(cfg.ProviderConfig) {
				tenantNodeIPAMConfig.EnableMultiSubnetCluster = false
			}

			// Wrap the informer to filter nodes
			filteringInformer := &utilnode.GCEFilteringNodeInformer{NodeInformer: cfg.NodeInformer}

			_, started, err := nodeipam.StartNodeIpamController(
				cfg.Context,
				filteringInformer,
				cfg.KubeClient,
				cfg.Cloud,
				cidrs,
				// Shared configuration (AllocateNodeCIDRs, ServiceCIDR, NodeCIDRMaskSize) is safe to reuse
				// because it defines cluster-wide constraints rather than tenant-specific state.
				// This ensures:
				// 1. Consistent IPAM behavior (e.g., node mask sizes) across all tenants.
				// 2. Prevention of CIDR conflicts (e.g., preventing PodCIDRs from overlapping
				//    with the globally reserved ServiceCIDR).
				mgrCfg.completedConfig.ComponentConfig.KubeCloudShared.AllocateNodeCIDRs,
				mgrCfg.nodeIPAMConfig.ServiceCIDR,
				mgrCfg.nodeIPAMConfig.SecondaryServiceCIDR,
				tenantNodeIPAMConfig,
				networkInformer,
				gnpInformer,
				nodeTopologyClient,
				ipam.CIDRAllocatorType(mgrCfg.completedConfig.ComponentConfig.KubeCloudShared.CIDRAllocatorType),
				cfg.ControllerContext.ControllerManagerMetrics,
			)
			if err != nil {
				return err
			}
			if !started {
				klog.Infof("Node IPAM Controller not started (disabled in config) for %s", cfg.ProviderConfig.Name)
			} else {
				klog.Infof("Node IPAM Controller started with ClusterCIDR: %s for %s", cidrs, cfg.ProviderConfig.Name)
			}
			// StartNodeIpamController spawns a goroutine internally (cidrAllocator.Run), so we must
			// explicitly block here to prevent the controller starter function from returning early.
			<-cfg.Context.Done()
			klog.Infof("Node IPAM Controller stopped for %s", cfg.ProviderConfig.Name)
			return nil
		},
		"node-lifecycle-controller": func(cfg *gketenantcontrollers.ControllerConfig) error {
			klog.Infof("Creating Node Lifecycle Controller for %s...", cfg.ProviderConfig.Name)
			nodeMonitorPeriod := mgrCfg.completedConfig.ComponentConfig.KubeCloudShared.NodeMonitorPeriod.Duration
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
			// lifecycleController.Run blocks until the context is cancelled, unlike the IPAM controller
			// which requires explicit blocking.
			lifecycleController.Run(cfg.Context, cfg.ControllerContext.ControllerManagerMetrics)
			return nil
		},
	}

	// Create the starter
	starter := gketenantcontrollers.NewControllersStarter(
		mgrCfg.completedConfig.ClientBuilder,
		mgrCfg.completedConfig.ClientBuilder.ClientOrDie(mgrCfg.initContext.ClientName),
		dynamicClient,
		mgrCfg.controllerContext.InformerFactory,
		mgrCfg.completedConfig,
		mgrCfg.controllerContext,
		controllers,
	)

	const multiProjectCCMFinalizer = "multiproject.networking.gke.io/ccm-cleanup"
	// Create the framework controller
	mgr := framework.New(
		dynamicClient,
		providerConfigInformer,
		multiProjectCCMFinalizer,
		starter,
		mgrCfg.ctx.Done(),
	)

	// Start network informers only if multinetworking is enabled
	if mgrCfg.nodeIPAMConfig.EnableMultiNetworking {
		networkInformerFactory.Start(mgrCfg.ctx.Done())
	}
	// Start provider config informer
	go providerConfigInformer.Run(mgrCfg.ctx.Done())

	// Run the manager
	go mgr.Run()

	return nil, starter, true, nil
}

// getCIDRsFromProviderConfig returns a comma-separated list of CIDRs from the given ProviderConfig.
func getCIDRsFromProviderConfig(pc *v1.ProviderConfig) string {
	var cidrs []string
	for _, podRange := range pc.Spec.NetworkConfig.SubnetInfo.PodRanges {
		cidrs = append(cidrs, podRange.CIDR)
	}

	return strings.Join(cidrs, ",")
}
