/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

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
	"math"
	"net"
	"strings"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// DefaultBlockSize is the default CIDR block size we request from GCE (16 IPs).
	DefaultBlockSize = "/28"
	// DefaultCapacity is the capacity of the default block size.
	DefaultCapacity = 16
)

// Controller manages NodeNetworkConfig resources and mutates GCE VM alias IPs.
type Controller struct {
	kubeClient kubernetes.Interface
	nncClient  nncclientset.Interface
	nncLister  nnclisters.NodeNetworkConfigLister
	nncSynced  cache.InformerSynced
	nodeLister corelisters.NodeLister
	nodeSynced cache.InformerSynced
	gceCloud   *gce.Cloud
	queue      workqueue.RateLimitingInterface
}

// NewController creates a new DynamicPodIPController.
func NewController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncInformer nncinformers.NodeNetworkConfigInformer,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) *Controller {
	c := &Controller{
		kubeClient: kubeClient,
		nncClient:  nncClient,
		nncLister:  nncInformer.Lister(),
		nncSynced:  nncInformer.Informer().HasSynced,
		nodeLister: nodeInformer.Lister(),
		nodeSynced: nodeInformer.Informer().HasSynced,
		gceCloud:   gceCloud,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "dynamic-pod-ip"),
	}

	nncInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueNodeNetworkConfig,
		UpdateFunc: func(old, new interface{}) {
			c.enqueueNodeNetworkConfig(new)
		},
		DeleteFunc: c.enqueueNodeNetworkConfig,
	})

	return c
}

// Name returns the name of the controller.
func (c *Controller) Name() string {
	return "dynamic-pod-ip-controller"
}

// Run starts the controller workers.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting Dynamic Pod IP Controller")
	defer klog.Info("Shutting down Dynamic Pod IP Controller")

	if !cache.WaitForCacheSync(stopCh, c.nncSynced, c.nodeSynced) {
		klog.Error("Failed to wait for caches to sync")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncNode(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 5 {
		klog.Warningf("Error syncing NodeNetworkConfig %q, retrying: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}

	klog.Errorf("Dropping NodeNetworkConfig %q out of the queue: %v", key, err)
	c.queue.Forget(key)
	runtime.HandleError(err)
}

func (c *Controller) enqueueNodeNetworkConfig(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func (c *Controller) syncNode(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	nnc, err := c.nncLister.Get(key)
	if errors.IsNotFound(err) {
		klog.V(3).Infof("NodeNetworkConfig %q has been deleted, skipping GCE cleanup (handled by VM deletion)", key)
		return nil
	}
	if err != nil {
		return err
	}

	// Fetch corresponding Node to get ProviderID
	node, err := c.nodeLister.Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("Node %q not found, but NodeNetworkConfig exists. Skipping reconciliation.", key)
			return nil
		}
		return err
	}

	if node.Spec.ProviderID == "" {
		return fmt.Errorf("node %q has no ProviderID, cannot reconcile", key)
	}

	nnc = nnc.DeepCopy()
	return c.reconcile(ctx, nnc, node.Spec.ProviderID)
}

func (c *Controller) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	klog.V(4).Infof("Reconciling NodeNetworkConfig %q with providerID %q", nnc.Name, providerID)

	// 1. Calculate additions and removals based on Spec vs Status.
	additions, removals, err := c.calculateChanges(nnc)
	if err != nil {
		klog.Errorf("Failed to calculate changes for %q: %v", nnc.Name, err)
		return c.updateStatusError(ctx, nnc, string(nncv1.NodeNetworkConfigInvalidParametersReason), err.Error())
	}

	if len(additions) == 0 && len(removals) == 0 {
		klog.V(4).Infof("NodeNetworkConfig %q is already aligned", nnc.Name)
		if c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready") {
			return c.updateNNCStatus(ctx, nnc)
		}
		return nil
	}

	// 2. Set condition to False (Updating) before starting GCE mutation.
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, "Updating", "GCE mutation in progress")
	if err := c.updateNNCStatus(ctx, nnc); err != nil {
		return fmt.Errorf("failed to update status to 'Updating' for %q: %w", nnc.Name, err)
	}

	// 3. Apply changes via GCE Provider sequentially per network.
	var allActualCIDRs []nncv1.PodCIDR

	existingNetworksWithChanges := sets.NewString()
	for netName := range additions {
		existingNetworksWithChanges.Insert(netName)
	}
	for netName := range removals {
		existingNetworksWithChanges.Insert(netName)
	}

	// Add existing CIDRs from networks that had NO changes.
	for _, pc := range nnc.Status.PodCIDRs {
		if !existingNetworksWithChanges.Has(pc.Network) {
			allActualCIDRs = append(allActualCIDRs, pc)
		}
	}

	// Process networks with changes.
	for _, network := range existingNetworksWithChanges.List() {
		adds := additions[network]
		rems := removals[network]
		netURL := c.resolveNetworkURL(network)

		klog.Infof("Applying GCE mutations for node %q, network %q (URL=%q): additions=%v, removals=%v", nnc.Name, network, netURL, adds, rems)

		// Call GCE provider (single network call)
		actualCIDRStrings, err := c.gceCloud.UpdateInstanceAliasIPRanges(ctx, providerID, netURL, adds, rems)
		if err != nil {
			klog.Errorf("GCE mutation failed for node %q, network %q: %v", nnc.Name, network, err)
			c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, string(nncv1.NodeNetworkConfigInvalidParametersReason), err.Error())
			_ = c.updateNNCStatus(ctx, nnc)
			return err // Re-queue
		}

		// Map strings back to nncv1.PodCIDR
		for _, cidr := range actualCIDRStrings {
			allActualCIDRs = append(allActualCIDRs, nncv1.PodCIDR{
				Id:      cidr,
				Network: network,
				CIDR:    cidr,
				Condition: &metav1.Condition{
					Type:               string(nncv1.PodCIDRConditionReady),
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             string(nncv1.PodCIDRReadyConditionReady),
					Message:            "Pod CIDR is ready and routed",
				},
			})
		}
	}

	// 4. Update Status with the new actual state
	nnc.Status.PodCIDRs = allActualCIDRs
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready")

	klog.Infof("Successfully reconciled NodeNetworkConfig %q", nnc.Name)
	return c.updateNNCStatus(ctx, nnc)
}

// resolveNetworkURL maps NNC network name to GCE Network URL.
func (c *Controller) resolveNetworkURL(networkName string) string {
	if networkName == "" || networkName == "default" {
		return c.gceCloud.NetworkURL()
	}
	// Construct guess URL for custom network in the same project
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", c.gceCloud.ProjectID(), networkName)
}

// calculateChanges compares Spec and Status to determine additions (range sizes) and removals (exact CIDRs).
func (c *Controller) calculateChanges(nnc *nncv1.NodeNetworkConfig) (map[string][]string, map[string][]string, error) {
	additions := make(map[string][]string)
	removals := make(map[string][]string)

	// 1. Calculate Additions
	for _, alloc := range nnc.Spec.Allocations {
		network := alloc.Network
		if network == "" {
			network = "default"
		}

		// Calculate current capacity for this network from Status
		currentCapacity := 0
		for _, pc := range nnc.Status.PodCIDRs {
			if pc.Network == network {
				cap, err := cidrCapacity(pc.CIDR)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to parse CIDR %q in status: %w", pc.CIDR, err)
				}
				currentCapacity += cap
			}
		}

		desiredPods := int(alloc.Pods)
		if desiredPods > currentCapacity {
			neededIPs := desiredPods - currentCapacity
			blocksNeeded := int(math.Ceil(float64(neededIPs) / float64(DefaultCapacity)))
			
			for i := 0; i < blocksNeeded; i++ {
				additions[network] = append(additions[network], DefaultBlockSize)
			}
			klog.V(3).Infof("Node %q network %q needs %d more IPs, requesting %d blocks of size %s", nnc.Name, network, neededIPs, blocksNeeded, DefaultBlockSize)
		}
	}

	// 2. Calculate Removals
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		network := rel.Network
		if network == "" {
			network = "default"
		}

		// Verify it exists in status before removing
		exists := false
		for _, pc := range nnc.Status.PodCIDRs {
			if pc.Network == network && pc.CIDR == rel.CIDR {
				exists = true
				break
			}
		}

		if exists {
			removals[network] = append(removals[network], rel.CIDR)
			klog.V(3).Infof("Node %q network %q: flagging %q for removal", nnc.Name, network, rel.CIDR)
		}
	}

	return additions, removals, nil
}

func (c *Controller) updateNNCStatus(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	_, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(ctx, nnc, metav1.UpdateOptions{})
	return err
}

func (c *Controller) updateStatusError(ctx context.Context, nnc *nncv1.NodeNetworkConfig, reason, message string) error {
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, reason, message)
	return c.updateNNCStatus(ctx, nnc)
}

// setCondition updates or appends the condition. Returns true if status changed.
func (c *Controller) setCondition(nnc *nncv1.NodeNetworkConfig, cType string, status corev1.ConditionStatus, reason, message string) bool {
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

func cidrCapacity(cidrStr string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return 0, err
	}
	ones, bits := ipNet.Mask.Size()
	if bits == 32 { // IPv4
		return 1 << (32 - ones), nil
	}
	return 0, nil
}

func lastComponent(s string) string {
	if parts := strings.Split(s, "/"); len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return s
}
