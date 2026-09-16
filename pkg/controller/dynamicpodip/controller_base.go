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

package dynamicpodip

// This file holds the machinery shared by the spec and status controllers.
//
// The two remain separate controllers with separate workqueues on purpose: they
// are enabled independently (multi-networking clusters populate status only;
// Adaptive Cluster IPAM clusters also mutate GCE from spec), they have
// different triggers, and a stuck GCE mutation must not block status
// publication. What they do share is the plumbing -- how a key becomes a
// (NodeNetworkConfig, providerID) pair, how failures are requeued, and how
// status is written back -- and that is what lives here.

import (
	"context"
	"fmt"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	workqueueBaseDelay = 5 * time.Millisecond
	workqueueMaxDelay  = 1000 * time.Second
	workqueueQPS       = 10
	workqueueBurst     = 100
)

// newNodeWorkqueue builds the rate-limited workqueue both controllers use.
//
// name is the workqueue metrics name, which is deliberately distinct from the
// controller name: it is part of the exported metric series, so the two should
// not be accidentally unified.
func newNodeWorkqueue(name string) workqueue.TypedRateLimitingInterface[string] {
	rateLimiter := workqueue.NewTypedMaxOfRateLimiter[string](
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](workqueueBaseDelay, workqueueMaxDelay),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(workqueueQPS), workqueueBurst)},
	)
	return workqueue.NewTypedRateLimitingQueueWithConfig[string](
		rateLimiter,
		workqueue.TypedRateLimitingQueueConfig[string]{Name: name},
	)
}

// nodeSyncBase carries the state both controllers need to turn a workqueue key
// into something reconcilable, and to write the result back. It is embedded, so
// its fields and methods are promoted onto each controller.
type nodeSyncBase struct {
	// name identifies the controller in logs and requeue messages.
	name string

	kubeClient kubernetes.Interface
	nncClient  nncclientset.Interface
	nncLister  nnclisters.NodeNetworkConfigLister
	nodeLister corelisters.NodeLister
	gceCloud   *gce.Cloud
	gceCache   *GCECache
}

// Name returns the name of the controller.
func (b *nodeSyncBase) Name() string {
	return b.name
}

// resolveNode turns a workqueue key into the NodeNetworkConfig to reconcile and
// the provider ID of its backing Node.
//
// Both lookups consult the lister first and fall back to a live read, because a
// key can be enqueued before the informer has caught up -- most often by the
// spec controller triggering the status controller immediately after its own
// write.
//
// A nil NodeNetworkConfig with a nil error means the object is gone and the key
// should be dropped. A *nodeNotReadyError means the Node has not finished
// bootstrapping and the key should be requeued with backoff.
//
// The returned object is a deep copy, so callers may mutate it freely without
// corrupting the informer cache.
func (b *nodeSyncBase) resolveNode(ctx context.Context, key string) (*nncv1.NodeNetworkConfig, string, error) {
	nnc, err := b.nncLister.Get(key)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, "", err
		}
		nnc, err = b.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, key, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				klog.V(4).Infof("%s: NodeNetworkConfig %q not found, skipping sync", b.name, key)
				return nil, "", nil
			}
			return nil, "", err
		}
	}

	node, err := b.nodeLister.Get(nnc.Name)
	if err != nil {
		if errors.IsNotFound(err) && b.kubeClient != nil {
			node, err = b.kubeClient.CoreV1().Nodes().Get(ctx, nnc.Name, metav1.GetOptions{})
		}
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, "", &nodeNotReadyError{nodeName: nnc.Name, reason: "node not found"}
			}
			return nil, "", fmt.Errorf("failed to get node %q: %w", nnc.Name, err)
		}
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		return nil, "", &nodeNotReadyError{nodeName: nnc.Name, reason: "node has empty Spec.ProviderID"}
	}

	return nnc.DeepCopy(), providerID, nil
}

// updateNNCStatus writes the status subresource.
func (b *nodeSyncBase) updateNNCStatus(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	_, err := b.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(ctx, nnc, metav1.UpdateOptions{})
	return err
}

// runNodeWorkers is the Run body shared by both controllers. cacheSyncs may be
// empty for a controller that is purely trigger-driven and therefore has no
// informer of its own to wait on.
func runNodeWorkers(
	name string,
	queue workqueue.TypedRateLimitingInterface[string],
	workers int,
	stopCh <-chan struct{},
	worker func(),
	cacheSyncs ...cache.InformerSynced,
) {
	defer runtime.HandleCrash()
	defer queue.ShutDown()

	klog.Infof("Starting %s workers", name)

	if len(cacheSyncs) > 0 && !cache.WaitForNamedCacheSync(name, stopCh, cacheSyncs...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(worker, time.Second, stopCh)
	}

	<-stopCh
	klog.Infof("Stopping %s workers", name)
}

// processNextNodeWorkItem pops one key and dispatches it to syncNode, applying
// the requeue policy shared by both controllers.
//
// A nodeNotReadyError is requeued with backoff but not reported as a controller
// error: it is the expected state while a node bootstraps. Requeuing is what
// makes it recoverable, since neither controller watches Nodes -- forgetting
// the key would strand that node's NodeNetworkConfig indefinitely.
func processNextNodeWorkItem(
	name string,
	queue workqueue.TypedRateLimitingInterface[string],
	syncNode func(key string) error,
) bool {
	key, shutdown := queue.Get()
	if shutdown {
		return false
	}
	defer queue.Done(key)

	err := syncNode(key)
	if err == nil {
		queue.Forget(key)
		return true
	}

	if notReady, ok := err.(*nodeNotReadyError); ok {
		klog.V(4).Infof("%s: deferring sync for %q: %v", name, key, notReady)
		queue.AddRateLimited(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("%s: error syncing node %q: %v", name, key, err))
	queue.AddRateLimited(key)
	return true
}

// setNNCCondition upserts a condition by type, returning whether anything
// changed.
//
// This delegates to the upstream helper rather than hand-rolling the upsert so
// that LastTransitionTime follows the API convention: it moves only when Status
// actually transitions, not when the Reason or Message is reworded. Consumers
// treat that field as "how long have we been in this state", so restamping it
// on a message change silently resets any alert watching for a node stuck
// not-ready.
func setNNCCondition(nnc *nncv1.NodeNetworkConfig, cType string, status metav1.ConditionStatus, reason, message string) bool {
	return apimeta.SetStatusCondition(&nnc.Status.Conditions, metav1.Condition{
		Type:    cType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

// cidrKey is the canonical identity for an alias IP range within a node: the
// network it belongs to plus the range itself. Used to join GCE state against
// Spec.ReleasableCIDRs and Status.PodCIDRs.
func cidrKey(network, cidr string) string {
	return fmt.Sprintf("%s/%s", network, cidr)
}

// forEachAliasRange invokes fn for every (network name, CIDR) pair attached to
// the instance.
//
// Interfaces whose network URL cannot be parsed are skipped with a warning, so
// callers must treat the enumeration as "known to be present" rather than
// exhaustive. In particular, never infer that a range is absent from GCE on the
// strength of this alone when the consequence is destructive.
func forEachAliasRange(ifaces []*networkInterface, fn func(network, cidr string)) {
	for _, iface := range ifaces {
		netName, err := ExtractNetworkName(iface.Network)
		if err != nil {
			klog.Warningf("Failed to extract network name from URL %q: %v", iface.Network, err)
			continue
		}
		for _, cidr := range iface.AliasIPRanges {
			fn(netName, cidr)
		}
	}
}

// gceCIDRSet returns the cidrKeys of every alias IP range attached to the
// instance.
func gceCIDRSet(ifaces []*networkInterface) sets.String {
	active := sets.NewString()
	forEachAliasRange(ifaces, func(network, cidr string) {
		active.Insert(cidrKey(network, cidr))
	})
	return active
}

// statusCIDRSet returns the cidrKeys currently published in Status.PodCIDRs.
func statusCIDRSet(nnc *nncv1.NodeNetworkConfig) sets.String {
	published := sets.NewString()
	for _, pc := range nnc.Status.PodCIDRs {
		published.Insert(cidrKey(pc.Network, pc.CIDR))
	}
	return published
}
