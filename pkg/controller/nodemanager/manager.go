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
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	cloudprovider "k8s.io/cloud-provider"
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
	crdclient "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/clientset/versioned"
	crdinformers "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/informers/externalversions"
	crdlisters "k8s.io/cloud-provider-gcp/pkg/providerconfig/client/listers/providerconfig/v1"
	"k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/klog/v2"

	networkclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	networkinformer "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
)

// NodeManagerController watches ProviderConfig CRDs and starts/stops
// scoped Node Controllers.
type NodeManagerController struct {
	// config is the completed CCM config
	config *config.CompletedConfig

	controlCtx controllermanagerapp.ControllerContext

	// cloud is the CCM's primary, cluster-wide cloud object
	cloud cloudprovider.Interface

	// kubeClient is the primary kubernetes client
	kubeClient clientset.Interface

	// mainInformerFactory is the CCM's primary, unfiltered informer factory
	mainInformerFactory informers.SharedInformerFactory

	// crdClient is a dedicated client for the ProviderConfig CRD
	crdClient crdclient.Interface

	// crdInformerFactory is the dedicated informer factory for ProviderConfig CRDs
	// Stored so it can be stopped during shutdown to prevent goroutine leaks.
	crdInformerFactory crdinformers.SharedInformerFactory

	// crdLister is a lister for the ProviderConfig CRD
	crdLister crdlisters.ProviderConfigLister

	// crdInformerSynced is a function to check if the CRD cache is synced
	crdInformerSynced cache.InformerSynced

	// queue processes ProviderConfig CR keys
	queue workqueue.RateLimitingInterface

	// runningControllers tracks the running node controller goroutines
	// Key: "namespace/name" of the ProviderConfig CR
	// Value: controllerState containing cancel func and last seen spec
	runningControllers     map[string]controllerState
	runningControllersLock sync.RWMutex

	nodeIPAMConfig     nodeipamconfig.NodeIPAMControllerConfiguration
	networkInformer    networkinformer.NetworkInformer
	gnpInformer        networkinformer.GKENetworkParamSetInformer
	nodeTopologyClient nodetopologyclientset.Interface
}

type controllerState struct {
	cancel context.CancelFunc
	spec   v1.ProviderConfigSpec
}

func NewNodeManagerController(kubeClient clientset.Interface, informerFactory informers.SharedInformerFactory,
	completedConfig *config.CompletedConfig, ctx controllermanagerapp.ControllerContext, cloud cloudprovider.Interface, nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration) (*NodeManagerController, error) {
	klog.Infof("Creating ProviderConfig Node Manager Controller")

	clientConfig, err := ctx.ClientBuilder.Config("providerconfig-node-manager")
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config: %w", err)
	}
	// Create a new client for the ProviderConfig CRD.
	// We need this to create a dedicated informer.
	crdClient, err := crdclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD client: %w", err)
	}

	// 3. Create a dedicated informer factory for our CRD
	klog.Infof("Creating ProviderConfig CRD informer factory")
	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, 10*time.Minute)
	crdInformer := crdInformerFactory.Cloud().V1().ProviderConfigs()

	// Initialize network clients and informers for IPAM
	// We initialize them here so they are created once and reused for all scoped controllers.
	kubeConfig := completedConfig.Complete().Kubeconfig
	kubeConfig.ContentType = "application/json" // required to serialize Networks to json
	networkClient, err := networkclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create network client: %w", err)
	}
	nodeTopologyClient, err := nodetopologyclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node topology client: %w", err)
	}
	nwInfFactory := networkinformers.NewSharedInformerFactory(networkClient, 30*time.Second)
	nwInformer := nwInfFactory.Networking().V1().Networks()
	gnpInformer := nwInfFactory.Networking().V1().GKENetworkParamSets()

	// 4. Instantiate the manager controller
	manager := &NodeManagerController{
		config:              completedConfig,
		controlCtx:          ctx,
		kubeClient:          kubeClient,
		cloud:               cloud,
		mainInformerFactory: informerFactory,
		crdClient:           crdClient,
		crdInformerFactory:  crdInformerFactory,
		crdLister:           crdInformer.Lister(),
		crdInformerSynced:   crdInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeManager"),
		runningControllers:  make(map[string]controllerState),
		nodeIPAMConfig:      nodeIPAMConfig,
		networkInformer:     nwInformer,
		gnpInformer:         gnpInformer,
		nodeTopologyClient:  nodeTopologyClient,
	}

	// 5. Set up the event handler for the CRD
	crdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.enqueueCR,
		UpdateFunc: func(old, new interface{}) { manager.enqueueCR(new) },
		DeleteFunc: manager.enqueueCR,
	})

	// 6. Start the dedicated CRD informer
	klog.Infof("Starting ProviderConfig CRD informer factory")
	go crdInformerFactory.Start(ctx.Stop)

	// Start network informers
	klog.Infof("Starting Network informer factory")
	go nwInfFactory.Start(ctx.Stop)

	// 7. Run the manager controller
	klog.Infof("Node Manager controller starting, will run until context is canceled")
	go manager.Run(ctx.Stop)

	return manager, nil
}

// enqueueCR adds a ProviderConfig CR to the workqueue
func (m *NodeManagerController) enqueueCR(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Failed to get key from ProviderConfig object: %v", err)
		runtime.HandleError(err)
		return
	}
	klog.V(3).Infof("Enqueueing ProviderConfig: %s", key)
	m.queue.Add(key)
}

// Run starts the controller's main loop
func (m *NodeManagerController) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer m.queue.ShutDown()

	klog.Info("Starting Node Manager controller worker")

	// Wait for the CRD cache to sync
	klog.Info("Waiting for ProviderConfig CRD cache to sync...")
	if !cache.WaitForCacheSync(stopCh, m.crdInformerSynced) {
		err := fmt.Errorf("timed out waiting for CRD caches to sync")
		klog.Error(err)
		runtime.HandleError(err)
		return
	}
	klog.Info("ProviderConfig CRD cache synced successfully")

	// Start the worker loop
	klog.Info("Starting Node Manager worker loop")
	go wait.Until(m.runWorker, time.Second, stopCh)

	// Block until stop signal is received
	<-stopCh
	klog.Info("Stop signal received. Shutting down Node Manager controller")

	// Ensure any running scoped controllers are canceled to avoid leaks
	klog.Infof("Canceling %d running scoped node controllers", len(m.runningControllers))
	m.runningControllersLock.Lock()
	for key, state := range m.runningControllers {
		klog.Infof("Canceling scoped controller for ProviderConfig '%s' due to manager shutdown", key)
		state.cancel()
		delete(m.runningControllers, key)
	}
	m.runningControllersLock.Unlock()
	klog.Info("Node Manager controller shutdown complete")
}

// runWorker processes items from the queue
func (m *NodeManagerController) runWorker() {
	for m.processNextWorkItem() {
	}
}

func (m *NodeManagerController) processNextWorkItem() bool {
	obj, shutdown := m.queue.Get()
	if shutdown {
		klog.V(2).Info("Work queue is shut down, stopping worker")
		return false
	}

	defer m.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		m.queue.Forget(obj)
		err := fmt.Errorf("expected string in queue but got %#v", obj)
		klog.Error(err)
		runtime.HandleError(err)
		return true
	}

	klog.V(3).Infof("Processing ProviderConfig: %s", key)
	if err := m.syncHandler(key); err != nil {
		m.queue.AddRateLimited(key)
		err = fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		klog.Error(err)
		runtime.HandleError(err)
	} else {
		m.queue.Forget(obj)
		klog.V(3).Infof("Successfully synced ProviderConfig: %s", key)
	}

	return true
}

// syncHandler is the main reconciliation logic
func (m *NodeManagerController) syncHandler(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", key)
	}

	klog.V(3).Infof("Syncing ProviderConfig, fetching from lister: %s", key)
	// Get the ProviderConfig CR from the lister
	cr, err := m.crdLister.Get(name)

	m.runningControllersLock.Lock()
	defer m.runningControllersLock.Unlock()

	if err != nil {
		// Case 1: The CR was DELETED
		if errors.IsNotFound(err) {
			klog.Infof("ProviderConfig '%s' not found (likely deleted). Stopping its node controller if running.", key)
			if state, exists := m.runningControllers[key]; exists {
				klog.Infof("Canceling scoped node controller for deleted ProviderConfig '%s'", key)
				state.cancel() // Send stop signal
				delete(m.runningControllers, key)
			} else {
				klog.V(3).Infof("No running controller found for deleted ProviderConfig '%s'", key)
			}
			return nil
		}
		return err
	}

	// Case 2: The CR was ADDED or UPDATED
	if state, exists := m.runningControllers[key]; exists {
		// Check if Spec has changed
		if !reflect.DeepEqual(state.spec, cr.Spec) {
			klog.Infof("ProviderConfig '%s' spec changed. Restarting controller.", key)
			state.cancel()
			delete(m.runningControllers, key)
			// Fall through to start a new one
		} else {
			klog.V(4).Infof("ProviderConfig '%s' unchanged. Skipping.", key)
			return nil
		}
	}

	// Case 3: The CR was ADDED (or updated and old one stopped)
	klog.Infof("New/Updated ProviderConfig '%s' detected. Starting new scoped node controller. ProjectID: %s", key, cr.Spec.ProjectID)

	// Create a new context so we can stop this goroutine later
	ctx, cancel := context.WithCancel(context.Background())

	// Store the cancel function and spec
	m.runningControllers[key] = controllerState{
		cancel: cancel,
		spec:   cr.Spec,
	}
	klog.V(3).Infof("Registered cancel function for ProviderConfig '%s' (total running: %d)", key, len(m.runningControllers))

	// Start the new node controller in a dedicated goroutine
	// We pass a copy of the CR to avoid data races
	go m.startScopedNodeControllers(ctx, cancel, cr.DeepCopy(), key)

	return nil
}
