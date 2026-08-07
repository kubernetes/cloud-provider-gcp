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
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// DefaultSpecControllerWorkers is the default number of worker goroutines for the spec controller.
	DefaultSpecControllerWorkers = 4

	specWorkqueueBaseDelay = 5 * time.Millisecond
	specWorkqueueMaxDelay  = 1000 * time.Second
	specWorkqueueQPS       = 10
	specWorkqueueBurst     = 100
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
	kubeClient    kubernetes.Interface
	nncClient     nncclientset.Interface
	nncLister     nnclisters.NodeNetworkConfigLister
	nncSynced     cache.InformerSynced
	nodeLister    corelisters.NodeLister
	nodeSynced    cache.InformerSynced
	gceCloud      *gce.Cloud
	gceCache      *GCECache
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

	rateLimiter := workqueue.NewTypedMaxOfRateLimiter[string](
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](specWorkqueueBaseDelay, specWorkqueueMaxDelay),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(specWorkqueueQPS), specWorkqueueBurst)},
	)

	c := &NodeNetworkConfigSpecController{
		kubeClient:    kubeClient,
		nncClient:     nncClient,
		nncLister:     nncInformer.Lister(),
		nncSynced:     nncInformer.Informer().HasSynced,
		nodeLister:    nodeInformer.Lister(),
		nodeSynced:    nodeInformer.Informer().HasSynced,
		gceCloud:      gceCloud,
		gceCache:      gceCache,
		statusTrigger: statusTrigger,
		queue:         workqueue.NewTypedRateLimitingQueueWithConfig[string](rateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "dynamic-pod-ip-spec"}),
	}

	nncInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueNodeNetworkConfig,
		UpdateFunc: func(old, newObj interface{}) {
			c.enqueueNodeNetworkConfig(newObj)
		},
		DeleteFunc: c.enqueueNodeNetworkConfig,
	})

	return c
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
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting NodeNetworkConfig Spec Controller workers")

	if !cache.WaitForNamedCacheSync("dynamic-pod-ip-spec", stopCh, c.nncSynced, c.nodeSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping NodeNetworkConfig Spec Controller workers")
}

func (c *NodeNetworkConfigSpecController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeNetworkConfigSpecController) processNextWorkItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncNode(key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("error syncing NNC spec for node %q: %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}

// Name returns the name of the controller.
func (c *NodeNetworkConfigSpecController) Name() string {
	return "node-network-config-spec-controller"
}

func (c *NodeNetworkConfigSpecController) syncNode(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// Get the NodeNetworkConfig CRD (check lister first, fall back to API client)
	nnc, err := c.nncLister.Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			nnc, err = c.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, key, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					klog.V(4).Infof("NodeNetworkConfig %q not found, skipping spec evaluation", key)
					return nil
				}
				return err
			}
		} else {
			return err
		}
	}

	// Get the Node to extract ProviderID
	node, err := c.nodeLister.Get(nnc.Name)
	if err != nil {
		if errors.IsNotFound(err) && c.kubeClient != nil {
			node, err = c.kubeClient.CoreV1().Nodes().Get(ctx, nnc.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					klog.V(4).Infof("Node %q not found, skipping spec evaluation", nnc.Name)
					return nil
				}
				return fmt.Errorf("failed to get node %q: %w", nnc.Name, err)
			}
		} else {
			if errors.IsNotFound(err) {
				klog.V(4).Infof("Node %q not found, skipping spec evaluation", nnc.Name)
				return nil
			}
			return fmt.Errorf("failed to get node %q: %w", nnc.Name, err)
		}
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		klog.Warningf("Node %q has empty Spec.ProviderID, skipping spec evaluation", nnc.Name)
		return nil
	}

	return c.reconcile(ctx, nnc.DeepCopy(), providerID)
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

	// If no mutations needed, trigger status populator and return
	if changes.Empty() {
		klog.V(4).Infof("No GCE changes required for node %q", nnc.Name)
		c.statusTrigger.EnqueueNode(nnc.Name)
		return nil
	}

	// Set status condition to Updating before mutating GCE
	nncCopy := nnc.DeepCopy()
	c.setCondition(nncCopy, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, "Updating", "Updating GCE VM IP alias ranges")
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

	// Trigger status controller
	c.statusTrigger.EnqueueNode(nnc.Name)

	return nil
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

func (c *NodeNetworkConfigSpecController) calculateChanges(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) (nodeNetworkChanges, error) {
	changes := make(nodeNetworkChanges)
	currentCapacity := make(map[string]int)
	activeCIDRs := sets.NewString()

	for _, iface := range ifaces {
		netName, err := ExtractNetworkName(iface.Network)
		if err != nil {
			klog.Warningf("Failed to extract network name from URL %q: %v", iface.Network, err)
			continue
		}
		for _, cidr := range iface.AliasIPRanges {
			cap, err := cidrCapacity(cidr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse CIDR %q in GCE: %w", cidr, err)
			}
			currentCapacity[netName] += cap
			activeCIDRs.Insert(fmt.Sprintf("%s/%s", netName, cidr))
		}
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
		key := fmt.Sprintf("%s/%s", network, rel.CIDR)

		if activeCIDRs.Has(key) {
			entry := changes[network]
			entry.removals = append(entry.removals, rel.CIDR)
			changes[network] = entry
			klog.V(3).Infof("Node %q network %q: flagging %q for removal", nnc.Name, network, rel.CIDR)
		}
	}

	return changes, nil
}

func (c *NodeNetworkConfigSpecController) updateNNCStatus(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	_, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(ctx, nnc, metav1.UpdateOptions{})
	return err
}

func (c *NodeNetworkConfigSpecController) updateStatusError(ctx context.Context, nnc *nncv1.NodeNetworkConfig, reason, message string) error {
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, reason, message)
	return c.updateNNCStatus(ctx, nnc)
}

func (c *NodeNetworkConfigSpecController) setCondition(nnc *nncv1.NodeNetworkConfig, cType string, status corev1.ConditionStatus, reason, message string) bool {
	now := metav1.Now()
	newCond := metav1.Condition{
		Type:               cType,
		Status:             metav1.ConditionStatus(status),
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	for i, cond := range nnc.Status.Conditions {
		if cond.Type == cType {
			if cond.Status == newCond.Status && cond.Reason == newCond.Reason && cond.Message == newCond.Message {
				return false
			}
			nnc.Status.Conditions[i] = newCond
			return true
		}
	}

	nnc.Status.Conditions = append(nnc.Status.Conditions, newCond)
	return true
}
