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
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"fmt"
	"net/netip"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/metis/pkg/store"
)

const (
	lowUtilizationThreshold  = 0.50
	highUtilizationThreshold = 0.80
	targetUtilizationAfterScaleUp = 0.75
	cooldownThreshold        = 10
	DefaultCooldownPushbackInterval = 2 * time.Second
	updateMaxRetries         = 10
	DefaultDrainingExpiration = 5 * time.Hour
	sustainedLowUtilizationDuration = 8 * time.Hour

	// DefaultWorkers is the number of workers to run the controller.
	// We use multiple workers to achieve parallel processing across different networks.
	// Sequential execution within a single network is guaranteed by using the network name
	// as the workqueue key.
	DefaultWorkers = 4
)

// DaemonController is a simple Kubernetes controller that processes queue items.
type DaemonController struct {
	queue       workqueue.TypedRateLimitingInterface[string]
	// syncHandler is the function that processes a queue item.
	// It is exposed as a field primarily for testability, allowing tests to override it.
	syncHandler func(ctx context.Context, network string) error
	name        string
	nncClient   nncclientset.Interface
	nodeName    string
	nncLister   nnclisters.NodeNetworkConfigLister
	nncSynced   cache.InformerSynced
	store           *store.Store
	monitorInterval time.Duration
	logger          logr.Logger
	lowUtilizationTimers map[string]time.Time
	OnCIDRAdded func(network string, availableIPs int)
	GetPendingRequestsCount func(network string) int
	cooldownPushbackInterval time.Duration
	drainingExpiration       time.Duration
}

// DaemonControllerConfig contains the configuration parameters for creating a DaemonController.
type DaemonControllerConfig struct {
	Name                    string
	Logger                  logr.Logger
	NNCClient               nncclientset.Interface
	NodeName                string
	NNCInformer             nncinformers.NodeNetworkConfigInformer
	Store                   *store.Store
	MonitorInterval         time.Duration
	OnCIDRAdded             func(network string, availableIPs int)
	GetPendingRequestsCount func(network string) int
	CooldownPushbackInterval time.Duration
	DrainingExpiration       time.Duration
}

// NewDaemonController creates a new DaemonController.
func NewDaemonController(cfg DaemonControllerConfig) *DaemonController {
	rl := workqueue.DefaultTypedControllerRateLimiter[string]()
	var queue workqueue.TypedRateLimitingInterface[string]
	if cfg.Name == "" {
		queue = workqueue.NewTypedRateLimitingQueue[string](rl)
	} else {
		queue = workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[string]{
			Name: cfg.Name,
		})
	}

	var nncLister nnclisters.NodeNetworkConfigLister
	var nncSynced cache.InformerSynced
	if cfg.NNCInformer != nil {
		nncLister = cfg.NNCInformer.Lister()
		nncSynced = cfg.NNCInformer.Informer().HasSynced
	}

	monitorInterval := cfg.MonitorInterval
	if monitorInterval <= 0 {
		monitorInterval = DefaultMonitorInterval
	}

	cooldownPushbackInterval := cfg.CooldownPushbackInterval
	if cooldownPushbackInterval <= 0 {
		cooldownPushbackInterval = DefaultCooldownPushbackInterval
	}

	drainingExpiration := cfg.DrainingExpiration
	if drainingExpiration <= 0 {
		drainingExpiration = DefaultDrainingExpiration
	}

	c := &DaemonController{
		queue:                   queue,
		name:                    cfg.Name,
		nncClient:               cfg.NNCClient,
		nodeName:                cfg.NodeName,
		nncLister:               nncLister,
		nncSynced:               nncSynced,
		store:                   cfg.Store,
		monitorInterval:         monitorInterval,
		cooldownPushbackInterval: cooldownPushbackInterval,
		drainingExpiration:       drainingExpiration,
		logger:                  cfg.Logger,
		lowUtilizationTimers:    make(map[string]time.Time),
		OnCIDRAdded:             cfg.OnCIDRAdded,
		GetPendingRequestsCount: cfg.GetPendingRequestsCount,
	}

	c.syncHandler = c.syncNodeNetworkConfig

	if cfg.NNCInformer != nil {
		cfg.NNCInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				nnc, ok := obj.(*nncv1.NodeNetworkConfig)
				if !ok {
					return
				}
				if nnc.Name == cfg.NodeName {
					for _, alloc := range nnc.Spec.Allocations {
						c.queue.Add(alloc.Network)
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
					c.logger.V(4).Info("Skipping enqueue, ResourceVersion hasn't changed", "name", newNNC.Name, "resourceVersion", newNNC.ResourceVersion)
					return
				}
				if newNNC.Name == cfg.NodeName {
					for _, alloc := range newNNC.Spec.Allocations {
						c.queue.Add(alloc.Network)
					}
				}
			},
		})
	}

	return c
}

// Run starts the controller workers and blocks until the context is cancelled.
func (c *DaemonController) Run(ctx context.Context, workers int) {
	defer c.queue.ShutDown()

	c.logger.Info("Starting daemon controller", "name", c.name, "workers", workers)
	defer c.logger.Info("Stopping daemon controller", "name", c.name)

	if c.nncSynced != nil {
		if !cache.WaitForNamedCacheSync(c.name, ctx.Done(), c.nncSynced) {
			return
		}
	}

	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		networks, err := c.store.GetAllNetworks(ctx)
		if err != nil {
			c.logger.Error(err, "failed to get networks for periodic sync")
			return
		}
		for _, network := range networks {
			c.queue.Add(network)
		}
	}, c.monitorInterval)

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *DaemonController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *DaemonController) processNextWorkItem(ctx context.Context) bool {
	item, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(item)

	err := c.syncHandler(ctx, item)
	if err == nil {
		c.queue.Forget(item)
		c.logger.V(4).Info("Successfully synced item", "item", item, "controller", c.name)
		return true
	}

	if c.queue.NumRequeues(item) < updateMaxRetries {
		c.logger.Error(err, "Failed to sync item, retrying", "item", item, "controller", c.name, "requeues", c.queue.NumRequeues(item))
		c.queue.AddRateLimited(item)
		return true
	}

	c.logger.Error(nil, "Failed to sync item, dropping from queue after max retries", "item", item, "controller", c.name, "maxRetries", updateMaxRetries)
	c.queue.Forget(item)
	return true
}

// Enqueue adds a network to the queue.
func (c *DaemonController) Enqueue(network string) {
	c.queue.Add(network)
}

// TODO: Consider the following optimization:
// 1. Only run syncCIDR or dynamicAllocation based on the trigger. For example,
//    if triggered by an IP exhaustion event, we might only need dynamicAllocation.
//    If triggered by an informer update, we might only need syncCIDR.
// Note: Skipping processing a queue item if another item for the same network is already
// waiting to be executed is not needed because the Kubernetes RateLimitingInterface
// workqueue already deduplicates items that are waiting in the queue.
func (c *DaemonController) syncNodeNetworkConfig(ctx context.Context, network string) error {
	c.logger.Info("Syncing network", "network", network)

	if err := c.syncCIDR(ctx, network); err != nil {
		return err
	}
	return c.dynamicAllocation(ctx, network)
}

// getNodeNetworkConfig fetches the NodeNetworkConfig CR.
// It prefers using the lister (cache) for efficiency, but falls back to the API client
// if the lister is not available. This fallback and the nil checks are primarily to
// support unit tests that do not initialize the full informer stack.
func (c *DaemonController) getNodeNetworkConfig(ctx context.Context) (*nncv1.NodeNetworkConfig, error) {
	if c.nncLister != nil {
		nnc, err := c.nncLister.Get(c.nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from lister: %w", err)
		}
		return nnc, nil
	}
	if c.nncClient != nil {
		nnc, err := c.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, c.nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from API: %w", err)
		}
		return nnc, nil
	}
	return nil, fmt.Errorf("no client or lister available to fetch NodeNetworkConfig")
}

func (c *DaemonController) syncCIDR(ctx context.Context, network string) error {
	c.logger.Info("Syncing NodeNetworkConfig status", "node", c.nodeName, "network", network)

	nnc, err := c.getNodeNetworkConfig(ctx)
	if err != nil {
		return err
	}

	if err := c.addCIDR(ctx, nnc, network); err != nil {
		return err
	}

	return c.maybeDeleteCIDRs(ctx, nnc)
}

func (c *DaemonController) addCIDR(ctx context.Context, nnc *nncv1.NodeNetworkConfig, network string) error {
	for _, podCIDR := range nnc.Status.PodCIDRs {
		if podCIDR.Network != network {
			continue
		}
		if podCIDR.Condition != nil && podCIDR.Condition.Status != metav1.ConditionTrue {
			c.logger.V(4).Info("PodCIDR not ready, skipping", "cidr", podCIDR.CIDR, "network", podCIDR.Network)
			continue
		}

		prefix, err := netip.ParsePrefix(podCIDR.CIDR)
		if err != nil {
			c.logger.Error(err, "failed to parse CIDR", "cidr", podCIDR.CIDR)
			continue
		}

		// Ignore IPv6 as it is not supported for dynamic allocation path.
		if prefix.Addr().Is6() {
			c.logger.V(4).Info("Ignoring IPv6 CIDR, not supported in dynamic allocation path", "cidr", podCIDR.CIDR)
			continue
		}

		c.logger.Info("Adding podCIDR to local DB", "cidr", podCIDR.CIDR, "network", podCIDR.Network)
		// Parse CIDR first for capacity calculation
		bits := prefix.Bits()
		availableIPs := 1 << (32 - bits)

		// Idempotency check: check if CIDR already exists
		_, exists, err := c.store.GetCIDRBlockByCIDR(ctx, podCIDR.CIDR)
		if err != nil {
			return fmt.Errorf("failed to check if CIDR exists in store: %w", err)
		}
		if exists {
			c.logger.V(4).Info("CIDR already exists in local DB", "cidr", podCIDR.CIDR)
			continue
		}

		err = c.store.AddCIDR(ctx, podCIDR.Network, podCIDR.CIDR)
		if err == nil {
			if c.OnCIDRAdded != nil {
				c.OnCIDRAdded(podCIDR.Network, availableIPs)
			}
		} else {
			if errors.Is(err, store.ErrCidrAlreadyExists) {
				c.logger.V(4).Info("CIDR already exists in local DB (race condition)", "cidr", podCIDR.CIDR)
			} else {
				return fmt.Errorf("failed to add CIDR to store: %w", err)
			}
		}
	}
	return nil
}

func (c *DaemonController) maybeDeleteCIDRs(ctx context.Context, nnc *nncv1.NodeNetworkConfig) error {
	// Reading deleting CIDRblock and then delete is not thread safe, the queue processing guarantees it is fine.
	toBeDeletedBlocks, err := c.store.GetDeletingCIDRBlocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to query deleting cidr blocks: %w", err)
	}

	var toBeDeletedBlockIDs []int64
	for _, block := range toBeDeletedBlocks {
		inStatus := false
		for _, podCIDR := range nnc.Status.PodCIDRs {
			if podCIDR.CIDR == block.CIDR {
				inStatus = true
				break
			}
		}

		if !inStatus {
			toBeDeletedBlockIDs = append(toBeDeletedBlockIDs, block.ID)
		}
	}

	for _, id := range toBeDeletedBlockIDs {
		err = c.store.DeleteCIDRBlock(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to delete cidr block %d from store: %w", id, err)
		}
		c.logger.Info("Deleted CIDR block from local DB as it was released by GCE", "cidrBlockID", id)
	}

	return nil
}

// The dynamicAllocation method evaluates the current IP usage and triggers dynamic allocation of more podCIDRs when the utilization exceeds the 80% threshold.
// It scales up by updating spec.allocation.pods in the NodeNetworkConfig CR, adding N more IPs so that utilization drops under 75%.
//
// Let:
//   U = Utilization
//   A = Local Used IPs
//   T = Local Total IPs
//   P = Pending Requests (waiting for local IP)
//   I = Inflight Outgoing Requests (waiting to be fulfilled)
//
// Formula: U = (A + P) / (T + I)
// For example:
//
// | Metric                                      | Value |
// |---------------------------------------------|-------|
// | Local DB IPs (Used/Total)                   | 43/48  |
// | Pending Requests                            | 10     |
// | Inflight Outgoing Requests                  | 16    |
// | Resulting Current Utilization               | 53/64 |
//
// With utilization (53/64 = ~83%) > 80%, it triggers fetching N more IPs so that the new utilization drops under 75%.
// The next dynamic allocation requests will see utilization not hitting 80% so it's a no-op.
//
// Examples for different node sizes (assuming 0 pending requests and 16 initial IPs):
//
// | Node Size    | Trigger Used IPs | New Total IPs | Pods Fetched | New Utilization |
// |--------------|------------------|---------------|--------------|-----------------|
// | Small (/28)  | 13 (~81.3%)      | 18            | 2            | ~72.2%          |
// | Medium (/26) | 52 (~81.3%)      | 70            | 6            | ~74.2%          |
// | Large (/24)  | 205 (~80.1%)     | 274           | 18           | ~74.8%          |
//
// Alternative formula to calculate utilization: 
// U = 1 - localAvailable / (requestedCapacity + initialCidrRange)
// where requestedCapacity is from CRD status, and initialCidrRange is initialIPs.
func (c *DaemonController) dynamicAllocation(ctx context.Context, network string) error {
	c.logger.Info("Evaluating IP usage for dynamic allocation", "node", c.nodeName)

	info, err := c.getUtilizationInfo(ctx, network)
	if err != nil {
		return err
	}

	// The first pod is not started yet. Wait until initial CIDR is added then do dynamic allocation.
	if info.InitialIPs == 0 {
		c.logger.Info("Initial IPs is 0, skipping dynamic allocation", "network", network)
		return nil
	}

	updated := false
	if info.Utilization > highUtilizationThreshold {
		updatedBranch, err := c.handleHighUtilization(network, info)
		if err != nil {
			return err
		}
		if updatedBranch {
			updated = true
		}
	} 

	c.maybeDrainExcessive(ctx, network, info)

	expiredUpdated, err := c.handleExpiredDrainingBlocks(ctx, network, info.NncCopy, info.CurrentAllocation)
	if err != nil {
		return err
	}
	if expiredUpdated {
		updated = true
	}

	
	if updated {
		patchData := map[string]interface{}{
			"spec": map[string]interface{}{
				"allocations":     info.NncCopy.Spec.Allocations,
				"releasableCIDRs": info.NncCopy.Spec.ReleasableCIDRs,
			},
		}
		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			return fmt.Errorf("failed to marshal patch: %w", err)
		}

		_, err = c.nncClient.NetworkingV1().NodeNetworkConfigs().Patch(ctx, c.nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch NodeNetworkConfig: %w", err)
		}
		c.logger.Info("Successfully patched NodeNetworkConfig", "allocations", info.NncCopy.Spec.Allocations, "releasableCIDRs", info.NncCopy.Spec.ReleasableCIDRs)
	}
	return nil
}

func (c *DaemonController) calculateUtilization(usedIPs, pendingRequests, totalRequestedCapacity int) float64 {
	if totalRequestedCapacity > 0 {
		return float64(usedIPs+pendingRequests) / float64(totalRequestedCapacity)
	}
	if usedIPs+pendingRequests > 0 {
		return 1.0
	}
	return 0.0
}

// UtilizationInfo holds details about IP utilization for a network.
type UtilizationInfo struct {
	// Utilization is the calculated IP utilization ratio for the network.
	Utilization            float64
	// Usage contains detailed IP usage counts from the local store.
	Usage                  store.NetworkIPUsage
	// PendingRequests is the number of IP allocation requests waiting to be fulfilled.
	PendingRequests        int
	// InitialIPs is the number of IPs provided in the initial node CIDR.
	InitialIPs            int
	// TargetPods is the current target number of pods requested for this network.
	TargetPods             int
	// CurrentAllocation points to the specific allocation for this network within NncCopy.
	// We keep both CurrentAllocation (a pointer) and NncCopy because CurrentAllocation
	// allows us to directly modify the specific network's allocation pods count, while
	// NncCopy provides access to the entire object for modifications to other fields
	// (like ReleasableCIDRs) and for the final patch operation.
	CurrentAllocation      *nncv1.Allocation
	// NncCopy is a deep copy of the NodeNetworkConfig CRD used for modifications.
	NncCopy                *nncv1.NodeNetworkConfig
	// TotalRequestedCapacity is the sum of InitialIPs and TargetPods.
	TotalRequestedCapacity int
}

func (c *DaemonController) getUtilizationInfo(ctx context.Context, network string) (*UtilizationInfo, error) {
	// GetIPUsageByNetwork returns IP usage details including allocated and cooldown counts, 
	// while ignoring CIDR blocks that are marked as Deleting.
	// Note that this includes CIDR blocks in Draining status in both used and total counts.
	// This ensures that processing prefetch (dynamic allocation) is not interfered with
	// (triggered unnecessarily) while we are trying to remove excessive capacity by draining blocks.
	usage, err := c.store.GetIPUsageByNetwork(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP usage: %w", err)
	}

	var nnc *nncv1.NodeNetworkConfig
	nnc, err = c.getNodeNetworkConfig(ctx)
	if err != nil {
		return nil, err
	}

	nncCopy := nnc.DeepCopy()

	usedIPs := usage.Allocated + usage.Cooldown

	initialIPs, err := c.store.GetInitialIPCount(ctx, network)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			initialIPs = 0
		} else {
			return nil, fmt.Errorf("failed to query initial IPs for network %s: %w", network, err)
		}
	}

	pendingRequests := 0
	if c.GetPendingRequestsCount != nil {
		pendingRequests = c.GetPendingRequestsCount(network)
	}

	targetPods := 0
	var currentAllocation *nncv1.Allocation
	for i := range nncCopy.Spec.Allocations {
		if nncCopy.Spec.Allocations[i].Network == network {
			currentAllocation = &nncCopy.Spec.Allocations[i]
			targetPods = int(currentAllocation.Pods)
			break
		}
	}

	totalRequestedCapacity := initialIPs + targetPods
		
	utilization := c.calculateUtilization(usedIPs, pendingRequests, totalRequestedCapacity)

	c.logger.Info("Calculated utilization", "network", network, "used", usedIPs, "pending", pendingRequests, "total", usage.Total, "totalRequestedCapacity", totalRequestedCapacity, "utilization", utilization)

	return &UtilizationInfo{
		Utilization:            utilization,
		Usage:                  usage,
		PendingRequests:        pendingRequests,
		InitialIPs:            initialIPs,
		TargetPods:             targetPods,
		CurrentAllocation:      currentAllocation,
		NncCopy:                nncCopy,
		TotalRequestedCapacity: totalRequestedCapacity,
	}, nil

}

func (c *DaemonController) handleHighUtilization(network string, info *UtilizationInfo) (bool, error) {
	usedIPs := info.Usage.Allocated + info.Usage.Cooldown
	// If there are a lot of IPs in cooldown, hold off sending outgoing requests.
	if info.Usage.Cooldown > cooldownThreshold {
		c.logger.Info("Too many IPs in cooldown, holding on sending outgoing requests", "network", network, "cooldownCount", info.Usage.Cooldown)
		c.queue.AddAfter(network, c.cooldownPushbackInterval)
		return false, nil
	}

	c.logger.Info(fmt.Sprintf("Utilization exceeds %d%%, triggering dynamic allocation", int(highUtilizationThreshold*100)), "network", network)

	// Calculate new target capacity to bring utilization below targetUtilizationAfterScaleUp
	// TODO: Cap for the max pod on node constraint
	newTotalCapacity := int(float64(usedIPs+info.PendingRequests)/targetUtilizationAfterScaleUp) + 1
	
	newTargetPods := newTotalCapacity - info.InitialIPs
	if newTargetPods < 0 {
		newTargetPods = 0
	}

	if newTargetPods <= info.TargetPods {
		c.logger.Info("Calculated new target pods is less than or equal to current target pods, skipping scale up", "network", network, "currentPods", info.TargetPods, "newPods", newTargetPods)
		return false, nil
	}

	c.logger.Info("Scaling up", "network", network, "currentPods", info.TargetPods, "newPods", newTargetPods)

	if info.CurrentAllocation != nil {
		info.CurrentAllocation.Pods = int32(newTargetPods)
	} else {
		info.NncCopy.Spec.Allocations = append(info.NncCopy.Spec.Allocations, nncv1.Allocation{
			Network: network,
			Pods:    int32(newTargetPods),
		})
	}

	return true, nil
}

// handleLowUtilization evaluates CIDR blocks to drain when utilization is low.
// It marks blocks as Draining to prevent new allocations in them, aiming to reduce excess capacity.
//
// Impact of IP Burst during Draining:
// If a sudden burst of IP requests arrives while blocks are draining:
// 1. The server will first use available IPs in non-draining (Ready) blocks.
// 2. If those are exhausted, it will automatically "undrain" blocks (mark them back as Ready) to satisfy requests.
// 3. If still needed, dynamic allocation will be triggered to request more capacity.
// Thus, draining is safe and won't cause starvation during bursts.
func (c *DaemonController) handleLowUtilization(ctx context.Context, network string, info *UtilizationInfo) (bool, error) {
	usedIPs := info.Usage.Allocated + info.Usage.Cooldown
	c.logger.Info(fmt.Sprintf("Utilization falls below %d%% for 8h, evaluating CIDR blocks to drain", int(lowUtilizationThreshold*100)), "network", network)

	readyBlocks, err := c.store.GetReadyCIDRBlocksSorted(ctx, network)
	if err != nil {
		return false, fmt.Errorf("failed to get ready blocks: %w", err)
	}

	if len(readyBlocks) <= 1 {
		c.logger.Info("Only initial block or no blocks available, skipping draining", "network", network)
		return false, nil
	}

	// Exclude initial block (earliest created_at). Assuming sorted DESC, initial is last.
	blocksToMark := readyBlocks[:len(readyBlocks)-1]

	totalRequestedCapacity := info.InitialIPs + info.TargetPods
	simulatedUsedIPs := usedIPs + info.PendingRequests
	
	// Target simulated used IPs to stay above lowUtilizationThreshold 
	targetUsedIPs:= int(lowUtilizationThreshold*float64(totalRequestedCapacity))
	
	updated := false
	for _, block := range blocksToMark {
		if simulatedUsedIPs >= targetUsedIPs {
			c.logger.Info("Target simulated used IPs reached, stopping marking blocks", "target", targetUsedIPs, "running", simulatedUsedIPs)
			break
		}
		availableIPs := block.TotalIPs - block.AllocatedIPs
		if availableIPs < 0 {
			availableIPs = 0
		}

		err = c.store.DrainCIDRBlock(ctx, block.ID)
		if err != nil {
			return false, fmt.Errorf("failed to drain block %d: %w", block.ID, err)
		}
		c.logger.Info("Marked CIDR block as Draining due to prolonged low utilization", "cidr", block.CIDR)

		simulatedUsedIPs += availableIPs
		updated = true
	}
	return updated, nil
}

// maybeDrainExcessive evaluates if the network utilization is low enough to start or continue
// the low utilization timer. If the low utilization is sustained for sustainedLowUtilizationDuration,
// it triggers handleLowUtilization to drain excessive CIDR blocks.
// If utilization is above the threshold, it resets any active timer for the network.
// Returns true if blocks were successfully drained.
func (c *DaemonController) maybeDrainExcessive(ctx context.Context, network string, info *UtilizationInfo) bool {
	if info.Utilization >= lowUtilizationThreshold {
		if _, ok := c.lowUtilizationTimers[network]; ok {
			delete(c.lowUtilizationTimers, network)
			c.logger.Info("Utilization went above threshold, reset timer", "network", network, "utilization", info.Utilization)
		}
	} else {
		if _, ok := c.lowUtilizationTimers[network]; !ok {
			c.lowUtilizationTimers[network] = time.Now()
			c.logger.Info("Low utilization detected, started timer", "network", network, "utilization", info.Utilization)
		}

		if time.Since(c.lowUtilizationTimers[network]) > sustainedLowUtilizationDuration {
			drained, err := c.handleLowUtilization(ctx, network, info)
			if err != nil {
				c.logger.Error(err, "Failed to handle low utilization", "network", network)
				return false
			}
			if drained {
				delete(c.lowUtilizationTimers, network)
				c.logger.Info("Successfully drained CIDR blocks, resetting low utilization timer", "network", network)
			}
			return drained
		}
	}
	return false
}

// handleExpiredDrainingBlocks queries for expired draining blocks, marks them as Deleting,
// and updates the CRD spec to release them.
func (c *DaemonController) handleExpiredDrainingBlocks(ctx context.Context, network string, nncCopy *nncv1.NodeNetworkConfig, currentAllocation *nncv1.Allocation) (bool, error) {
	expiredBlocks, err := c.store.FindAndMarkExpiredDrainingCIDRBlocks(ctx, network, c.drainingExpiration)
	if err != nil {
		return false, fmt.Errorf("failed to query and mark draining cidr blocks: %w", err)
	}

	var reducePods int
	updated := false

	deletingBlocks, err := c.store.GetDeletingCIDRBlocks(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to query deleting cidr blocks: %w", err)
	}

	// Create a map for quick lookup of ID by CIDR from Status
	statusMap := make(map[string]nncv1.PodCIDR)
	for _, podCIDR := range nncCopy.Status.PodCIDRs {
		if podCIDR.Network == network {
			statusMap[podCIDR.CIDR] = podCIDR
		}
	}

	// Create a set (map) for quick lookup of existing ReleasableCIDRs
	releasableSet := make(map[string]bool)
	for _, releasable := range nncCopy.Spec.ReleasableCIDRs {
		releasableSet[releasable.CIDR] = true
	}

	for _, block := range expiredBlocks {
		reducePods += block.TotalIPs

		if !releasableSet[block.CIDR] {
			podCIDR, inStatus := statusMap[block.CIDR]
			if !inStatus {
				c.logger.Error(nil, "failed to find CIDR in CR status for draining block", "cidr", block.CIDR)
				continue
			}

			nncCopy.Spec.ReleasableCIDRs = append(nncCopy.Spec.ReleasableCIDRs, podCIDR)
			releasableSet[block.CIDR] = true
			updated = true
		}
	}

	// Reconcile blocks that are in Deleting state in the local DB but failed to be added
	// to the CRD's Spec.ReleasableCIDRs in a previous iteration.
	for _, block := range deletingBlocks {
		podCIDR, inStatus := statusMap[block.CIDR]
		if inStatus && !releasableSet[block.CIDR] {
			c.logger.Info("Re-adding deleting block to ReleasableCIDRs for reconciliation", "cidr", block.CIDR)
			nncCopy.Spec.ReleasableCIDRs = append(nncCopy.Spec.ReleasableCIDRs, podCIDR)
			releasableSet[block.CIDR] = true // Update map to avoid duplicates if deletingBlocks has duplicates
			reducePods += block.TotalIPs
			updated = true
		}
	}

	if updated {
		c.logger.Info("Marked CIDR blocks as Deleting and added to ReleasableCIDRs", "network", network)

		if currentAllocation != nil {
			newTargetPods := int(currentAllocation.Pods) - reducePods
			if newTargetPods < 0 {
				newTargetPods = 0
			}
			currentAllocation.Pods = int32(newTargetPods)
		}
	}

	return updated, nil
}
