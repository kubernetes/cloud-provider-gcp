/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"context"

	"k8s.io/client-go/tools/cache"

	// This is the OSS Node Controller package
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
	"k8s.io/cloud-provider/controllers/node"
	"k8s.io/klog/v2"
)

// startScopedNodeController runs a new OSS Node Controller in a goroutine.
// This goroutine blocks until the provided context is canceled.
func (m *NodeManagerController) startScopedNodeController(ctx context.Context, pc *v1.ProviderConfig, pcKey string) {
	klog.Infof("[%s] Attempting to start scoped node controller", pcKey)

	// Create the new, scoped GCECloud object
	klog.V(2).Infof("[%s] Creating tenant-scoped GCE cloud object...", pcKey)
	scopedCloud, err := CreateTenantScopedGCECloud(m.config, pc)
	if err != nil {
		klog.Errorf("[%s] Failed to create scoped cloud: %v. Aborting controller startup.", pcKey, err)
		// We need to stop tracking this controller so it can be retried
		m.stopAndForget(pcKey)
		return
	}
	klog.V(2).Infof("[%s] Scoped GCE cloud created successfully", pcKey)

	providerConfigName, err := getNodeLabelSelector(pc)
	if err != nil {
		klog.Errorf("[%s] Failed to get node label selector: %v. Aborting controller startup.", pcKey, err)
		m.stopAndForget(pcKey)
		return
	}
	klog.Infof("[%s] Using node label selector: %s", pcKey, providerConfigName)

	klog.V(2).Infof("[%s] Creating filtered informer factory...", pcKey)
	filteredFactory := NewFilteredSharedInformerFactory(m.mainInformerFactory, ProviderConfigLabelKey, providerConfigName)

	klog.Infof("[%s] Waiting for main informer caches to sync...", pcKey)
	if !cache.WaitForCacheSync(ctx.Done(),
		m.mainInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		m.mainInformerFactory.Core().V1().Pods().Informer().HasSynced,
	) {
		klog.Errorf("[%s] Failed to sync main caches. Aborting controller startup.", pcKey)
		m.stopAndForget(pcKey)
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
		m.stopAndForget(pcKey)
		return
	}
	klog.V(2).Infof("[%s] OSS Cloud Node Controller created successfully", pcKey)

	// Run with a wrapper so we can log lifecycle and clean up the registry when it exits.
	klog.Infof("[%s] Starting OSS Node Controller (will run until context is canceled)", pcKey)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				klog.Errorf("[%s] OSS Node Controller panicked: %v", pcKey, r)
			}
		}()

		klog.Infof("[%s] OSS Node Controller is running", pcKey)
		nodeController.Run(ctx.Done(), m.controlCtx.ControllerManagerMetrics)
		klog.Infof("[%s] OSS Node Controller exited normally", pcKey)

		// Ensure the runningControllers map is cleaned up if the controller exited.
		m.runningControllersLock.Lock()
		if _, exists := m.runningControllers[pcKey]; exists {
			klog.V(2).Infof("[%s] Cleaning up controller entry from running map", pcKey)
			delete(m.runningControllers, pcKey)
		}
		m.runningControllersLock.Unlock()
	}()
}

// stopAndForget ensures a controller's context is canceled and removes it
// from the tracking map.
func (m *NodeManagerController) stopAndForget(key string) {
	m.runningControllersLock.Lock()
	defer m.runningControllersLock.Unlock()

	if cancel, exists := m.runningControllers[key]; exists {
		klog.Infof("[%s] Stopping and cleaning up controller reference", key)
		cancel()
		delete(m.runningControllers, key)
	} else {
		klog.V(3).Infof("[%s] No running controller found to stop", key)
	}
}
