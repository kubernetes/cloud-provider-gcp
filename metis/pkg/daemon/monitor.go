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
	"fmt"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/metis/pkg/store"
)

const (
	DefaultMonitorInterval                  = 2 * time.Second
	DefaultReleaseCooldown                  = 1 * time.Minute
	DefaultLowUtilizationThreshold         = 0.50
	DefaultHighUtilizationThreshold        = 0.80
	DefaultTargetUtilizationAfterScaleUp   = 0.75
	DefaultCooldownPushbackThreshold       = 10
	DefaultCooldownPushbackInterval = 2 * time.Second
	DefaultDrainingExpiration       = 5 * time.Hour
	DefaultSustainedLowUtilizationDuration = 8 * time.Hour

	updateMaxRetries                = 10
	defaultMonitorWorkers          = 4
)

type Monitor struct {
	queue                   workqueue.TypedRateLimitingInterface[string]
	nncClient               nncclientset.Interface
	nodeName                string
	store                   *store.Store
	logger                  logr.Logger
	lowUtilizationTimers    map[string]time.Time
	GetPendingRequestsCount func(network string) int
	cooldownPushbackInterval time.Duration
	drainingExpiration       time.Duration
	monitorInterval          time.Duration
	lowUtilizationThreshold         float64
	highUtilizationThreshold        float64
	targetUtilizationAfterScaleUp   float64
	// cooldownPushbackThreshold is the maximum number of IPs allowed to be in cooldown
	// before we hold off on sending new outgoing requests for more CIDR blocks.
	// This prevents requesting more capacity when we have many IPs about to become available.
	cooldownPushbackThreshold       int
	sustainedLowUtilizationDuration time.Duration
}

// MonitorConfig holds the configuration for the Monitor.
type MonitorConfig struct {
	Logger                  logr.Logger
	NNCClient               nncclientset.Interface
	Store                   *store.Store
	NodeName                string
	GetPendingRequestsCount func(network string) int
	CooldownPushbackInterval time.Duration
	DrainingExpiration       time.Duration
	MonitorInterval          time.Duration
	SustainedLowUtilizationDuration time.Duration
}

// NewMonitor creates a new Monitor.
func NewMonitor(cfg MonitorConfig) *Monitor {
	rl := workqueue.DefaultTypedControllerRateLimiter[string]()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[string]{
		Name: "metis-daemon-monitor",
	})

	cooldownPushbackInterval := cfg.CooldownPushbackInterval
	if cooldownPushbackInterval <= 0 {
		cooldownPushbackInterval = DefaultCooldownPushbackInterval
	}
	drainingExpiration := cfg.DrainingExpiration
	if drainingExpiration <= 0 {
		drainingExpiration = DefaultDrainingExpiration
	}
	sustainedLowUtilizationDuration := cfg.SustainedLowUtilizationDuration
	if sustainedLowUtilizationDuration <= 0 {
		sustainedLowUtilizationDuration = DefaultSustainedLowUtilizationDuration
	}
	monitorInterval := cfg.MonitorInterval
	if monitorInterval <= 0 {
		monitorInterval = DefaultMonitorInterval
	}

	return &Monitor{
		queue:                   queue,
		nncClient:               cfg.NNCClient,
		nodeName:                cfg.NodeName,
		store:                   cfg.Store,
		logger:                  cfg.Logger,
		lowUtilizationTimers:    make(map[string]time.Time),
		GetPendingRequestsCount: cfg.GetPendingRequestsCount,
		cooldownPushbackInterval: cooldownPushbackInterval,
		drainingExpiration:       drainingExpiration,
		monitorInterval:          monitorInterval,
		lowUtilizationThreshold:         DefaultLowUtilizationThreshold,
		highUtilizationThreshold:        DefaultHighUtilizationThreshold,
		targetUtilizationAfterScaleUp:   DefaultTargetUtilizationAfterScaleUp,
		cooldownPushbackThreshold:       DefaultCooldownPushbackThreshold,
		sustainedLowUtilizationDuration: sustainedLowUtilizationDuration,
	}
}

// Run starts the workers and periodic enqueuer.
func (m *Monitor) Run(ctx context.Context, workers int) {
	defer m.queue.ShutDown()

	m.logger.Info("Starting IPAM monitor", "workers", workers)
	defer m.logger.Info("Stopping IPAM monitor")

	// Periodic enqueuer
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		networks, err := m.store.GetAllNetworks(ctx)
		if err != nil {
			m.logger.Error(err, "failed to get networks for periodic sync")
			return
		}
		for _, network := range networks {
			m.queue.Add(network)
		}
	}, m.monitorInterval)

	// Periodic expired draining blocks handler
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		networks, err := m.store.GetAllNetworks(ctx)
		if err != nil {
			m.logger.Error(err, "failed to get networks for expired draining blocks check")
			return
		}
		for _, network := range networks {
			info, err := m.getUtilizationInfo(ctx, network)
			if err != nil {
				m.logger.Error(err, "failed to get utilization info", "network", network)
				continue
			}
			updated, err := m.handleExpiredDrainingBlocks(ctx, network, info.NncCopy, info.CurrentAllocation)
			if err != nil {
				m.logger.Error(err, "failed to handle expired draining blocks", "network", network)
				continue
			}
			if updated {
				if err := m.patchNNC(ctx, info); err != nil {
					m.logger.Error(err, "failed to patch NNC for expired draining blocks", "network", network)
				}
			}
		}
	}, 1*time.Minute)

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, m.runWorker, time.Second)
	}

	<-ctx.Done()
}

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

	err := m.syncNetwork(ctx, item)
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
func (m *Monitor) Enqueue(network string) {
	m.queue.Add(network)
}

func (m *Monitor) syncNetwork(ctx context.Context, network string) error {
	m.logger.Info("Evaluating IP usage for dynamic allocation", "node", m.nodeName)

	info, err := m.getUtilizationInfo(ctx, network)
	if err != nil {
		return err
	}

	if info.InitialIPs == 0 {
		m.logger.Info("Initial IPs is 0, skipping dynamic allocation", "network", network)
		return nil
	}

	updated := false
	if info.Utilization > m.highUtilizationThreshold {
		updatedBranch, err := m.handleHighUtilization(network, info)
		if err != nil {
			return err
		}
		if updatedBranch {
			updated = true
		}
	}

	m.maybeDrainExcessive(ctx, network, info)

	if updated {
		return m.patchNNC(ctx, info)
	}
	return nil
}

func (m *Monitor) patchNNC(ctx context.Context, info *UtilizationInfo) error {
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

	_, err = m.nncClient.NetworkingV1().NodeNetworkConfigs().Patch(ctx, m.nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch NodeNetworkConfig: %w", err)
	}
	m.logger.Info("Successfully patched NodeNetworkConfig", "allocations", info.NncCopy.Spec.Allocations, "releasableCIDRs", info.NncCopy.Spec.ReleasableCIDRs)
	return nil
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
	Utilization            float64
	Usage                  store.NetworkIPUsage
	PendingRequests        int
	InitialIPs            int
	TargetPods             int
	CurrentAllocation      *nncv1.Allocation
	NncCopy                *nncv1.NodeNetworkConfig
	TotalRequestedCapacity int
}

func (m *Monitor) getUtilizationInfo(ctx context.Context, network string) (*UtilizationInfo, error) {
	usage, err := m.store.GetIPUsageByNetwork(ctx, network)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP usage: %w", err)
	}

	nnc, err := m.getNodeNetworkConfig(ctx)
	if err != nil {
		return nil, err
	}

	nncCopy := nnc.DeepCopy()
	usedIPs := usage.Allocated + usage.Cooldown

	initialIPs, err := m.store.GetInitialIPCount(ctx, network)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			initialIPs = 0
		} else {
			return nil, fmt.Errorf("failed to query initial IPs for network %s: %w", network, err)
		}
	}

	pendingRequests := 0
	if m.GetPendingRequestsCount != nil {
		pendingRequests = m.GetPendingRequestsCount(network)
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
	utilization := m.calculateUtilization(usedIPs, pendingRequests, totalRequestedCapacity)

	m.logger.Info("Calculated utilization", "network", network, "used", usedIPs, "pending", pendingRequests, "total", usage.Total, "totalRequestedCapacity", totalRequestedCapacity, "utilization", utilization)

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

func (m *Monitor) getNodeNetworkConfig(ctx context.Context) (*nncv1.NodeNetworkConfig, error) {
	if m.nncClient != nil {
		nnc, err := m.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, m.nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get NodeNetworkConfig from API: %w", err)
		}
		return nnc, nil
	}
	return nil, fmt.Errorf("no client available to fetch NodeNetworkConfig")
}

func (m *Monitor) handleHighUtilization(network string, info *UtilizationInfo) (bool, error) {
	usedIPs := info.Usage.Allocated + info.Usage.Cooldown
	if info.Usage.Cooldown > m.cooldownPushbackThreshold {
		m.logger.Info("Too many IPs in cooldown, holding on sending outgoing requests", "network", network, "cooldownCount", info.Usage.Cooldown)
		m.queue.AddAfter(network, m.cooldownPushbackInterval)
		return false, nil
	}

	m.logger.Info(fmt.Sprintf("Utilization exceeds %d%%, triggering dynamic allocation", int(m.highUtilizationThreshold*100)), "network", network)

	newTotalCapacity := int(float64(usedIPs+info.PendingRequests)/m.targetUtilizationAfterScaleUp) + 1
	newTargetPods := newTotalCapacity - info.InitialIPs
	if newTargetPods < 0 {
		newTargetPods = 0
	}

	if newTargetPods <= info.TargetPods {
		m.logger.Info("Calculated new target pods is less than or equal to current target pods, skipping scale up", "network", network, "currentPods", info.TargetPods, "newPods", newTargetPods)
		return false, nil
	}

	m.logger.Info("Scaling up", "network", network, "currentPods", info.TargetPods, "newPods", newTargetPods)

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

func (m *Monitor) handleLowUtilization(ctx context.Context, network string, info *UtilizationInfo) (bool, error) {
	usedIPs := info.Usage.Allocated + info.Usage.Cooldown
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
	totalRequestedCapacity := info.InitialIPs + info.TargetPods
	simulatedUsedIPs := usedIPs + info.PendingRequests
	targetUsedIPs := int(m.lowUtilizationThreshold * float64(totalRequestedCapacity))

	updated := false
	for _, block := range blocksToMark {
		if simulatedUsedIPs >= targetUsedIPs {
			m.logger.Info("Target simulated used IPs reached, stopping marking blocks", "target", targetUsedIPs, "running", simulatedUsedIPs)
			break
		}
		availableIPs := block.TotalIPs - block.AllocatedIPs
		if availableIPs < 0 {
			availableIPs = 0
		}

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
			drained, err := m.handleLowUtilization(ctx, network, info)
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

func (m *Monitor) handleExpiredDrainingBlocks(ctx context.Context, network string, nncCopy *nncv1.NodeNetworkConfig, currentAllocation *nncv1.Allocation) (bool, error) {
	expiredBlocks, err := m.store.FindAndMarkExpiredDrainingCIDRBlocks(ctx, network, m.drainingExpiration)
	if err != nil {
		return false, fmt.Errorf("failed to query and mark draining cidr blocks: %w", err)
	}

	var reducePods int
	updated := false

	deletingBlocks, err := m.store.GetDeletingCIDRBlocks(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to query deleting cidr blocks: %w", err)
	}

	statusMap := make(map[string]nncv1.PodCIDR)
	for _, podCIDR := range nncCopy.Status.PodCIDRs {
		if podCIDR.Network == network {
			statusMap[podCIDR.CIDR] = podCIDR
		}
	}

	releasableSet := make(map[string]bool)
	for _, releasable := range nncCopy.Spec.ReleasableCIDRs {
		releasableSet[releasable.CIDR] = true
	}

	for _, block := range expiredBlocks {
		reducePods += block.TotalIPs

		if !releasableSet[block.CIDR] {
			podCIDR, inStatus := statusMap[block.CIDR]
			if !inStatus {
				m.logger.Error(nil, "failed to find CIDR in CR status for draining block", "cidr", block.CIDR)
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
			m.logger.Info("Re-adding deleting block to ReleasableCIDRs for reconciliation", "cidr", block.CIDR)
			nncCopy.Spec.ReleasableCIDRs = append(nncCopy.Spec.ReleasableCIDRs, podCIDR)
			releasableSet[block.CIDR] = true
			reducePods += block.TotalIPs
			updated = true
		}
	}

	if updated {
		m.logger.Info("Marked CIDR blocks as Deleting and added to ReleasableCIDRs", "network", network)

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
