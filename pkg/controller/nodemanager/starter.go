/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"context"

	"k8s.io/client-go/tools/cache"

	// This is the OSS Node Controller package
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam"
	"k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/ipam"
	"k8s.io/cloud-provider/controllers/node"
	"k8s.io/cloud-provider/controllers/nodelifecycle"
	"k8s.io/klog/v2"
)

// startScopedNodeControllers runs a new OSS Node Controller in a goroutine.
// This goroutine blocks until the provided context is canceled.
func (m *NodeManagerController) startScopedNodeControllers(ctx context.Context, cancel context.CancelFunc, pc *v1.ProviderConfig, pcKey string) {
	klog.Infof("[%s] Attempting to start scoped node controller", pcKey)
	var filteredFactory *FilteredSharedInformerFactory

	// Ensure cleanup happens when this function exits (either due to error or context cancellation)
	defer func() {
		klog.Infof("[%s] Scoped node controller stopping", pcKey)
		// Cancel the context to stop other controllers (IPAM, Lifecycle)
		cancel()

		// Cleanup event handlers from the global informer
		if filteredFactory != nil {
			klog.Infof("[%s] Cleaning up filtered informer event handlers", pcKey)
			filteredFactory.Cleanup()
		}

		// Ensure the runningControllers map is cleaned up
		m.runningControllersLock.Lock()
		if _, exists := m.runningControllers[pcKey]; exists {
			klog.V(2).Infof("[%s] Cleaning up controller entry from running map", pcKey)
			delete(m.runningControllers, pcKey)
		}
		m.runningControllersLock.Unlock()
	}()

	// Create the new, scoped GCECloud object
	klog.V(2).Infof("[%s] Creating tenant-scoped GCE cloud object...", pcKey)
	scopedCloud, err := CreateTenantScopedGCECloud(m.config, pc)
	if err != nil {
		klog.Errorf("[%s] Failed to create scoped cloud: %v. Aborting controller startup.", pcKey, err)
		return
	}
	klog.V(2).Infof("[%s] Scoped GCE cloud created successfully", pcKey)

	providerConfigName, err := getNodeLabelSelector(pc)
	if err != nil {
		klog.Errorf("[%s] Failed to get node label selector: %v. Aborting controller startup.", pcKey, err)
		return
	}
	klog.Infof("[%s] Using node label selector: %s", pcKey, providerConfigName)

	klog.V(2).Infof("[%s] Creating filtered informer factory...", pcKey)
	filteredFactory = NewFilteredSharedInformerFactory(m.mainInformerFactory, ProviderConfigLabelKey, providerConfigName)

	klog.Infof("[%s] Waiting for main informer caches to sync...", pcKey)
	if !cache.WaitForCacheSync(ctx.Done(),
		m.mainInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		m.mainInformerFactory.Core().V1().Pods().Informer().HasSynced,
	) {
		klog.Errorf("[%s] Failed to sync main caches. Aborting controller startup.", pcKey)
		return
	}
	klog.Infof("[%s] Main informer caches synced successfully", pcKey)

	klog.Infof("[%s] Creating OSS Cloud Node Controller...", pcKey)
	nodeController, err := node.NewCloudNodeController(
		filteredFactory.Core().V1().Nodes(),
		m.kubeClient,
		scopedCloud,
		m.config.ComponentConfig.NodeStatusUpdateFrequency.Duration,
		m.config.ComponentConfig.NodeController.ConcurrentNodeSyncs,
	)
	if err != nil {
		klog.Errorf("[%s] Failed to create OSS Node Controller: %v. Aborting controller startup.", pcKey, err)
		return
	}
	klog.V(2).Infof("[%s] OSS Cloud Node Controller created successfully", pcKey)

	// Start Node IPAM Controller
	klog.Infof("[%s] Starting Node IPAM Controller...", pcKey)
	_, started, err := nodeipam.StartNodeIpamController(
		ctx,
		filteredFactory.Core().V1().Nodes(),
		m.kubeClient,
		scopedCloud,
		m.config.ComponentConfig.KubeCloudShared.ClusterCIDR,
		m.config.ComponentConfig.KubeCloudShared.AllocateNodeCIDRs,
		m.nodeIPAMConfig.ServiceCIDR,
		m.nodeIPAMConfig.SecondaryServiceCIDR,
		m.nodeIPAMConfig,
		m.networkInformer,
		m.gnpInformer,
		m.nodeTopologyClient,
		ipam.CIDRAllocatorType(m.config.ComponentConfig.KubeCloudShared.CIDRAllocatorType),
		m.controlCtx.ControllerManagerMetrics,
	)
	if err != nil {
		klog.Errorf("[%s] Failed to start Node IPAM Controller: %v", pcKey, err)
		// We don't abort here, as the main node controller is running.
	} else if !started {
		klog.Infof("[%s] Node IPAM Controller not started (disabled in config)", pcKey)
	} else {
		klog.Infof("[%s] Node IPAM Controller started", pcKey)
	}

	// Start Node Lifecycle Controller
	klog.Infof("[%s] Creating Node Lifecycle Controller...", pcKey)
	nodeMonitorPeriod := m.config.ComponentConfig.KubeCloudShared.NodeMonitorPeriod.Duration
	lifecycleController, err := nodelifecycle.NewCloudNodeLifecycleController(
		filteredFactory.Core().V1().Nodes(),
		m.kubeClient,
		scopedCloud,
		nodeMonitorPeriod,
	)
	if err != nil {
		klog.Errorf("[%s] Failed to create Node Lifecycle Controller: %v", pcKey, err)
	} else {
		klog.Infof("[%s] Starting Node Lifecycle Controller...", pcKey)
		go lifecycleController.Run(ctx, m.controlCtx.ControllerManagerMetrics)
	}

	// Run the OSS Node Controller (Blocking)
	// This function acts as the supervisor goroutine. It blocks here until the Node Controller exits
	// (due to context cancellation or error). When it returns, the deferred cleanup function
	// will cancel the context (stopping IPAM and Lifecycle controllers) and clean up resources.
	klog.Infof("[%s] Starting OSS Node Controller (will run until context is canceled)", pcKey)
	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("[%s] OSS Node Controller panicked: %v", pcKey, r)
		}
	}()
	nodeController.Run(ctx.Done(), m.controlCtx.ControllerManagerMetrics)
	klog.Infof("[%s] OSS Node Controller exited normally", pcKey)
}
