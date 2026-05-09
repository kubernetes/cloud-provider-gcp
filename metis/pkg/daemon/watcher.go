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

type Watcher struct {
	queue       workqueue.TypedRateLimitingInterface[string]
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

	w.logger.Info("Starting Metis Daemon NodeNetworkConfig CRD watcher", "workers", workers)
	defer w.logger.Info("Stopping Metis Daemon NodeNetworkConfig CRD watcher")

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
	w.logger.Info("Syncing NodeNetworkConfig status", "node", w.nodeName, "network", network)

	nnc, err := w.getNodeNetworkConfig(ctx)
	if err != nil {
		return err
	}

	if err := w.addCIDR(ctx, nnc, network); err != nil {
		return err
	}

	return w.maybeDeleteCIDRs(ctx, nnc, network)
}

// getNodeNetworkConfig fetches the NodeNetworkConfig CR.
// It prefers using the lister (cache) for efficiency, but falls back to the API client
// if the lister is not available. This fallback is primarily to support unit tests
// that do not initialize the full informer stack.
func (w *Watcher) getNodeNetworkConfig(ctx context.Context) (*nncv1.NodeNetworkConfig, error) {
	if w.nncLister != nil {
		nnc, err := w.nncLister.Get(w.nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from lister: %w", err)
		}
		return nnc, nil
	}
	if w.nncClient != nil {
		nnc, err := w.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, w.nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from API: %w", err)
		}
		return nnc, nil
	}
	return nil, fmt.Errorf("no client or lister available to fetch NodeNetworkConfig")
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

		w.logger.Info("Adding podCIDR to local DB", "cidr", podCIDR.CIDR, "network", podCIDR.Network)
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

	var blocksToDelete []store.DeletingCIDRBlock
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
		w.logger.Info("Deleted CIDR block from local DB as it was released by GCE", "cidrBlockID", block.ID, "cidr", block.CIDR, "network", network)
	}

	return nil
}
