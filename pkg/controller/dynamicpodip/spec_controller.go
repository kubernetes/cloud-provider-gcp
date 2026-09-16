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

import (
	"context"
	"fmt"
	"net"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// DefaultSpecControllerWorkers is the default number of worker goroutines for the spec controller.
	DefaultSpecControllerWorkers = 4

	specControllerName = "node-network-config-spec-controller"
)

// networkChanges tracks addition CIDR sizes and removal exact CIDRs for a specific network.
type networkChanges struct {
	additions []string
	removals  []string
}

// nodeNetworkChanges is a map of network name to its networkChanges.
type nodeNetworkChanges map[string]networkChanges

// Empty returns true if there are no additions or removals across any network.
func (c nodeNetworkChanges) Empty() bool {
	for _, nc := range c {
		if len(nc.additions) > 0 || len(nc.removals) > 0 {
			return false
		}
	}
	return true
}

// Networks returns a sorted list of network names that have changes.
func (c nodeNetworkChanges) Networks() []string {
	return sets.StringKeySet(c).List()
}

// GetNetwork returns the networkChanges for a specific network.
func (c nodeNetworkChanges) GetNetwork(network string) networkChanges {
	return c[network]
}

// NodeNetworkConfigSpecController ("Write" side / GCE Mutator) evaluates nnc.Spec allocations
// against GCE state and performs GCE VM alias IP mutations.
type NodeNetworkConfigSpecController struct {
	nodeSyncBase

	nncSynced     cache.InformerSynced
	nodeSynced    cache.InformerSynced
	statusTrigger StatusTrigger
	queue         workqueue.TypedRateLimitingInterface[string]
}

// NewSpecController constructs a new NodeNetworkConfigSpecController.
func NewSpecController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncInformer nncinformers.NodeNetworkConfigInformer,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
	gceCache *GCECache,
	statusTrigger StatusTrigger,
) *NodeNetworkConfigSpecController {
	if statusTrigger == nil {
		statusTrigger = &NoopStatusTrigger{}
	}

	c := &NodeNetworkConfigSpecController{
		nodeSyncBase: nodeSyncBase{
			name:       specControllerName,
			kubeClient: kubeClient,
			nncClient:  nncClient,
			nncLister:  nncInformer.Lister(),
			nodeLister: nodeInformer.Lister(),
			gceCloud:   gceCloud,
			gceCache:   gceCache,
		},
		nncSynced:     nncInformer.Informer().HasSynced,
		nodeSynced:    nodeInformer.Informer().HasSynced,
		statusTrigger: statusTrigger,
		queue:         newNodeWorkqueue("dynamic-pod-ip-spec"),
	}

	nncInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueueNodeNetworkConfig,
		UpdateFunc: c.handleNodeNetworkConfigUpdate,
		DeleteFunc: c.enqueueNodeNetworkConfig,
	})

	return c
}

// handleNodeNetworkConfigUpdate enqueues a NodeNetworkConfig only when its spec actually changed.
//
// Only the spec drives GCE mutations. NodeNetworkConfig declares a status subresource, so the API
// server bumps metadata.Generation only for spec changes -- not for status writes, and not for
// metadata-only changes. Filtering on it therefore drops two sources of redundant work:
//
//   - Watch relists, which redeliver every object as an update with an unchanged ResourceVersion.
//     (Because the informer factory is built with a resync period of 0, client-go's own "treat
//     unchanged ResourceVersion as a resync" filter does not suppress these for this listener.)
//   - The status writes that this controller and the status controller perform themselves, which
//     would otherwise re-enqueue this controller roughly twice for every real spec change.
//
// Each redundant sync is not free: it ends by triggering the status controller, which does an
// uncached GCE instances.get.
func (c *NodeNetworkConfigSpecController) handleNodeNetworkConfigUpdate(old, newObj interface{}) {
	oldNNC, oldOK := old.(*nncv1.NodeNetworkConfig)
	newNNC, newOK := newObj.(*nncv1.NodeNetworkConfig)
	if oldOK && newOK && oldNNC.Generation == newNNC.Generation {
		klog.V(5).Infof("Skipping NodeNetworkConfig %q update: spec generation unchanged (%d)", newNNC.Name, newNNC.Generation)
		return
	}
	// Fall through on an unexpected type so that we fail open rather than dropping an update.
	c.enqueueNodeNetworkConfig(newObj)
}

func (c *NodeNetworkConfigSpecController) enqueueNodeNetworkConfig(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Run starts the spec controller workers.
func (c *NodeNetworkConfigSpecController) Run(workers int, stopCh <-chan struct{}) {
	runNodeWorkers(c.name, c.queue, workers, stopCh, c.runWorker, c.nncSynced, c.nodeSynced)
}

func (c *NodeNetworkConfigSpecController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeNetworkConfigSpecController) processNextWorkItem() bool {
	return processNextNodeWorkItem(c.name, c.queue, c.syncNode)
}

func (c *NodeNetworkConfigSpecController) syncNode(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	nnc, providerID, err := c.resolveNode(ctx, key)
	if err != nil || nnc == nil {
		return err
	}

	return c.reconcile(ctx, nnc, providerID)
}

func (c *NodeNetworkConfigSpecController) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	// Retrieve GCE actual state via cache
	ifaces, err := c.gceCache.Get(ctx, nnc.Name, providerID)
	if err != nil {
		klog.Errorf("Failed to get GCE state for node %q: %v", nnc.Name, err)
		c.updateStatusError(ctx, nnc.DeepCopy(), string(nncv1.NodeNetworkConfigInvalidParametersReason), fmt.Sprintf("Failed to get GCE instance: %v", err))
		return err
	}

	// Calculate changes needed between Spec and GCE
	changes, err := c.calculateChanges(nnc, ifaces)
	if err != nil {
		klog.Errorf("Failed to calculate changes for node %q: %v", nnc.Name, err)
		c.updateStatusError(ctx, nnc.DeepCopy(), string(nncv1.NodeNetworkConfigInvalidParametersReason), fmt.Sprintf("Invalid status CIDR: %v", err))
		return err
	}

	// If no mutations needed, trigger status populator and return.
	// The prune still runs: entries can reference ranges that are already gone
	// from GCE, either because we crashed before pruning them last time, because
	// the daemon re-added one it has not yet seen released, or because the range
	// was never on this node at all. None of those produce GCE changes.
	if changes.Empty() {
		klog.V(4).Infof("No GCE changes required for node %q", nnc.Name)
		c.statusTrigger.EnqueueNode(nnc.Name)
		return c.pruneReleasableCIDRs(ctx, nnc, gceCIDRSet(ifaces))
	}

	// Set status condition to Updating before mutating GCE
	nncCopy := nnc.DeepCopy()
	setNNCCondition(nncCopy, string(nncv1.NodeNetworkConfigConditionReady), metav1.ConditionFalse, "Updating", "Updating GCE VM IP alias ranges")
	if err := c.updateNNCStatus(ctx, nncCopy); err != nil {
		return fmt.Errorf("failed to update status condition to Updating: %w", err)
	}

	// Execute GCE VM alias IP mutations
	for _, network := range changes.Networks() {
		netChanges := changes.GetNetwork(network)
		networkURL, err := ResolveNetworkURL(c.gceCloud, network)
		if err != nil {
			klog.Errorf("Failed to resolve network URL for network %q: %v", network, err)
			c.updateStatusError(ctx, nnc.DeepCopy(), string(nncv1.NodeNetworkConfigInvalidParametersReason), fmt.Sprintf("Failed to resolve network URL: %v", err))
			return fmt.Errorf("failed to resolve network URL for network %q: %w", network, err)
		}

		klog.Infof("Applying GCE mutations for node %q, network %q (URL=%q): additions=%v, removals=%v",
			nnc.Name, network, networkURL, netChanges.additions, netChanges.removals)

		err = c.gceCloud.UpdateInstanceAliasIPRanges(ctx, providerID, networkURL, netChanges.additions, netChanges.removals)
		if err != nil {
			klog.Errorf("GCE mutation failed for node %q network %q: %v", nnc.Name, network, err)
			c.updateStatusError(ctx, nnc.DeepCopy(), string(nncv1.NodeNetworkConfigInvalidParametersReason), fmt.Sprintf("GCE mutation failed: %v", err))
			return fmt.Errorf("failed GCE mutation for network %q: %w", network, err)
		}
	}

	// Trigger status controller. Do this before the prune so that publishing the
	// new GCE state is never gated on a spec write succeeding.
	c.statusTrigger.EnqueueNode(nnc.Name)

	// Every removal above succeeded, so the instance now holds the ranges we
	// started with minus those. Derived rather than re-read: a fresh GCE read
	// here would cost a round trip and could also observe ranges added by this
	// pass, whose CIDRs we cannot know until the next reconcile anyway.
	postMutation := gceCIDRSet(ifaces)
	for _, network := range changes.Networks() {
		for _, removed := range changes.GetNetwork(network).removals {
			postMutation.Delete(cidrKey(network, removed))
		}
	}

	return c.pruneReleasableCIDRs(ctx, nnc, postMutation)
}

func cidrCapacity(cidrStr string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return 0, err
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return 0, fmt.Errorf("CIDR %q is not IPv4 (bits=%d), only IPv4 is supported", cidrStr, bits)
	}
	return 1 << (32 - ones), nil
}

// releasableCIDRSet returns the cidrKeys the daemon has asked us to release.
// Entries with an empty network are skipped here; calculateChanges rejects them
// explicitly so the error surfaces with a useful message.
func releasableCIDRSet(nnc *nncv1.NodeNetworkConfig) sets.String {
	releasable := sets.NewString()
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		if rel.Network == "" {
			continue
		}
		releasable.Insert(cidrKey(rel.Network, rel.CIDR))
	}
	return releasable
}

func (c *NodeNetworkConfigSpecController) calculateChanges(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) (nodeNetworkChanges, error) {
	changes := make(nodeNetworkChanges)
	currentCapacity := make(map[string]int)
	activeCIDRs := gceCIDRSet(ifaces)
	releasable := releasableCIDRSet(nnc)

	var capErr error
	forEachAliasRange(ifaces, func(netName, cidr string) {
		if capErr != nil {
			return
		}
		// A range the daemon has asked us to release must not count toward
		// satisfying demand. The daemon stops using it the moment it adds
		// the entry and reduces its own Allocations[].Pods ask by the same
		// capacity, so counting it here would make a scale-up that
		// coincides with a release under-allocate by exactly that much --
		// and nothing re-enqueues us afterwards to correct it.
		if releasable.Has(cidrKey(netName, cidr)) {
			return
		}
		cap, err := cidrCapacity(cidr)
		if err != nil {
			capErr = fmt.Errorf("failed to parse CIDR %q in GCE: %w", cidr, err)
			return
		}
		currentCapacity[netName] += cap
	})
	if capErr != nil {
		return nil, capErr
	}

	// Additions
	for _, alloc := range nnc.Spec.Allocations {
		network := alloc.Network
		if network == "" {
			return nil, fmt.Errorf("allocation has empty network name")
		}
		currentCap := currentCapacity[network]
		desiredPods := int(alloc.Pods)

		if desiredPods > currentCap {
			neededIPs := desiredPods - currentCap
			blocksNeeded := (neededIPs + DefaultCapacity - 1) / DefaultCapacity

			entry := changes[network]
			for i := 0; i < blocksNeeded; i++ {
				entry.additions = append(entry.additions, DefaultBlockSize)
			}
			changes[network] = entry
			klog.V(3).Infof("Node %q network %q needs %d more IPs, requesting %d blocks of size %s", nnc.Name, network, neededIPs, blocksNeeded, DefaultBlockSize)
		}
	}

	// Removals
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		network := rel.Network
		if network == "" {
			return nil, fmt.Errorf("releasable CIDR has empty network name")
		}
		key := cidrKey(network, rel.CIDR)

		if activeCIDRs.Has(key) {
			entry := changes[network]
			entry.removals = append(entry.removals, rel.CIDR)
			changes[network] = entry
			klog.V(3).Infof("Node %q network %q: flagging %q for removal", nnc.Name, network, rel.CIDR)
		}
	}

	return changes, nil
}

// pruneReleasableCIDRs removes entries from Spec.ReleasableCIDRs whose range is
// no longer attached to the instance.
//
// The Metis daemon owns this field and also removes its own entries, once the
// CIDR disappears from Status.PodCIDRs. We are a second remover, not the only
// one. That is safe because neither side ever needs to add on behalf of the
// other, so a read-modify-write that only ever deletes converges regardless of
// how the two interleave. It is also why Update is used rather than a bare
// merge patch: Update carries resourceVersion, so a concurrent daemon write
// loses cleanly with a conflict instead of silently clobbering an addition and
// dropping a release request on the floor.
//
// The predicate is level-triggered on GCE state rather than edge-triggered on
// "the entry whose range we just deleted". That makes it recover from a crash
// between the GCE mutation and this write, clean up entries naming ranges that
// were never on this node, and tolerate the daemon re-adding an entry it has
// not yet observed as released.
//
// presentInGCE must describe the instance *after* this pass's mutations.
func (c *NodeNetworkConfigSpecController) pruneReleasableCIDRs(ctx context.Context, nnc *nncv1.NodeNetworkConfig, presentInGCE sets.String) error {
	// Decide what is stale from this reconcile's consistent view, and apply only
	// that decision below. Re-deriving staleness from the live object inside the
	// retry would judge entries the daemon added after our GCE snapshot was
	// taken, and prune them on the basis of a read that predates them.
	stale := sets.NewString()
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		key := cidrKey(rel.Network, rel.CIDR)
		if !presentInGCE.Has(key) {
			stale.Insert(key)
		}
	}
	if stale.Len() == 0 {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Live read: the lister is stale immediately after our own writes, and
		// writing back a stale copy would clobber concurrent Allocations edits.
		live, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, nnc.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		kept := make([]nncv1.PodCIDR, 0, len(live.Spec.ReleasableCIDRs))
		var pruned []string
		for _, rel := range live.Spec.ReleasableCIDRs {
			if stale.Has(cidrKey(rel.Network, rel.CIDR)) {
				pruned = append(pruned, rel.CIDR)
				continue
			}
			kept = append(kept, rel)
		}
		if len(pruned) == 0 {
			// Already gone, most likely because the daemon got there first.
			return nil
		}

		updated := live.DeepCopy()
		if len(kept) == 0 {
			updated.Spec.ReleasableCIDRs = nil
		} else {
			updated.Spec.ReleasableCIDRs = kept
		}
		if _, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return err
		}
		klog.Infof("Pruned released CIDRs from spec.releasableCIDRs for node %q: %v", nnc.Name, pruned)
		return nil
	})
}

func (c *NodeNetworkConfigSpecController) updateStatusError(ctx context.Context, nnc *nncv1.NodeNetworkConfig, reason, message string) error {
	setNNCCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), metav1.ConditionFalse, reason, message)
	return c.updateNNCStatus(ctx, nnc)
}
