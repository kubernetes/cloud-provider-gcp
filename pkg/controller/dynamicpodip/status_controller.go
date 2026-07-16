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
	"strings"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"golang.org/x/time/rate"
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

// NodeNetworkConfigStatusController reconciles the status section of NodeNetworkConfig CRDs to match actual GCE VM alias IP range state.
type NodeNetworkConfigStatusController struct {
	kubeClient kubernetes.Interface
	nncClient  nncclientset.Interface
	nncLister  nnclisters.NodeNetworkConfigLister
	nodeLister corelisters.NodeLister
	gceCloud   *gce.Cloud
	gceCache   *GCECache
	queue      workqueue.TypedRateLimitingInterface[string]
	clock      clock.Clock
}

// NewStatusController constructs a new NodeNetworkConfigStatusController.
func NewStatusController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncLister nnclisters.NodeNetworkConfigLister,
	nodeLister corelisters.NodeLister,
	gceCloud *gce.Cloud,
	gceCache *GCECache,
	clk clock.Clock,
) *NodeNetworkConfigStatusController {
	if clk == nil {
		clk = clock.RealClock{}
	}

	rateLimiter := workqueue.NewTypedMaxOfRateLimiter[string](
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)

	return &NodeNetworkConfigStatusController{
		kubeClient: kubeClient,
		nncClient:  nncClient,
		nncLister:  nncLister,
		nodeLister: nodeLister,
		gceCloud:   gceCloud,
		gceCache:   gceCache,
		queue:      workqueue.NewTypedRateLimitingQueueWithConfig[string](rateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "nnc-status-populator"}),
		clock:      clk,
	}
}

// Name returns the name of the controller.
func (c *NodeNetworkConfigStatusController) Name() string {
	return "node-network-config-status-controller"
}

// EnqueueNode requests an asynchronous status sync for the specified node name.
func (c *NodeNetworkConfigStatusController) EnqueueNode(nodeName string) {
	if c == nil || c.queue == nil {
		return
	}
	c.queue.Add(nodeName)
}

// Run starts the status controller workers.
func (c *NodeNetworkConfigStatusController) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting NodeNetworkConfig Status Controller workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping NodeNetworkConfig Status Controller workers")
}

func (c *NodeNetworkConfigStatusController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeNetworkConfigStatusController) processNextWorkItem() bool {
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

	runtime.HandleError(fmt.Errorf("error syncing NNC status for node %q: %v", key, err))
	c.queue.AddRateLimited(key)
	return true
}

func (c *NodeNetworkConfigStatusController) syncNode(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// Get the NodeNetworkConfig CRD (check lister first, fall back to API client)
	nnc, err := c.nncLister.Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			nnc, err = c.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, key, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					klog.V(4).Infof("NodeNetworkConfig %q not found, skipping status sync", key)
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
					klog.V(4).Infof("Node %q not found, skipping status sync", nnc.Name)
					return nil
				}
				return fmt.Errorf("failed to get node %q: %w", nnc.Name, err)
			}
		} else {
			if errors.IsNotFound(err) {
				klog.V(4).Infof("Node %q not found, skipping status sync", nnc.Name)
				return nil
			}
			return fmt.Errorf("failed to get node %q: %w", nnc.Name, err)
		}
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		klog.Warningf("Node %q has empty Spec.ProviderID, skipping status sync", nnc.Name)
		return nil
	}

	return c.reconcile(ctx, nnc.DeepCopy(), providerID)
}

func (c *NodeNetworkConfigStatusController) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	// Retrieve GCE actual state via cache.
	ifaces, err := c.gceCache.Get(ctx, nnc.Name, providerID)
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
	gceCIDRs := sets.NewString()
	for _, iface := range ifaces {
		netName := c.mapURLToNetworkName(iface.Network, nnc)
		for _, cidr := range iface.AliasIPRanges {
			gceCIDRs.Insert(fmt.Sprintf("%s/%s", netName, cidr))
		}
	}

	statusCIDRs := sets.NewString()
	for _, pc := range nnc.Status.PodCIDRs {
		statusCIDRs.Insert(fmt.Sprintf("%s/%s", pc.Network, pc.CIDR))
	}

	return !gceCIDRs.Equal(statusCIDRs)
}

// syncStatusToGCE updates the NNC Status PodCIDRs to match the actual GCE state.
func (c *NodeNetworkConfigStatusController) syncStatusToGCE(ctx context.Context, nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) error {
	var allActualCIDRs []nncv1.PodCIDR

	managedNetworks := sets.NewString()
	managedNetworks.Insert(c.resolveNetworkURL(""))
	managedNetworks.Insert(c.resolveNetworkURL("default"))
	for _, alloc := range nnc.Spec.Allocations {
		managedNetworks.Insert(c.resolveNetworkURL(alloc.Network))
	}
	for _, pc := range nnc.Status.PodCIDRs {
		managedNetworks.Insert(c.resolveNetworkURL(pc.Network))
	}

	for _, iface := range ifaces {
		if !managedNetworks.Has(iface.Network) {
			continue
		}
		netName := c.mapURLToNetworkName(iface.Network, nnc)
		for _, cidr := range iface.AliasIPRanges {
			allActualCIDRs = append(allActualCIDRs, nncv1.PodCIDR{
				Id:      cidr,
				Network: netName,
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

	nnc.Status.PodCIDRs = allActualCIDRs
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready")

	klog.Infof("Syncing NNC status to GCE state for %q (CIDRs: %d)", nnc.Name, len(allActualCIDRs))
	return c.updateNNCStatus(ctx, nnc)
}

func (c *NodeNetworkConfigStatusController) updateNNCStatus(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	_, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(ctx, nnc, metav1.UpdateOptions{})
	return err
}

func (c *NodeNetworkConfigStatusController) setCondition(nnc *nncv1.NodeNetworkConfig, cType string, status corev1.ConditionStatus, reason, message string) bool {
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

func (c *NodeNetworkConfigStatusController) resolveNetworkURL(netName string) string {
	if c.gceCloud == nil {
		if netName == "" || netName == "default" {
			return "https://www.googleapis.com/compute/v1/projects/test-project/global/networks/default"
		}
		return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/test-project/global/networks/%s", netName)
	}
	if netName == "" || netName == "default" {
		return c.gceCloud.NetworkURL()
	}
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", c.gceCloud.ProjectID(), netName)
}

func (c *NodeNetworkConfigStatusController) mapURLToNetworkName(url string, nnc *nncv1.NodeNetworkConfig) string {
	for _, alloc := range nnc.Spec.Allocations {
		if c.resolveNetworkURL(alloc.Network) == url {
			return alloc.Network
		}
	}
	for _, pc := range nnc.Status.PodCIDRs {
		if c.resolveNetworkURL(pc.Network) == url {
			return pc.Network
		}
	}
	if url == c.resolveNetworkURL("default") || url == c.resolveNetworkURL("") {
		return "default"
	}
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
