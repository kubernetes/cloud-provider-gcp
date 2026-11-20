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
	"reflect"
	"strings"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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

	// ManagedByLabelKey marks NodeNetworkConfigs whose existence the CCM owns.
	// The status controller only ever deletes objects carrying
	// ManagedByLabelValue, so an NNC created by anything else is left alone.
	ManagedByLabelKey = "networking.gke.io/managed-by"
	// ManagedByLabelValue is the value the CCM stamps on NodeNetworkConfigs it creates.
	ManagedByLabelValue = "cloud-controller-manager"
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

// NodeNetworkConfigStatusController owns the existence of NodeNetworkConfig objects and reconciles
// their status section to match actual GCE VM alias IP range state.
//
// Whether a node should have a NodeNetworkConfig is a single predicate (see nncWanted): always,
// when the spec controller is enabled; otherwise exactly while the node participates in at least
// one Ready additional Network. The controller creates the object when the predicate holds and it
// is missing, and deletes it -- provided it created it -- when the predicate no longer holds. It
// never writes to GCE.
//
// It is driven from three directions. It watches Nodes itself, which is what keeps it alive in
// multi-networking clusters where the spec controller is not enabled; it re-evaluates every node
// when a Network or GKENetworkParamSet changes, since that is what moves the predicate; and it
// exposes EnqueueNode so that the spec controller can hand it a node the moment it has finished
// mutating GCE, rather than waiting for that change to show up indirectly.
type NodeNetworkConfigStatusController struct {
	nodeSyncBase

	// nncAlwaysWanted short-circuits nncWanted to true. It is set when Adaptive
	// Cluster IPAM is enabled: Metis is then present and needs a
	// NodeNetworkConfig on every node for the default network, whether or not
	// any additional Network exists.
	nncAlwaysWanted bool

	nodeSynced cache.InformerSynced
	queue      workqueue.TypedRateLimitingInterface[string]
	clock      clock.Clock
}

// NewStatusController constructs a new NodeNetworkConfigStatusController.
//
// networkResolver may be nil, in which case only nic0 ranges are published, as
// the default network, and no NodeNetworkConfig is created unless
// nncAlwaysWanted is set. When it is non-nil the caller must have wired
// EnqueueAllNodes in as one of its onChange functions, since Network changes
// are otherwise invisible to this controller.
//
// nncAlwaysWanted must be set when the spec controller runs alongside this one;
// see the field for why.
func NewStatusController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncLister nnclisters.NodeNetworkConfigLister,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
	gceCache *GCECache,
	clk clock.Clock,
	networkResolver *NetworkResolver,
	nncAlwaysWanted bool,
) *NodeNetworkConfigStatusController {
	if clk == nil {
		clk = clock.RealClock{}
	}

	c := &NodeNetworkConfigStatusController{
		nodeSyncBase: nodeSyncBase{
			name:            statusControllerName,
			kubeClient:      kubeClient,
			nncClient:       nncClient,
			nncLister:       nncLister,
			nodeLister:      nodeInformer.Lister(),
			gceCloud:        gceCloud,
			gceCache:        gceCache,
			networkResolver: networkResolver,
		},
		nncAlwaysWanted: nncAlwaysWanted,
		nodeSynced:      nodeInformer.Informer().HasSynced,
		queue:           newNodeWorkqueue("nnc-status-populator"),
		clock:           clk,
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueueNode,
		UpdateFunc: c.handleNodeUpdate,
		// No DeleteFunc: there is no status left to publish for a node that is
		// gone, and the NodeNetworkConfig is removed along with it through its
		// owner reference.
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

// EnqueueAllNodes enqueues every Node currently in the lister. It is meant to
// be the NetworkResolver's onChange function: a Network appearing, becoming
// Ready, ceasing to be Ready, or disappearing can change which nodes want a
// NodeNetworkConfig and which ranges are publishable on all of them. Networks
// are few and change rarely, so re-evaluating every node is cheap, and far
// simpler than working out which nodes a given Network could affect.
//
// Before the Node informer has synced the lister may be incomplete; that is
// fine, because the informer's own Add events cover whatever it has not
// delivered yet, and workers do not start until both caches are synced.
//
// It is safe to call on a nil controller, which is what lets the resolver be
// constructed, with this wired in, before the controller that owns it.
func (c *NodeNetworkConfigStatusController) EnqueueAllNodes() {
	if c == nil || c.queue == nil {
		return
	}
	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		runtime.HandleError(fmt.Errorf("%s: failed to list nodes after a network change: %v", c.name, err))
		return
	}
	for _, node := range nodes {
		c.queue.Add(node.Name)
	}
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
	syncs := append([]cache.InformerSynced{c.nodeSynced}, c.networkResolver.Synced()...)
	runNodeWorkers(c.name, c.queue, workers, stopCh, c.runWorker, syncs...)
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

	node, err := c.getReadyNode(ctx, key)
	if err != nil {
		return err
	}
	nnc, err := c.getNodeNetworkConfig(ctx, key)
	if err != nil {
		return err
	}

	// Retrieve GCE actual state via ForceGet to ensure fresh status population.
	// This is needed both to decide whether the node wants a NodeNetworkConfig
	// and, if so, to publish its ranges.
	ifaces, err := c.gceCache.ForceGet(ctx, node.Name, node.Spec.ProviderID)
	if err != nil {
		klog.Errorf("Failed to get GCE state for node %q: %v", node.Name, err)
		return err
	}

	wanted := c.nncWanted(ifaces)
	switch {
	case nnc == nil && !wanted:
		klog.V(4).Infof("%s: node %q wants no NodeNetworkConfig and has none, skipping sync", c.name, node.Name)
		return nil
	case nnc == nil:
		nnc, err = c.createNodeNetworkConfig(ctx, node)
		if err != nil {
			return err
		}
	case !wanted && isManagedByCCM(nnc):
		return c.deleteNodeNetworkConfig(ctx, nnc)
	}

	return c.publishStatus(ctx, nnc, ifaces)
}

// nncWanted reports whether a node with the given interfaces should have a
// NodeNetworkConfig:
//
//	nncWanted = nncAlwaysWanted || node has >= 1 Ready additional Network
//	                               matching one of its interfaces
//
// The second clause is evaluated over Networks rather than alias IP ranges: a
// Ready Network whose interface has no range attached yet still warrants an
// object for the ranges to be published into. Matching uses the same
// NetworkAttachment / (VPC, VPCSubnet) rules as range attribution.
//
// Because nncAlwaysWanted is fixed for the life of the process, the predicate
// can only go false when the spec controller is off -- i.e. in multi-network
// clusters without Adaptive Cluster IPAM -- which is exactly the configuration
// where the CCM is the sole writer and must clean up after a Network is
// rolled back or removed.
func (c *NodeNetworkConfigStatusController) nncWanted(ifaces []*networkInterface) bool {
	if c.nncAlwaysWanted {
		return true
	}
	for _, params := range c.networkResolver.ReadyAdditionalNetworks() {
		if findMatchingInterface(params, ifaces) != nil {
			return true
		}
	}
	return false
}

// isManagedByCCM reports whether the CCM created this NodeNetworkConfig, and
// so may delete it.
//
// TODO: remove this check, and the label, once the removal of Metis's
// NodeNetworkConfig creation has merged and fully rolled out. At that point
// the CCM is the only creator and every NodeNetworkConfig is its own.
func isManagedByCCM(nnc *nncv1.NodeNetworkConfig) bool {
	return nnc != nil && nnc.Labels[ManagedByLabelKey] == ManagedByLabelValue
}

// createNodeNetworkConfig creates an empty NodeNetworkConfig for node, owned by
// it so that Kubernetes garbage collection removes the object when the node
// goes away, and labelled so that this controller knows it may delete it.
//
// AlreadyExists is not an error: the object may have been created by a
// concurrent worker, or by Metis in clusters that still run its creation path.
// Either way the live object is returned for reconciliation to continue.
func (c *NodeNetworkConfigStatusController) createNodeNetworkConfig(ctx context.Context, node *v1.Node) (*nncv1.NodeNetworkConfig, error) {
	// Deliberately not metav1.NewControllerRef: that sets blockOwnerDeletion,
	// which (a) would make a stuck NodeNetworkConfig hold up Node deletion,
	// the opposite of what we want, and (b) requires update on
	// nodes/finalizers when the OwnerReferencesPermissionEnforcement
	// admission plugin is on, a permission the CCM is not guaranteed to have.
	isController := true
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   node.Name,
			Labels: map[string]string{ManagedByLabelKey: ManagedByLabelValue},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
				Controller: &isController,
			}},
		},
	}
	created, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err == nil {
		klog.Infof("%s: created NodeNetworkConfig for node %q", c.name, node.Name)
		return created, nil
	}
	if !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create NodeNetworkConfig for node %q: %w", node.Name, err)
	}
	klog.V(4).Infof("%s: NodeNetworkConfig for node %q was created concurrently", c.name, node.Name)
	existing, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return existing, nil
}

// deleteNodeNetworkConfig removes a NodeNetworkConfig this controller created
// and that its node no longer wants. The UID precondition guards against
// deleting a replacement object created in the meantime; a precondition
// failure or NotFound simply means there is nothing left to do.
func (c *NodeNetworkConfigStatusController) deleteNodeNetworkConfig(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	klog.Infof("%s: node %q no longer participates in any Ready additional Network, deleting its NodeNetworkConfig", c.name, nnc.Name)
	uid := nnc.UID
	err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Delete(ctx, nnc.Name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid},
	})
	if err != nil && !errors.IsNotFound(err) && !errors.IsConflict(err) {
		return fmt.Errorf("failed to delete NodeNetworkConfig %q: %w", nnc.Name, err)
	}
	return nil
}

// reconcile publishes the GCE state of providerID into nnc's status. It is the
// entry point for callers that already hold a NodeNetworkConfig and do not need
// the existence decision made by syncNode.
func (c *NodeNetworkConfigStatusController) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	// Retrieve GCE actual state via ForceGet to ensure fresh status population.
	ifaces, err := c.gceCache.ForceGet(ctx, nnc.Name, providerID)
	if err != nil {
		klog.Errorf("Failed to get GCE state for node %q: %v", nnc.Name, err)
		return err
	}
	return c.publishStatus(ctx, nnc, ifaces)
}

// publishStatus writes Status.PodCIDRs from ifaces if it differs from what is
// already published.
//
// While any AliasNetworkEndpoint on the node is still CREATING or DELETING the
// snapshot is incomplete, so publication is deferred with a transientANEError
// (requeued with backoff). This is checked here rather than before the
// existence decision in syncNode: creating the NodeNetworkConfig is inert and
// need not wait for ANEs to settle, and deletion cannot coincide with ANEs
// (ANEs imply the spec controller, which implies nncAlwaysWanted).
func (c *NodeNetworkConfigStatusController) publishStatus(ctx context.Context, nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) error {
	if hasTransientANE(ifaces) {
		return &transientANEError{nodeName: nnc.Name}
	}
	if c.statusNeedsSync(nnc, ifaces) {
		return c.syncStatusToGCE(ctx, nnc, ifaces)
	}
	return nil
}

// statusNeedsSync checks if the NNC Status PodCIDRs set differs from the pod
// ranges derivable from the node's interfaces.
func (c *NodeNetworkConfigStatusController) statusNeedsSync(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) bool {
	return !c.publishableCIDRSet(ifaces).Equal(statusCIDRSet(nnc))
}

// forEachPublishedPodCIDR invokes fn for every Pod CIDR this controller
// publishes for the node: the IPv4 alias IP ranges and AliasNetworkEndpoints
// from forEachPodCIDREntry, followed by the IPv6 pod ranges from
// forEachIPv6PodRange.
//
// An IPv6 range's ID is its CIDR and its reuse policy is Reusable: the reuse
// policy records only what kind of GCE resource backs the CIDR (alias IP
// range vs. AliasNetworkEndpoint), and an interface's IPv6 /96 is neither an
// ANE nor dynamically allocated.
func (c *NodeNetworkConfigStatusController) forEachPublishedPodCIDR(ifaces []*networkInterface, fn func(network, id, cidr string, reusePolicy nncv1.PodCIDRType)) {
	c.forEachPodCIDREntry(ifaces, fn)
	c.forEachIPv6PodRange(ifaces, func(network, cidr string) {
		fn(network, cidr, cidr, nncv1.PodCIDRTypeReusable)
	})
}

// forEachIPv6PodRange invokes fn for every (network name, IPv6 /112) pair on
// the node.
//
// IPv6 is attributed the other way round from IPv4. An alias IP range names
// the subnet range it came from, which identifies its Network; an interface's
// IPv6 /96 names nothing, it simply belongs to the interface. So instead of
// walking ranges and asking which Network owns each, this walks Ready Networks
// and asks which interface carries each: if that interface has an IPv6
// address and the Network's StackType includes IPv6, the interface's /112 is
// published under that Network. Several Networks sharing an interface each
// get the same /112, and an IPv4-only Network on a dual-stack interface gets
// nothing, both deliberately.
//
// Networks that are not Ready are skipped, exactly as their IPv4 ranges are:
// NCM sets Ready=False on a Network when IPv6 multi-networking is rolled back,
// and the CCM follows.
func (c *NodeNetworkConfigStatusController) forEachIPv6PodRange(ifaces []*networkInterface, fn func(network, cidr string)) {
	for _, params := range c.networkResolver.ReadyNetworks() {
		if !stackTypeHasIPv6(params.StackType) {
			continue
		}
		iface := interfaceForNetwork(params, ifaces)
		if iface == nil {
			continue
		}
		cidr, ok := ipv6PodRange(iface.IPv6Address)
		if !ok {
			if iface.IPv6Address != "" {
				klog.Warningf("Skipping malformed IPv6 address %q on interface %q for network %q", iface.IPv6Address, iface.Name, params.NetworkName)
			}
			continue
		}
		fn(params.NetworkName, cidr)
	}
}

// publishableCIDRSet returns the cidrKeys of every pod range this controller
// would publish for the node. Unlike gceCIDRSet, which the spec controller
// uses to account for alias IP ranges it manages, this includes IPv6 ranges.
func (c *NodeNetworkConfigStatusController) publishableCIDRSet(ifaces []*networkInterface) sets.String {
	all := sets.NewString()
	c.forEachPublishedPodCIDR(ifaces, func(network, _, cidr string, _ nncv1.PodCIDRType) {
		all.Insert(cidrKey(network, cidr))
	})
	return all
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
	now := metav1.Now()
	return c.updateNNCStatus(ctx, nnc, func(target *nncv1.NodeNetworkConfig) {
		var allActualCIDRs []nncv1.PodCIDR
		readySince := podCIDRReadySince(target)

		c.forEachPublishedPodCIDR(ifaces, func(network, id, cidr string, reusePolicy nncv1.PodCIDRType) {
			transitioned := now
			if prev, ok := readySince[cidrKey(network, cidr)]; ok {
				transitioned = prev
			}
			allActualCIDRs = append(allActualCIDRs, nncv1.PodCIDR{
				Id:          id,
				Network:     network,
				CIDR:        cidr,
				ReusePolicy: reusePolicy,
				Condition: &metav1.Condition{
					Type:               string(nncv1.PodCIDRConditionReady),
					Status:             metav1.ConditionTrue,
					LastTransitionTime: transitioned,
					Reason:             string(nncv1.PodCIDRReadyConditionReady),
					Message:            "Pod CIDR is ready and routed",
				},
			})
		})

		target.Status.PodCIDRs = allActualCIDRs
		_ = setNNCCondition(target, string(nncv1.NodeNetworkConfigConditionReady), metav1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready")
		klog.Infof("Syncing NNC status to GCE state for %q (CIDRs: %d)", target.Name, len(allActualCIDRs))
	})
}
