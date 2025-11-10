/*
Copyright 2025 The Kubernetes Authors.

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

package nodemanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	cloudprovider "k8s.io/cloud-provider"
	crdclient "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/clientset/versioned"
	crdinformers "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/informers/externalversions"
	crdlisters "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/listers/providerconfig/v1"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/klog/v2"
)

// ManagerController watches ProviderConfig CRDs and starts/stops
// scoped Node Controllers.
type ManagerController struct {
	// mainKubeClient is the standard, cluster-wide client
	mainKubeClient kubernetes.Interface
	// mainCloud is the CCM's primary, cluster-wide cloud object
	mainCloud *gce.Cloud
	// mainInformerFactory is the CCM's primary, unfiltered informer factory
	mainInformerFactory informers.SharedInformerFactory

	// crdClient is a dedicated client for the ProviderConfig CRD
	crdClient crdclient.Interface
	// crdLister is a lister for the ProviderConfig CRD
	crdLister crdlisters.ProviderConfigLister
	// crdInformerSynced is a function to check if the CRD cache is synced
	crdInformerSynced cache.InformerSynced

	// queue processes ProviderConfig CR keys
	queue workqueue.RateLimitingInterface

	// runningControllers tracks the running node controller goroutines
	// Key: "namespace/name" of the ProviderConfig CR
	// Value: context.CancelFunc to stop the goroutine
	runningControllers     map[string]context.CancelFunc
	runningControllersLock sync.RWMutex
}

func StartNodeManagerController(ctx controllermanagerapp.ControllerContext, completedConfig *config.CompletedConfig, cloud cloudprovider.Interface) (cloudprovider.Interface, bool, error) {
	klog.Infof("Starting ProviderConfig Node Manager Controller")

	// 1. Get the main GCECloud object.
	// We need the concrete type to be able to copy it.
	gceCloud, ok := (cloud).(*gce.Cloud)
	if !ok {
		klog.Errorf("StartNodeManagerController only works with gce.Cloud, got %T", cloud)
		return nil, false, fmt.Errorf("StartNodeManagerController only works with gce.Cloud, got %T", cloud)
	}

	// 2. Create a new client for the ProviderConfig CRD.
	// We need this to create a dedicated informer.
	crdClient, err := crdclient.NewForConfig(completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName).CoreV1().RESTClient().Get().RequestURI)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create CRD client: %w", err)
	}

	// 3. Create a dedicated informer factory for our CRD
	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, 10*time.Minute)
	crdInformer := crdInformerFactory.Providerconfig().V1().ProviderConfigs()

	// 4. Instantiate the manager controller
	manager := &ManagerController{
		mainKubeClient:      ctx.KubeClient,
		mainCloud:           gceCloud,
		mainInformerFactory: ctx.InformerFactory,
		crdClient:           crdClient,
		crdLister:           crdInformer.Lister(),
		crdInformerSynced:   crdInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeManager"),
		runningControllers:  make(map[string]context.CancelFunc),
	}

	// 5. Set up the event handler for the CRD
	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.enqueueCR,
		UpdateFunc: func(old, new interface{}) { manager.enqueueCR(new) },
		DeleteFunc: manager.enqueueCR,
	})

	// 6. Start the dedicated CRD informer
	go crdInformerFactory.Start(ctx.Stop)

	// 7. Run the manager controller
	go manager.Run(ctx.Stop)

	// This controller doesn't implement a cloud provider interface
	return nil, true, nil
}

// enqueueCR adds a ProviderConfig CR to the workqueue
func (m *ManagerController) enqueueCR(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	m.queue.Add(key)
}

// Run starts the controller's main loop
func (m *ManagerController) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer m.queue.ShutDown()

	klog.Info("Starting Node Manager controller worker")

	// Wait for the CRD cache to sync
	if !cache.WaitForCacheSync(stopCh, m.crdInformerSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for CRD caches to sync"))
		return
	}

	// Start the worker loop
	go wait.Until(m.runWorker, time.Second, stopCh)

	<-stopCh
	klog.Info("Shutting down Node Manager controller")
}

// runWorker processes items from the queue
func (m *ManagerController) runWorker() {
	for m.processNextWorkItem() {
	}
}

func (m *ManagerController) processNextWorkItem() bool {
	obj, shutdown := m.queue.Get()
	if shutdown {
		return false
	}

	defer m.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		m.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("expected string in queue but got %#v", obj))
		return true
	}

	if err := m.syncHandler(key); err != nil {
		m.queue.AddRateLimited(key)
		runtime.HandleError(fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error()))
	} else {
		m.queue.Forget(obj)
	}

	return true
}

// syncHandler is the main reconciliation logic
func (m *ManagerController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", key)
	}

	// Get the ProviderConfig CR from the lister
	cr, err := m.crdLister.ProviderConfigs(namespace).Get(name)

	m.runningControllersLock.Lock()
	defer m.runningControllersLock.Unlock()

	if err != nil {
		// Case 1: The CR was DELETED
		if errors.IsNotFound(err) {
			klog.Infof("ProviderConfig '%s' deleted, stopping its node controller.", key)
			if cancel, exists := m.runningControllers[key]; exists {
				cancel() // Send stop signal
				delete(m.runningControllers, key)
			}
			return nil
		}
		return err
	}

	// Case 2: The CR was ADDED or UPDATED
	if _, exists := m.runningControllers[key]; exists {
		// TODO: Handle updates.
		// For now, we assume if it's running, it's fine.
		// A real implementation would compare cr.Spec and restart
		// the goroutine if the projectID or token fields changed.
		klog.V(2).Infof("ProviderConfig '%s' updated, but controller is already running. Ignoring.", key)
		return nil
	}

	// Case 3: The CR was ADDED
	klog.Infof("New ProviderConfig '%s' found. Starting new node controller.", key)

	// Create a new context so we can stop this goroutine later
	ctx, cancel := context.WithCancel(context.Background())

	// Store the cancel function
	m.runningControllers[key] = cancel

	// Start the new node controller in a dedicated goroutine
	// We pass a copy of the CR to avoid data races
	go m.startScopedNodeController(ctx, cr.DeepCopy(), key)

	return nil
}
