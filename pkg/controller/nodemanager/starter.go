/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"

	// This is the OSS Node Controller package
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
	"k8s.io/cloud-provider/controllers/node"
	"k8s.io/klog/v2"
)

// startScopedNodeController runs a new OSS Node Controller in a goroutine.
// This goroutine blocks until the provided context is canceled.
func (m *ManagerController) startScopedNodeController(ctx context.Context, cr *v1.ProviderConfig, crKey string) {
	klog.Infof("Attempting to start scoped node controller for '%s'", crKey)

	// 1. Create the new, scoped GCECloud object
	scopedCloud, err := CreateTenantScopedGCECloud(m.mainCloud, cr)
	if err != nil {
		klog.Errorf("Failed to create scoped cloud for '%s': %v. Aborting.", crKey, err)
		// We need to stop tracking this controller so it can be retried
		m.stopAndForget(crKey)
		return
	}

	// 2. Get the label selector from the CR
	labelSelectorString, err := getNodeLabelSelector(cr)
	if err != nil {
		klog.Errorf("Failed to get node label selector for '%s': %v. Aborting.", crKey, err)
		m.stopAndForget(crKey)
		return
	}
	klog.Infof("Using node label selector for '%s': %s", crKey, labelSelectorString)

	// 3. Create the TweakListOptions function
	tweakOptions := func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelectorString
	}

	// 4. Create a new, DEDICATED informer factory with this filter
	filteredFactory := informers.NewSharedInformerFactoryWithOptions(
		m.mainKubeClient,
		10*time.Minute, // Or use a value from config
		informers.WithTweakListOptions(tweakOptions),
	)

	// 5. Create the new, scoped ControllerContext
	// This is the magic: we create a new context object that passes
	// our *scoped* cloud and *filtered* informer factory to the controller.
	scopedCtx := cloudprovider.ControllerContext{
		KubeClient:      m.mainKubeClient,
		Cloud:           scopedCloud,
		InformerFactory: filteredFactory,
		Stop:            ctx.Done(),
		// TODO: You must also pass the NodeController's specific config
		// from the main CCM config object (e.g., nodeMonitorPeriod, etc.)
		// This requires plumbing it from StartNodeManagerController -> ManagerController
	}

	// 6. Start the filtered factory
	// This will block until the context is canceled
	go filteredFactory.Start(ctx.Done())

	// 7. Wait for the new factory's caches to sync
	klog.Infof("Waiting for filtered informer caches to sync for '%s'...", crKey)
	// The Node Controller needs Nodes and Pods.
	// **IMPORTANT**: You must add *all* informers the Node Controller needs here.
	if !cache.WaitForCacheSync(ctx.Done(),
		filteredFactory.Core().V1().Nodes().Informer().HasSynced,
		filteredFactory.Core().V1().Pods().Informer().HasSynced,
	) {
		klog.Errorf("Failed to sync filtered caches for '%s'. Aborting.", crKey)
		m.stopAndForget(crKey) // stopAndForget will cancel the context, stopping filteredFactory
		return
	}

	klog.Infof("Caches synced for '%s'. Starting OSS Node Controller.", crKey)

	// 8. Run the OSS Node Controller!
	// This function will block until scopedCtx.Stop is closed (i.e., our context is canceled).
	// We pass 'nil' for the node client, as StartNodeController will use the KubeClient.
	if err := node.RunWithContext(scopedCtx, nil); err != nil {
		klog.Errorf("Node controller for '%s' exited with error: %v", crKey, err)
	}

	klog.Infof("Scoped node controller for '%s' has stopped.", crKey)

	// Clean up just in case the stop was not initiated by the manager
	m.stopAndForget(crKey)
}

// stopAndForget ensures a controller's context is canceled and removes it
// from the tracking map.
func (m *ManagerController) stopAndForget(key string) {
	m.runningControllersLock.Lock()
	defer m.runningControllersLock.Unlock()

	if cancel, exists := m.runningControllers[key]; exists {
		klog.Infof("Cleaning up controller reference for '%s'", key)
		cancel()
		delete(m.runningControllers, key)
	}
}
