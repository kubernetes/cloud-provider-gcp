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
	"net"
	"strings"
	"sync"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	"golang.org/x/time/rate"
	computebeta "google.golang.org/api/compute/v0.beta"
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
	"k8s.io/utils/clock"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const (
	// DefaultBlockSizeMask is the default CIDR mask we request from GCE (e.g. 28 for 16 IPs).
	DefaultBlockSizeMask = 28

	// reconcileTimeout is the maximum time allowed for a single node reconciliation.
	reconcileTimeout = 60 * time.Second
)

var (
	// DefaultBlockSize is the string representation of the default block size (derived from DefaultBlockSizeMask).
	DefaultBlockSize string
	// DefaultCapacity is the number of IPs in the default block size (derived from DefaultBlockSizeMask).
	DefaultCapacity int
)

func init() {
	DefaultCapacity = 1 << (32 - DefaultBlockSizeMask)
	DefaultBlockSize = fmt.Sprintf("/%d", DefaultBlockSizeMask)
}

// Controller manages NodeNetworkConfig resources and mutates GCE VM alias IPs.
type Controller struct {
	kubeClient kubernetes.Interface
	nncClient  nncclientset.Interface
	nncLister  nnclisters.NodeNetworkConfigLister
	nncSynced  cache.InformerSynced
	nodeLister corelisters.NodeLister
	nodeSynced cache.InformerSynced
	gceCloud   *gce.Cloud
	queue      workqueue.TypedRateLimitingInterface[string]
	gceCache   *GCECache
	clock      clock.Clock
}

// NewController creates a new DynamicPodIPController.
func NewController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncInformer nncinformers.NodeNetworkConfigInformer,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) *Controller {
	// Explicitly construct the rate limiter for the workqueue
	rateLimiter := workqueue.NewTypedMaxOfRateLimiter[string](
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)

	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		gceIfaces, err := gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return toNetworkInterfaces(gceIfaces), nil
	}

	c := &Controller{
		kubeClient: kubeClient,
		nncClient:  nncClient,
		nncLister:  nncInformer.Lister(),
		nncSynced:  nncInformer.Informer().HasSynced,
		nodeLister: nodeInformer.Lister(),
		nodeSynced: nodeInformer.Informer().HasSynced,
		gceCloud:   gceCloud,
		queue:      workqueue.NewTypedRateLimitingQueueWithConfig[string](rateLimiter, workqueue.TypedRateLimitingQueueConfig[string]{Name: "dynamic-pod-ip"}),
		gceCache:   NewGCECache(loader, 10*time.Second, clock.RealClock{}),
		clock:      clock.RealClock{},
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

// SetClock overrides the default real clock with a custom clock (used in unit tests).
func (c *Controller) SetClock(clk clock.Clock) {
	c.clock = clk
	c.gceCache.clock = clk
}

// Name returns the name of the controller.
func (c *Controller) Name() string {
	return "dynamic-pod-ip-controller"
}

// Run starts the controller workers.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
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

	err := c.syncNode(key)
	c.handleErr(err, key)
	return true
}

func (c *Controller) handleErr(err error, key string) {
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
	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
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

// networkChanges encapsulates the additions and removals for a specific network interface.
type networkChanges struct {
	additions []string
	removals  []string
}

// nodeNetworkChanges wraps a map of network names to their networkChanges for a node.
type nodeNetworkChanges map[string]networkChanges

// Empty returns true if there are no changes for any network.
func (n nodeNetworkChanges) Empty() bool {
	return len(n) == 0
}

// Networks returns a sorted slice of network names that have changes.
func (n nodeNetworkChanges) Networks() []string {
	return sets.StringKeySet(n).List()
}

// GetNetwork returns the networkChanges for a given network name.
func (n nodeNetworkChanges) GetNetwork(network string) networkChanges {
	return n[network]
}

func (c *Controller) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	nodeName := nnc.Name

	// 1. Retrieve the actual GCE state. The cache automatically handles hits, misses, and refreshes.
	ifaces, err := c.gceCache.Get(ctx, nodeName, providerID)
	if err != nil {
		klog.Errorf("Failed to get GCE state for node %q: %v", nodeName, err)
		_ = c.updateStatusError(ctx, nnc, string(nncv1.NodeNetworkConfigInvalidParametersReason), err.Error())
		return fmt.Errorf("failed to get GCE state for node %q: %w", nodeName, err)
	}

	// 2. Calculate changes using GCE actual state instead of K8s status
	changes, err := c.calculateChanges(nnc, ifaces)
	if err != nil {
		klog.Errorf("Failed to calculate changes for %q: %v", nnc.Name, err)
		return c.updateStatusError(ctx, nnc, string(nncv1.NodeNetworkConfigInvalidParametersReason), err.Error())
	}

	if changes.Empty() {
		klog.V(4).Infof("NodeNetworkConfig %q is already aligned", nnc.Name)
		if c.statusNeedsSync(nnc, ifaces) {
			return c.syncStatusToGCE(ctx, nnc, ifaces)
		}
		if c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionTrue, string(nncv1.NodeNetworkConfigReadyReason), "Node network config is ready") {
			return c.updateNNCStatus(ctx, nnc)
		}
		return nil
	}

	// 3. Set condition to False (Updating) before starting GCE mutation.
	c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, "Updating", "GCE mutation in progress")
	if err := c.updateNNCStatus(ctx, nnc); err != nil {
		return fmt.Errorf("failed to update status to 'Updating' for %q: %w", nnc.Name, err)
	}

	// 4. Apply changes via GCE Provider sequentially per network (Provider returns error only).
	for _, network := range changes.Networks() {
		change := changes.GetNetwork(network)
		netURL := c.resolveNetworkURL(network)

		klog.Infof("Applying GCE mutations for node %q, network %q (URL=%q): additions=%v, removals=%v", nnc.Name, network, netURL, change.additions, change.removals)

		err := c.gceCloud.UpdateInstanceAliasIPRanges(ctx, providerID, netURL, change.additions, change.removals)
		if err != nil {
			klog.Errorf("GCE mutation failed for node %q, network %q: %v", nnc.Name, network, err)
			c.setCondition(nnc, string(nncv1.NodeNetworkConfigConditionReady), corev1.ConditionFalse, string(nncv1.NodeNetworkConfigInvalidParametersReason), err.Error())
			_ = c.updateNNCStatus(ctx, nnc)
			return err // Re-queue (cache is NOT invalidated, will retry using cached old state)
		}
	}

	// 5. Active Refresh: ForceGet bypasses TTL, performs a fresh GCE GET, and updates the cache.
	finalIfaces, err := c.gceCache.ForceGet(ctx, nodeName, providerID)
	if err != nil {
		return fmt.Errorf("failed to refresh GCE state after mutation for node %q: %w", nodeName, err)
	}

	// 6. Sync K8s Status using the fresh, cached GCE state.
	return c.syncStatusToGCE(ctx, nnc, finalIfaces)
}

// resolveNetworkURL maps NNC network name to GCE Network URL.
func (c *Controller) resolveNetworkURL(networkName string) string {
	if networkName == "" || networkName == "default" {
		return c.gceCloud.NetworkURL()
	}
	// Construct guess URL for custom network in the same project
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", c.gceCloud.ProjectID(), networkName)
}

// mapURLToNetworkName maps a GCE Network URL back to the network name used in the NNC Spec/Status.
func (c *Controller) mapURLToNetworkName(url string, nnc *nncv1.NodeNetworkConfig) string {
	// Check default network
	if url == c.gceCloud.NetworkURL() {
		for _, alloc := range nnc.Spec.Allocations {
			if alloc.Network == "default" || alloc.Network == "" {
				return alloc.Network
			}
		}
		for _, pc := range nnc.Status.PodCIDRs {
			if pc.Network == "default" || pc.Network == "" {
				return pc.Network
			}
		}
		return "default" // fallback
	}

	// Check custom networks in spec/status
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

	// Fallback to the last component of the URL
	return lastComponent(url)
}

// lastComponent returns the last component of a slash-separated URL/path.
func lastComponent(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// networkInterface is a controller-internal, lightweight representation of a GCE network interface.
type networkInterface struct {
	Name          string
	Network       string
	AliasIPRanges []string
}

// toNetworkInterfaces converts a slice of GCE API computebeta.NetworkInterface objects to controller-internal networkInterface objects.
func toNetworkInterfaces(gceIfaces []*computebeta.NetworkInterface) []*networkInterface {
	if gceIfaces == nil {
		return nil
	}
	res := make([]*networkInterface, len(gceIfaces))
	for i, iface := range gceIfaces {
		if iface == nil {
			continue
		}
		ni := &networkInterface{
			Name:    iface.Name,
			Network: iface.Network,
		}
		for _, r := range iface.AliasIpRanges {
			if r != nil && r.IpCidrRange != "" {
				ni.AliasIPRanges = append(ni.AliasIPRanges, r.IpCidrRange)
			}
		}
		res[i] = ni
	}
	return res
}

// statusNeedsSync checks if the NNC Status PodCIDRs set differs from the actual GCE alias IP ranges.
func (c *Controller) statusNeedsSync(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) bool {
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
func (c *Controller) syncStatusToGCE(ctx context.Context, nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) error {
	var allActualCIDRs []nncv1.PodCIDR

	// Build a set of managed network URLs to filter out unmanaged GCE interfaces
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
			continue // Skip unmanaged GCE interfaces
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

// calculateChanges compares Spec and GCE actual interfaces to determine additions (sizes) and removals (exact CIDRs).
func (c *Controller) calculateChanges(nnc *nncv1.NodeNetworkConfig, ifaces []*networkInterface) (nodeNetworkChanges, error) {
	changes := make(nodeNetworkChanges)

	currentCapacity := make(map[string]int)
	activeCIDRs := sets.NewString()

	for _, iface := range ifaces {
		netName := c.mapURLToNetworkName(iface.Network, nnc)
		for _, cidr := range iface.AliasIPRanges {
			cap, err := cidrCapacity(cidr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse CIDR %q in GCE: %w", cidr, err)
			}
			currentCapacity[netName] += cap
			activeCIDRs.Insert(fmt.Sprintf("%s/%s", netName, cidr))
		}
	}

	// 1. Calculate Additions (Growth)
	for _, alloc := range nnc.Spec.Allocations {
		network := alloc.Network
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

	// 2. Calculate Removals (Shrink)
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		network := rel.Network
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

// cidrCapacity calculates the number of IP addresses in a CIDR block.
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

// --- GCE Cache Implementation (Loading Cache with Per-Node Locking) ---

// GCEInstanceLoader is a functional dependency injected into the cache to fetch fresh data from GCE.
type GCEInstanceLoader func(ctx context.Context, providerID string) ([]*networkInterface, error)

// CachedInstance represents a cached view of a single GCE instance's network interfaces, protected by its own mutex.
type CachedInstance struct {
	mu          sync.Mutex // Guards this specific node's cached interfaces and timestamp
	interfaces  []*networkInterface
	lastUpdated time.Time
}

// GCECache manages thread-safe, concurrent timed caching of GCE instance states using per-node locking.
type GCECache struct {
	mapLock   sync.RWMutex // Guards the map structure itself
	instances map[string]*CachedInstance
	loader    GCEInstanceLoader
	ttl       time.Duration
	clock     clock.Clock
}

// NewGCECache constructs a new GCE loading cache.
func NewGCECache(loader GCEInstanceLoader, ttl time.Duration, clock clock.Clock) *GCECache {
	return &GCECache{
		instances: make(map[string]*CachedInstance),
		loader:    loader,
		ttl:       ttl,
		clock:     clock,
	}
}

// getOrCreateInstance retrieves or initializes the CachedInstance pointer for a node under the global map lock.
func (c *GCECache) getOrCreateInstance(nodeName string) *CachedInstance {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	inst, ok := c.instances[nodeName]
	if !ok {
		inst = &CachedInstance{}
		c.instances[nodeName] = inst
	}
	return inst
}

// Get retrieves the cached network interfaces for the node.
// If the cache is stale or missing, it calls the GCE loader holding ONLY the node-specific lock.
func (c *GCECache) Get(ctx context.Context, nodeName string, providerID string) ([]*networkInterface, error) {
	return c.get(ctx, nodeName, providerID, false)
}

// ForceGet bypasses the TTL check, forces a fresh load from GCE, updates the cache, and returns the state.
func (c *GCECache) ForceGet(ctx context.Context, nodeName string, providerID string) ([]*networkInterface, error) {
	return c.get(ctx, nodeName, providerID, true)
}

func (c *GCECache) get(ctx context.Context, nodeName string, providerID string, force bool) ([]*networkInterface, error) {
	inst := c.getOrCreateInstance(nodeName)

	// Lock only this specific node's state. Other nodes can be processed concurrently.
	inst.mu.Lock()
	defer inst.mu.Unlock()

	now := c.clock.Now()
	if force || inst.lastUpdated.IsZero() || now.Sub(inst.lastUpdated) > c.ttl {
		ifaces, err := c.loader(ctx, providerID)
		if err != nil {
			return nil, err
		}
		inst.interfaces = deepCopyInterfaces(ifaces)
		inst.lastUpdated = now
	}

	return deepCopyInterfaces(inst.interfaces), nil
}

// deepCopyInterfaces performs a deep copy of internal network interfaces to prevent data races.
func deepCopyInterfaces(ifaces []*networkInterface) []*networkInterface {
	if ifaces == nil {
		return nil
	}
	copy := make([]*networkInterface, len(ifaces))
	for i, ni := range ifaces {
		if ni == nil {
			continue
		}
		niCopy := &networkInterface{
			Name:    ni.Name,
			Network: ni.Network,
		}
		if ni.AliasIPRanges != nil {
			niCopy.AliasIPRanges = append([]string(nil), ni.AliasIPRanges...)
		}
		copy[i] = niCopy
	}
	return copy
}
