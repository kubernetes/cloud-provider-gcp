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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	nnclisters "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/listers/nodenetworkconfig/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/metis/pkg/store"
)

const (
	DefaultMonitorInterval         = 2 * time.Second
	DefaultReleaseCooldown         = 1 * time.Minute
	DefaultLowUtilizationThreshold = 0.50

	DefaultTargetUtilizationAfterScaleUp   = 0.75
	DefaultCooldownPushbackThreshold       = 10
	DefaultCooldownPushbackInterval        = 2 * time.Second
	DefaultDrainingExpiration              = 5 * time.Hour
	DefaultSustainedLowUtilizationDuration = 8 * time.Hour

	updateMaxRetries = 10

	// defaultMonitorWorkers is set to 1 because the monitor does read-modify-write updates on
	// the NodeNetworkConfig (NNC) custom resource. Running multiple workers concurrently could
	// cause race conditions where one worker overwrites updates from another.
	defaultMonitorWorkers = 1
)

// Monitor manages the dynamic scaling (up and down) of IP CIDR block capacity
// for each network on a node.
//
// The Monitor runs a periodic control loop to evaluate capacity, while also
// supporting on-demand triggers from the gRPC server when local IP exhaustion occurs.
//
// Key Behaviors:
//
// 1. Dynamic Scale-Up (Prefetching):
//   - Calculates utilization as: (AllocatedIPs + PendingRequests) / TotalCapacity.
//   - If utilization exceeds the target, it increases requested pod capacity in the
//     NodeNetworkConfig (NNC) CRD (modifying Spec.Allocations[].Pods) aiming for a
//     target utilization (default 75%).
//   - Pushes back and defers new requests if the number of IPs currently in a release
//     cooldown state exceeds a pushback threshold (default 10), preventing premature
//     capacity requests.
//
// 2. Scale-Down (Draining & Releasing):
//   - Draining: If utilization remains below a low threshold (default 50%) for a
//     sustained period (default 8 hours), it marks non-initial ready CIDR blocks as
//     'Draining' one by one until simulated utilization of remaining ready blocks
//     recovers above the threshold.
//   - Draining blocks are excluded from new allocations, but can be transitioned
//     back to 'Ready' by the gRPC server to quickly reclaim capacity during sudden
//     allocation bursts.
//   - Releasing: Periodically scans 'Draining' blocks. Once they have spent at least
//     the expiration duration (default 5 hours) in the draining state AND have zero
//     active allocations, the Monitor marks them as 'Deleting', appends them to
//     Spec.ReleasableCIDRs in the NNC CRD, and reduces the target NNC Pods allocation.
//     These 'Deleting' blocks are fully deleted from the local store once the CCM
//     Dynamic IPAM controller removes them from the NNC Status.
//
// The Monitor uses a rate-limiting workqueue to coordinate and serialize network
// sync requests, benefiting from deduplication and automatic backoff retries.
type Monitor struct {
	queue                   workqueue.TypedRateLimitingInterface[string]
	nncClient               nncclientset.Interface
	nncLister               nnclisters.NodeNetworkConfigLister
	nodeName                string
	store                   *store.Store
	logger                  logr.Logger
	lowUtilizationTimers    map[string]time.Time
	GetPendingRequestsCount func(network string) int

	// drainingExpiration is the duration after which a draining CIDR block is considered expired and candidate for release.
	drainingExpiration time.Duration

	// monitorInterval is how often the monitor evaluates network utilization (pre-fetch) and checks for expired draining blocks.
	monitorInterval time.Duration

	// lowUtilizationThreshold is the threshold (e.g., 0.50) below which a network is considered under-utilized.
	lowUtilizationThreshold float64

	// targetUtilizationAfterScaleUp is the desired utilization level (e.g., 0.75) aimed for after scaling up.
	targetUtilizationAfterScaleUp float64

	// cooldownPushbackThreshold is the maximum number of IPs allowed to be in cooldown
	// before we hold off on sending new outgoing requests for more CIDR blocks.
	// This prevents requesting more capacity when we have many IPs about to become available.
	cooldownPushbackThreshold int

	// cooldownPushbackInterval is how long to wait before re-evaluating a network when held back by cooldown threshold.
	cooldownPushbackInterval time.Duration

	// sustainedLowUtilizationDuration is the duration utilization must remain low before triggering a drain.
	sustainedLowUtilizationDuration time.Duration
}

// MonitorConfig holds the configuration for the Monitor.
type MonitorConfig struct {
	Logger                          logr.Logger
	NNCClient                       nncclientset.Interface
	NNCInformer                     nncinformers.NodeNetworkConfigInformer
	Store                           *store.Store
	NodeName                        string
	GetPendingRequestsCount         func(network string) int
	CooldownPushbackInterval        time.Duration
	DrainingExpiration              time.Duration
	MonitorInterval                 time.Duration
	SustainedLowUtilizationDuration time.Duration
}

// SetDefaults applies default values to the MonitorConfig fields if they are unset (<= 0).
func (c *MonitorConfig) SetDefaults() {
	if c.CooldownPushbackInterval <= 0 {
		c.CooldownPushbackInterval = DefaultCooldownPushbackInterval
	}
	if c.DrainingExpiration <= 0 {
		c.DrainingExpiration = DefaultDrainingExpiration
	}
	if c.SustainedLowUtilizationDuration <= 0 {
		c.SustainedLowUtilizationDuration = DefaultSustainedLowUtilizationDuration
	}
	if c.MonitorInterval <= 0 {
		c.MonitorInterval = DefaultMonitorInterval
	}
}

// NewMonitor creates a new Monitor.
func NewMonitor(cfg MonitorConfig) *Monitor {
	cfg.SetDefaults()

	rl := workqueue.DefaultTypedControllerRateLimiter[string]()
	// We use a rate-limiting queue to:
	// 1. Deduplicate requests from the daemon server and the periodic monitor loop.
	// 2. Decouple the daemon server from processing inline, allowing it to just enqueue items.
	// 3. Benefit from automatic exponential backoff for retries on failure.
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[string]{
		Name: "metis-daemon-monitor",
	})

	var nncLister nnclisters.NodeNetworkConfigLister
	if cfg.NNCInformer != nil {
		nncLister = cfg.NNCInformer.Lister()
	}

	return &Monitor{
		queue:                           queue,
		nncClient:                       cfg.NNCClient,
		nncLister:                       nncLister,
		nodeName:                        cfg.NodeName,
		store:                           cfg.Store,
		logger:                          cfg.Logger,
		lowUtilizationTimers:            make(map[string]time.Time),
		GetPendingRequestsCount:         cfg.GetPendingRequestsCount,
		cooldownPushbackInterval:        cfg.CooldownPushbackInterval,
		drainingExpiration:              cfg.DrainingExpiration,
		monitorInterval:                 cfg.MonitorInterval,
		lowUtilizationThreshold:         DefaultLowUtilizationThreshold,
		targetUtilizationAfterScaleUp:   DefaultTargetUtilizationAfterScaleUp,
		cooldownPushbackThreshold:       DefaultCooldownPushbackThreshold,
		sustainedLowUtilizationDuration: cfg.SustainedLowUtilizationDuration,
	}
}

// Run starts the workers and periodic enqueuer.
func (m *Monitor) Run(ctx context.Context, workers int) {
	defer m.queue.ShutDown()

	m.logger.Info("Starting Metis Daemon Monitor", "node", m.nodeName, "workers", workers, "interval", m.monitorInterval)
	defer m.logger.Info("Stopping Metis Daemon Monitor")

	// Periodic enqueuer
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		m.Enqueue()
	}, m.monitorInterval)

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, m.runWorker, m.monitorInterval)
	}

	<-ctx.Done()
}

// applyDeletingBlocks is removed because its logic is integrated into syncAll

func (m *Monitor) runWorker(ctx context.Context) {
	for m.processNextWorkItem(ctx) {
	}
}

func (m *Monitor) processNextWorkItem(ctx context.Context) bool {
	item, quit := m.queue.Get()
	if quit {
		return false
	}
	defer m.queue.Done(item)

	err := m.syncAll(ctx)
	if err == nil {
		m.queue.Forget(item)
		return true
	}

	if m.queue.NumRequeues(item) < updateMaxRetries {
		m.logger.Error(err, "Failed to process item, retrying", "item", item, "requeues", m.queue.NumRequeues(item))
		m.queue.AddRateLimited(item)
		return true
	}

	m.logger.Error(nil, "Failed to process item, dropping from queue after max retries", "item", item)
	m.queue.Forget(item)
	return true
}

// Enqueue adds a network to the queue.
// Enqueue adds a sync request to the queue.
func (m *Monitor) Enqueue() {
	m.queue.Add("sync")
}

func (m *Monitor) syncAll(ctx context.Context) error {
	m.logger.Info("Evaluating IP usage for dynamic allocation on node", "node", m.nodeName)

	nnc, err := getNodeNetworkConfig(ctx, m.nncLister, m.nncClient, m.nodeName)
	if err != nil {
		m.logger.Error(err, "failed to get NodeNetworkConfig")
		return err
	}
	nncCopy := nnc.DeepCopy()

	networks, err := m.store.GetAllNetworks(ctx)
	if err != nil {
		m.logger.Error(err, "failed to get networks from store")
		return err
	}

	updated := false
	var allNewReleasables []nncv1.PodCIDR

	for _, network := range networks {
		info, err := m.getUtilizationInfo(ctx, network, nncCopy)
		if err != nil {
			return err
		}

		if info.Usage.Total == 0 {
			m.logger.Info("Total IPs is 0, skipping dynamic allocation", "network", network)
			continue
		}

		// 1. Calculate desired scale-up target
		desiredPods := m.maybeScaleUp(network, info)

		// 2. Trigger draining if needed
		m.maybeDrainExcessive(ctx, network, info)

		// 3. Reconcile deleting blocks
		newReleasables, reducePods, err := m.reconcileDeletingBlocks(ctx, network, info.CurrentReleasables, info.CurrentStatus)
		if err != nil {
			return err
		}
		allNewReleasables = append(allNewReleasables, newReleasables...)

		// 4. Compute final target pods size
		finalTargetPods := max(0, desiredPods-reducePods)

		// 5. Apply target allocation changes
		if m.updateAllocationPods(nncCopy, network, info.CurrentAllocation, finalTargetPods) {
			updated = true
		}
	}

	// 6. Apply releasable updates
	if !reflect.DeepEqual(nncCopy.Spec.ReleasableCIDRs, allNewReleasables) {
		nncCopy.Spec.ReleasableCIDRs = allNewReleasables
		updated = true
	}

	if updated {
		return m.patchNNC(ctx, nncCopy)
	}
	return nil
}

func (m *Monitor) patchNNC(ctx context.Context, nncCopy *nncv1.NodeNetworkConfig) error {
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"allocations":     nncCopy.Spec.Allocations,
			"releasableCIDRs": nncCopy.Spec.ReleasableCIDRs,
		},
	}
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = m.nncClient.NetworkingV1().NodeNetworkConfigs().Patch(ctx, m.nodeName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch NodeNetworkConfig: %w", err)
	}
	m.logger.Info("Successfully patched NodeNetworkConfig", "allocations", nncCopy.Spec.Allocations, "releasableCIDRs", nncCopy.Spec.ReleasableCIDRs)
	return nil
}

func (m *Monitor) updateAllocationPods(nncCopy *nncv1.NodeNetworkConfig, network string, currentAllocation *nncv1.Allocation, targetPods int) bool {
	if currentAllocation != nil {
		if currentAllocation.Pods != int32(targetPods) {
			m.logger.Info("Updated target allocation Pods", "network", network, "oldTarget", currentAllocation.Pods, "newTarget", targetPods)
			currentAllocation.Pods = int32(targetPods)
			return true
		}
	} else {
		nncCopy.Spec.Allocations = append(nncCopy.Spec.Allocations, nncv1.Allocation{
			Network: network,
			Pods:    int32(targetPods),
		})
		m.logger.Info("Added target allocation Pods", "network", network, "target", targetPods)
		return true
	}
	return false
}

func (m *Monitor) calculateUtilization(usedIPs, pendingRequests, totalRequestedCapacity int) float64 {
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
	Utilization        float64
	Usage              store.NetworkIPUsage
	PendingRequests    int
	CurrentReleasables []nncv1.PodCIDR
	CurrentStatus      []nncv1.PodCIDR
	CurrentAllocation  *nncv1.Allocation
}

func (m *Monitor) getUtilizationInfo(ctx context.Context, network string, nncCopy *nncv1.NodeNetworkConfig) (*UtilizationInfo, error) {
	// GetIPUsageByNetwork returns IP usage details including allocated and cooldown counts,
	// while ignoring CIDR blocks that are marked as Deleting.
	// Note that this includes CIDR blocks in Draining status in both used and total counts.
	// This ensures that processing prefetch (dynamic allocation) is not interfered with
	// (triggered unnecessarily) while we are trying to remove excessive capacity by draining blocks.
	usage, err := m.store.GetIPUsageByNetwork(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP usage: %w", err)
	}

	var currentReleasables []nncv1.PodCIDR
	for _, r := range nncCopy.Spec.ReleasableCIDRs {
		if r.Network == network {
			currentReleasables = append(currentReleasables, r)
		}
	}

	var currentStatus []nncv1.PodCIDR
	for _, s := range nncCopy.Status.PodCIDRs {
		if s.Network == network {
			currentStatus = append(currentStatus, s)
		}
	}

	usedIPs := usage.Allocated // Only allocated, not in cooldown

	pendingRequests := 0
	if m.GetPendingRequestsCount != nil {
		pendingRequests = m.GetPendingRequestsCount(network)
	}

	utilization := m.calculateUtilization(usedIPs, pendingRequests, usage.Total)

	m.logger.Info("Calculated utilization", "network", network, "used", usedIPs, "pending", pendingRequests, "total", usage.Total, "utilization", utilization)

	return &UtilizationInfo{
		Utilization:        utilization,
		Usage:              usage,
		PendingRequests:    pendingRequests,
		CurrentReleasables: currentReleasables,
		CurrentStatus:      currentStatus,
		CurrentAllocation:  getAllocationForNetwork(nncCopy, network),
	}, nil
}

func (m *Monitor) maybeScaleUp(network string, info *UtilizationInfo) int {
	usedIPs := info.Usage.Allocated
	pendingRequests := info.PendingRequests
	localTotal := info.Usage.Total

	currentPods := 0
	if info.CurrentAllocation != nil {
		currentPods = int(info.CurrentAllocation.Pods)
	}

	// TODO: In a burst of release immediately after dynamic allocation is triggered,
	// there may never be new CIDRs to wake up the blocking requests from the daemon server.
	// So we need to callback onCIDR when we check there are enough available IPs.
	if info.Usage.Cooldown > m.cooldownPushbackThreshold {
		m.logger.Info("Too many IPs in cooldown, holding on sending outgoing requests", "network", network, "cooldownCount", info.Usage.Cooldown)
		m.queue.AddAfter("sync", m.cooldownPushbackInterval)
		return currentPods
	}

	// Base line is total local IPs + pending requests
	baseLine := localTotal + pendingRequests
	podsWithBuffer := int(math.Ceil(float64(usedIPs+pendingRequests) / m.targetUtilizationAfterScaleUp))

	newPods := max(podsWithBuffer, baseLine)
	return max(newPods, currentPods)
}

// Note that utilization stats and store CIDR blocks might already be changed by the time we attempt to drain.
// It is possible to drain a newly added block (less likely to happen due to small window), or drained more
// or less blocks than strictly necessary, and that is still fine. The system will self-correct in subsequent cycles.
func (m *Monitor) drainExcessive(ctx context.Context, network string, info *UtilizationInfo) (bool, error) {
	usedIPs := info.Usage.Allocated // Only allocated, not in cooldown
	m.logger.Info(fmt.Sprintf("Utilization falls below %d%% for 8h, evaluating CIDR blocks to drain", int(m.lowUtilizationThreshold*100)), "network", network)

	readyBlocks, err := m.store.GetReadyCIDRBlocksSorted(ctx, network)
	if err != nil {
		return false, fmt.Errorf("failed to get ready blocks: %w", err)
	}

	if len(readyBlocks) <= 1 {
		m.logger.Info("Only initial block or no blocks available, skipping draining", "network", network)
		return false, nil
	}

	blocksToMark := readyBlocks[:len(readyBlocks)-1]
	totalPods := info.Usage.Total + info.PendingRequests
	simulatedUsedIPs := usedIPs + info.PendingRequests
	targetUsedIPs := int(m.lowUtilizationThreshold * float64(totalPods))

	updated := false
	for _, block := range blocksToMark {
		if simulatedUsedIPs >= targetUsedIPs {
			m.logger.Info("Target simulated used IPs reached, stopping marking blocks", "target", targetUsedIPs, "running", simulatedUsedIPs)
			break
		}
		availableIPs := max(0, block.TotalIPs-block.AllocatedIPs)

		err = m.store.DrainCIDRBlock(ctx, block.ID)
		if err != nil {
			return false, fmt.Errorf("failed to drain block %d: %w", block.ID, err)
		}
		m.logger.Info("Marked CIDR block as Draining due to prolonged low utilization", "cidr", block.CIDR)

		simulatedUsedIPs += availableIPs
		updated = true
	}
	return updated, nil
}

func (m *Monitor) maybeDrainExcessive(ctx context.Context, network string, info *UtilizationInfo) bool {
	if info.Utilization >= m.lowUtilizationThreshold {
		if _, ok := m.lowUtilizationTimers[network]; ok {
			delete(m.lowUtilizationTimers, network)
			m.logger.Info("Utilization went above threshold, reset timer", "network", network, "utilization", info.Utilization)
		}
	} else {
		if _, ok := m.lowUtilizationTimers[network]; !ok {
			m.lowUtilizationTimers[network] = time.Now()
			m.logger.Info("Low utilization detected, started timer", "network", network, "utilization", info.Utilization)
		}

		if time.Since(m.lowUtilizationTimers[network]) > m.sustainedLowUtilizationDuration {
			drained, err := m.drainExcessive(ctx, network, info)
			if err != nil {
				m.logger.Error(err, "Failed to handle low utilization", "network", network)
				return false
			}
			if drained {
				delete(m.lowUtilizationTimers, network)
				m.logger.Info("Successfully drained CIDR blocks, resetting low utilization timer", "network", network)
			}
			return drained
		}
	}
	return false
}

func (m *Monitor) reconcileDeletingBlocks(
	ctx context.Context,
	network string,
	currentReleasables []nncv1.PodCIDR,
	currentStatus []nncv1.PodCIDR,
) ([]nncv1.PodCIDR, int, error) {
	// 1. Update local DB to mark expired draining blocks as deleting
	_, err := m.store.FindAndMarkExpiredDrainingCIDRBlocks(ctx, network, m.drainingExpiration)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find and mark expired draining blocks: %w", err)
	}

	// 2. Read all deleting CIDR blocks from local DB for this network
	deletingBlocks, err := m.store.GetDeletingCIDRBlocks(ctx, network)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query deleting blocks: %w", err)
	}

	// Map status CIDRs for quick lookup
	statusMap := make(map[string]nncv1.PodCIDR)
	for _, podCIDR := range currentStatus {
		statusMap[podCIDR.CIDR] = podCIDR
	}

	releasableMap := make(map[string]bool)
	for _, releasable := range currentReleasables {
		releasableMap[releasable.CIDR] = true
	}

	var newReleasables []nncv1.PodCIDR
	var reducePods int

	for _, block := range deletingBlocks {
		podCIDR, inStatus := statusMap[block.CIDR]
		if !inStatus {
			// Case A: Not in CR status anymore -> delete from DB (Reconciliation fallback)
			err = m.store.DeleteCIDRBlock(ctx, block.ID)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to delete released CIDR block %d from store: %w", block.ID, err)
			}
			m.logger.Info("Deleted CIDR block from local DB as it was released by GCE (reconciliation)", "cidrBlockID", block.ID, "cidr", block.CIDR, "network", network)
		} else {
			// Case B: Still in CR status -> keep it in ReleasableCIDRs
			newReleasables = append(newReleasables, podCIDR)

			if !releasableMap[block.CIDR] {
				// Case B.1: Not in release section yet -> add to release section
				reducePods += block.TotalIPs
				releasableMap[block.CIDR] = true
			}
		}
	}

	// TODO: detect out of sync that the items in currentReleasable but no longer in current status or in  deletingBlocks  in local DB, emit metrics or log. This should never happen.

	return newReleasables, reducePods, nil
}

func getAllocationForNetwork(nnc *nncv1.NodeNetworkConfig, network string) *nncv1.Allocation {
	for i := range nnc.Spec.Allocations {
		if nnc.Spec.Allocations[i].Network == network {
			return &nnc.Spec.Allocations[i]
		}
	}
	return nil
}
