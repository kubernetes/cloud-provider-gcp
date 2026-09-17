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
	"reflect"
	"strings"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

const (
	// DefaultStatusControllerWorkers is the default number of worker goroutines for the status controller.
	DefaultStatusControllerWorkers = 4

	statusControllerName = "node-network-config-status-controller"
)

// StatusTrigger defines an asynchronous interface to request an NNC status refresh for a node.
type StatusTrigger interface {
	// EnqueueNode requests an asynchronous status sync for the specified node.
	// If status population is disabled, this call is a safe no-op.
	EnqueueNode(nodeName string)
}

// NoopStatusTrigger is a safe no-op implementation of StatusTrigger used when status population is disabled.
type NoopStatusTrigger struct{}

// EnqueueNode performs a no-op when status population is disabled.
func (n *NoopStatusTrigger) EnqueueNode(nodeName string) {}

// NodeNetworkConfigStatusController reconciles the status section of NodeNetworkConfig CRDs to
// match actual GCE VM alias IP range state.
//
// It is driven from two directions. It watches Nodes itself, which is what keeps it alive in
// multi-networking clusters where the spec controller is not enabled; and it exposes EnqueueNode so
// that the spec controller can hand it a node the moment it has finished mutating GCE, rather than
// waiting for that change to show up indirectly.
type NodeNetworkConfigStatusController struct {
	nodeSyncBase

	nodeSynced cache.InformerSynced
	queue      workqueue.TypedRateLimitingInterface[string]
	clock      clock.Clock
}

// NewStatusController constructs a new NodeNetworkConfigStatusController.
func NewStatusController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncLister nnclisters.NodeNetworkConfigLister,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
	gceCache *GCECache,
	clk clock.Clock,
) *NodeNetworkConfigStatusController {
	if clk == nil {
		clk = clock.RealClock{}
	}

	c := &NodeNetworkConfigStatusController{
		nodeSyncBase: nodeSyncBase{
			name:       statusControllerName,
			kubeClient: kubeClient,
			nncClient:  nncClient,
			nncLister:  nncLister,
			nodeLister: nodeInformer.Lister(),
			gceCloud:   gceCloud,
			gceCache:   gceCache,
		},
		nodeSynced: nodeInformer.Informer().HasSynced,
		queue:      newNodeWorkqueue("nnc-status-populator"),
		clock:      clk,
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueueNode,
		UpdateFunc: c.handleNodeUpdate,
		// No DeleteFunc: there is no status left to publish for a node that is
		// gone, and the NodeNetworkConfig is removed along with it.
	})

	return c
}

// EnqueueNode requests an asynchronous status sync for the specified node name.
func (c *NodeNetworkConfigStatusController) EnqueueNode(nodeName string) {
	if c == nil || c.queue == nil {
		return
	}
	c.queue.Add(nodeName)
}

// enqueueNode is the informer Add handler.
func (c *NodeNetworkConfigStatusController) enqueueNode(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// handleNodeUpdate enqueues a Node only when something this controller could act on has changed.
//
// Filtering matters more here than it looks: every sync performs an uncached GCE instances.get via
// ForceGet, before statusNeedsSync is even consulted. An unfiltered handler would therefore turn
// each node's routine kubelet status write -- which happens periodically whether or not anything
// changed -- into a GCE API call, scaling with cluster size for no benefit.
//
// The predicate deliberately mirrors what the cloud CIDR allocator already treats as significant,
// since that is the trigger this handler replaces. Note that Spec.PodCIDRs is *not* part of it:
// NodeNetworkConfig status covers alias ranges on every NIC, and secondary network ranges never
// appear there, so keying on it would miss precisely the multi-network case.
func (c *NodeNetworkConfigStatusController) handleNodeUpdate(old, newObj interface{}) {
	oldNode, oldOK := old.(*v1.Node)
	newNode, newOK := newObj.(*v1.Node)
	if oldOK && newOK && !nodeNeedsStatusSync(oldNode, newNode) {
		return
	}
	// Fall through on an unexpected type so that we fail open rather than dropping an update.
	c.enqueueNode(newObj)
}

// nodeNeedsStatusSync reports whether a Node update could change what this controller publishes.
func nodeNeedsStatusSync(oldNode, newNode *v1.Node) bool {
	// Until a ProviderID exists there is no instance to read, and syncNode rejects the node as not
	// ready. The transition is the moment it becomes actionable.
	if oldNode.Spec.ProviderID != newNode.Spec.ProviderID {
		return true
	}
	return multiNetworkStateChanged(oldNode, newNode)
}

// multiNetworkStateChanged reports whether the multi-network state the CCM maintains on a Node has
// changed.
//
// This duplicates nodeMultiNetworkChanged in the nodeipam/ipam package rather than sharing it:
// that package imports this one for the status trigger, so depending on it here would create an
// import cycle. Keep the two in sync.
func multiNetworkStateChanged(oldNode, newNode *v1.Node) bool {
	if !reflect.DeepEqual(multiNetworkAnnotations(oldNode.GetAnnotations()), multiNetworkAnnotations(newNode.GetAnnotations())) {
		return true
	}
	return !reflect.DeepEqual(multiNetworkCapacity(oldNode.Status.Capacity), multiNetworkCapacity(newNode.Status.Capacity))
}

func multiNetworkAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	filtered := map[string]string{}
	for _, key := range []string{
		networkv1.NodeNetworkAnnotationKey,
		networkv1.MultiNetworkAnnotationKey,
		networkv1.NorthInterfacesAnnotationKey,
	} {
		if val, ok := annotations[key]; ok {
			filtered[key] = val
		}
	}
	return filtered
}

func multiNetworkCapacity(capacity v1.ResourceList) v1.ResourceList {
	if capacity == nil {
		return nil
	}
	filtered := v1.ResourceList{}
	for k, v := range capacity {
		name := k.String()
		if strings.HasPrefix(name, networkv1.NetworkResourceKeyPrefix) && strings.HasSuffix(name, ".IP") {
			filtered[k] = v.DeepCopy()
		}
	}
	return filtered
}

// Run starts the status controller workers.
func (c *NodeNetworkConfigStatusController) Run(workers int, stopCh <-chan struct{}) {
	runNodeWorkers(c.name, c.queue, workers, stopCh, c.runWorker, c.nodeSynced)
}

func (c *NodeNetworkConfigStatusController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeNetworkConfigStatusController) processNextWorkItem() bool {
	return processNextNodeWorkItem(c.name, c.queue, c.syncNode)
}

func (c *NodeNetworkConfigStatusController) syncNode(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	nnc, providerID, err := c.resolveNode(ctx, key)
	if err != nil || nnc == nil {
		return err
	}

	return c.reconcile(ctx, nnc, providerID)
}

func (c *NodeNetworkConfigStatusController) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	// Retrieve GCE actual state via ForceGet to ensure fresh status population.
	ifaces, err := c.gceCache.ForceGet(ctx, nnc.Name, providerID)
	if err != nil {
		klog.Errorf("Failed to get GCE state for node %q: %v", nnc.Name, err)
		return err
	}

	// Sync status if needed
	if c.statusNeedsSync(nnc, ifaces) {
		return c.syncStatusToGCE(ctx, nnc, ifaces)
	}

	return nil
}

// statusNeedsSync checks if the NNC Status PodCIDRs set differs from the actual GCE alias IP ranges.
func (c *NodeNetworkConfigStatusController) statusNeedsSync(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) bool {
	return !gceCIDRSet(ifaces).Equal(statusCIDRSet(nnc))
}

// podCIDRReadySince indexes, by range, the time at which each currently
// published pod CIDR last became Ready.
//
// syncStatusToGCE rebuilds Status.PodCIDRs wholesale from the GCE snapshot, so
// without this every range in the list would get a fresh LastTransitionTime
// whenever any single range is added or removed. Only entries that already read
// Ready/True are carried forward; anything else is a real transition and must be
// restamped.
func podCIDRReadySince(nnc *nncv1.NodeNetworkConfig) map[string]metav1.Time {
	since := make(map[string]metav1.Time, len(nnc.Status.PodCIDRs))
	for _, pc := range nnc.Status.PodCIDRs {
		if pc.Condition == nil || pc.Condition.LastTransitionTime.IsZero() {
			continue
		}
		if pc.Condition.Type != string(nncv1.PodCIDRConditionReady) || pc.Condition.Status != metav1.ConditionTrue {
			continue
		}
		since[cidrKey(pc.Network, pc.CIDR)] = pc.Condition.LastTransitionTime
	}
	return since
}

// syncStatusToGCE updates the NNC Status PodCIDRs to match the actual GCE state.
func (c *NodeNetworkConfigStatusController) syncStatusToGCE(ctx context.Context, nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) error {
	var allActualCIDRs []nncv1.PodCIDR

	readySince := podCIDRReadySince(nnc)
	now := metav1.Now()

	forEachAliasRange(ifaces, func(network, cidr string) {
		transitioned := now
		if prev, ok := readySince[cidrKey(network, cidr)]; ok {
			transitioned = prev
		}
		allActualCIDRs = append(allActualCIDRs, nncv1.PodCIDR{
			Id:      cidr,
			Network: network,
			CIDR:    cidr,
			Condition: &metav1.Condition{
				Type:               string(nncv1.PodCIDRConditionReady),
				Status:             metav1.ConditionTrue,
				LastTransitionTime: transitioned,
				Reason:             string(nncv1.PodCIDRReadyConditionReady),
				Message:            "Pod CIDR is ready and routed",
			},
		})
	})

	nncCopy := nnc.DeepCopy()
	nncCopy.Status.PodCIDRs = allActualCIDRs
	_ = setNNCCondition(nncCopy, string(nncv1.NodeNetworkConfigConditionReady), metav1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready")

	klog.Infof("Syncing NNC status to GCE state for %q (CIDRs: %d)", nncCopy.Name, len(allActualCIDRs))
	return c.updateNNCStatus(ctx, nncCopy)
}
