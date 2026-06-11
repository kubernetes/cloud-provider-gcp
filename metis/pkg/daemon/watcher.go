/*
Copyright 2026 The Kubernetes Authors.

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

package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/metis/pkg/store"
)

const defaultWatcherWorkers = 4

// Watcher monitors the NodeNetworkConfig (NNC) resource for the local node where this daemon runs.
// It synchronizes the local database with the control plane's CIDR allocations:
//  1. Adds newly allocated CIDRs to the local database, triggering the OnCIDRAdded callback to wake up
//     blocked CNI requests.
//  2. Safely deletes CIDR blocks from the local database once they are successfully released by GCE/controller
//     and removed from the NNC Status.
type Watcher struct {
	queue workqueue.TypedRateLimitingInterface[string]
	// syncHandler is the function called to sync a network work item. Decoupling this
	// via a field function pointer allows unit tests to easily mock/override the sync
	// logic without requiring full clients, informers, or stores.
	syncHandler func(ctx context.Context, network string) error
	nncClient   nncclientset.Interface
	nodeName    string
	nncLister   nnclisters.NodeNetworkConfigLister
	nncSynced   cache.InformerSynced
	store       *store.Store
	logger      logr.Logger
	OnCIDRAdded func(network string, availableIPs int)
}

// NewWatcher creates a new Watcher.
func NewWatcher(logger logr.Logger, nncClient nncclientset.Interface, nncInformer nncinformers.NodeNetworkConfigInformer, store *store.Store, nodeName string, onCIDRAdded func(network string, availableIPs int)) *Watcher {
	rl := workqueue.DefaultTypedControllerRateLimiter[string]()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[string]{
		Name: "metis-nnc-watcher",
	})

	var nncLister nnclisters.NodeNetworkConfigLister
	var nncSynced cache.InformerSynced
	// nncInformer is is never nil. This is for UT.
	if nncInformer != nil {
		nncLister = nncInformer.Lister()
		nncSynced = nncInformer.Informer().HasSynced
	}

	w := &Watcher{
		queue:       queue,
		nncClient:   nncClient,
		nodeName:    nodeName,
		nncLister:   nncLister,
		nncSynced:   nncSynced,
		store:       store,
		logger:      logger,
		OnCIDRAdded: onCIDRAdded,
	}
	w.syncHandler = w.syncCIDR

	// nncInformer is never nil. This is for UT.
	if nncInformer != nil {
		nncInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				nnc, ok := obj.(*nncv1.NodeNetworkConfig)
				if !ok {
					return
				}
				if nnc.Name == nodeName {
					for _, alloc := range nnc.Spec.Allocations {
						w.queue.Add(alloc.Network)
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldNNC, ok1 := oldObj.(*nncv1.NodeNetworkConfig)
				newNNC, ok2 := newObj.(*nncv1.NodeNetworkConfig)
				if !ok1 || !ok2 {
					return
				}
				if oldNNC.ResourceVersion == newNNC.ResourceVersion {
					return
				}
				if newNNC.Name == nodeName {
					for _, alloc := range newNNC.Spec.Allocations {
						w.queue.Add(alloc.Network)
					}
				}
			},
		})
	}

	return w
}

// Run starts the watcher workers and blocks until the context is cancelled.
func (w *Watcher) Run(ctx context.Context, workers int) {
	defer w.queue.ShutDown()

	w.logger.Info("Starting Metis Daemon watcher", "node", w.nodeName, "workers", workers)
	defer w.logger.Info("Stopping Metis Daemon watcher")

	if w.nncSynced != nil {
		if !cache.WaitForNamedCacheSync("MetisNNCWatcher", ctx.Done(), w.nncSynced) {
			return
		}
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, w.runWorker, time.Second)
	}

	<-ctx.Done()
}

func (w *Watcher) runWorker(ctx context.Context) {
	for w.processNextWorkItem(ctx) {
	}
}

func (w *Watcher) processNextWorkItem(ctx context.Context) bool {
	item, quit := w.queue.Get()
	if quit {
		return false
	}
	defer w.queue.Done(item)

	err := w.syncHandler(ctx, item)
	if err == nil {
		w.queue.Forget(item)
		return true
	}

	if w.queue.NumRequeues(item) < updateMaxRetries {
		w.logger.Error(err, "Failed to sync item, retrying", "item", item, "requeues", w.queue.NumRequeues(item))
		w.queue.AddRateLimited(item)
		return true
	}

	w.logger.Error(nil, "Failed to sync item, dropping from queue after max retries", "item", item)
	w.queue.Forget(item)
	return true
}

func (w *Watcher) syncCIDR(ctx context.Context, network string) error {
	w.logger.Info("Daemon watcher starting synchronization: reconciling CIDR blocks with NodeNetworkConfig status", "node", w.nodeName, "network", network)

	nnc, err := getNodeNetworkConfig(ctx, w.nncLister, w.nncClient, w.nodeName)
	if err != nil {
		return err
	}

	if err := w.addCIDR(ctx, nnc, network); err != nil {
		return err
	}

	if err := w.maybeDeleteCIDRs(ctx, nnc, network); err != nil {
		return err
	}

	w.logger.Info("Daemon watcher synchronization done", "node", w.nodeName, "network", network)
	return nil
}

func (w *Watcher) addCIDR(ctx context.Context, nnc *nncv1.NodeNetworkConfig, network string) error {
	for _, podCIDR := range nnc.Status.PodCIDRs {
		if podCIDR.Network != network {
			continue
		}
		if podCIDR.Condition != nil && podCIDR.Condition.Status != metav1.ConditionTrue {
			w.logger.V(4).Info("PodCIDR not ready, skipping", "cidr", podCIDR.CIDR, "network", podCIDR.Network)
			continue
		}

		prefix, err := netip.ParsePrefix(podCIDR.CIDR)
		if err != nil {
			w.logger.Error(err, "failed to parse CIDR", "cidr", podCIDR.CIDR)
			continue
		}

		if prefix.Addr().Is6() {
			w.logger.V(4).Info("Ignoring IPv6 CIDR, not supported in dynamic allocation path", "cidr", podCIDR.CIDR)
			continue
		}

		bits := prefix.Bits()
		availableIPs := 1 << (32 - bits)

		_, exists, err := w.store.GetCIDRBlockByCIDRAndNetwork(ctx, podCIDR.CIDR, network)
		if err != nil {
			return fmt.Errorf("failed to check if CIDR exists in store: %w", err)
		}
		if exists {
			w.logger.V(4).Info("CIDR already exists in local DB", "cidr", podCIDR.CIDR)
			continue
		}

		w.logger.Info("Watcher adding new ready podCIDR to local DB", "cidr", podCIDR.CIDR, "network", podCIDR.Network, "availableIPs", availableIPs)
		err = w.store.AddCIDR(ctx, podCIDR.Network, podCIDR.CIDR)
		if err == nil {
			if w.OnCIDRAdded != nil {
				w.OnCIDRAdded(podCIDR.Network, availableIPs)
			}
		} else {
			if errors.Is(err, store.ErrCidrAlreadyExists) {
				w.logger.V(4).Info("CIDR already exists in local DB (race condition)", "cidr", podCIDR.CIDR)
			} else {
				return fmt.Errorf("failed to add CIDR to store: %w", err)
			}
		}
	}
	return nil
}

func (w *Watcher) maybeDeleteCIDRs(ctx context.Context, nnc *nncv1.NodeNetworkConfig, network string) error {
	toBeDeletedBlocks, err := w.store.GetDeletingCIDRBlocks(ctx, network)
	if err != nil {
		return fmt.Errorf("failed to query deleting cidr blocks: %w", err)
	}

	// Create a map for quick lookup of CIDRs in NNC status for the current network
	statusCIDRs := make(map[string]bool)
	for _, podCIDR := range nnc.Status.PodCIDRs {
		if podCIDR.Network == network {
			statusCIDRs[podCIDR.CIDR] = true
		}
	}

	var blocksToDelete []store.CIDRBlock
	for _, block := range toBeDeletedBlocks {
		if !statusCIDRs[block.CIDR] {
			blocksToDelete = append(blocksToDelete, block)
		}
	}

	for _, block := range blocksToDelete {
		err = w.store.DeleteCIDRBlock(ctx, block.ID)
		if err != nil {
			return fmt.Errorf("failed to delete cidr block %d from store: %w", block.ID, err)
		}
		w.logger.Info("Watcher deleted CIDR block from local DB as GCE has released it", "cidrBlockID", block.ID, "cidr", block.CIDR, "network", network)
	}

	return nil
}
