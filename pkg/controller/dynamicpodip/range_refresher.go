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

// Package dynamicpodip implements controllers for dynamic pod IP allocation
// on GCE.
//
// Note: PodRangeRefresher in this file is a stop-gap solution to dynamically
// discover candidate secondary ranges from the GKE Container API until the
// --multi-secondary-ranges CLI flag is populated directly by GKE Cluster
// Server.
package dynamicpodip

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	container "google.golang.org/api/container/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

const (
	// DefaultPodRangeRefreshInterval is the periodic interval to refresh
	// secondary pod ranges from the Container API.
	DefaultPodRangeRefreshInterval = 5 * time.Minute

	// minInvalidateInterval prevents stampeding the Container API when multiple
	// nodes fail allocations simultaneously.
	minInvalidateInterval = 10 * time.Second
)

// CandidateRangeProvider returns candidate secondary range names for alias IP
// range allocations.
type CandidateRangeProvider interface {
	// GetCandidateRanges returns the list of candidate secondary range names.
	GetCandidateRanges(ctx context.Context) ([]string, error)
	// Invalidate marks the candidate range cache as stale, triggering an
	// immediate refresh.
	Invalidate()
	// Run runs background maintenance (e.g. periodic refresh) until stopCh is
	// closed.
	Run(stopCh <-chan struct{})
}

// SecondaryRangeStatus represents the lifecycle state of a secondary IP range.
type SecondaryRangeStatus string

const (
	// SecondaryRangeActive indicates the secondary range is active for new pod
	// IP allocations.
	SecondaryRangeActive SecondaryRangeStatus = "ACTIVE"
	// SecondaryRangeDraining indicates the secondary range is draining and must
	// not be used for new allocations.
	SecondaryRangeDraining SecondaryRangeStatus = "DRAINING"
)

// SecondaryRangeConfig represents a secondary range name and its lifecycle
// status.
type SecondaryRangeConfig struct {
	Name   string
	Status SecondaryRangeStatus
}

// ParseSecondaryRangeConfigs parses a slice of "range" or "range=STATUS"
// entries (comma-separated or distinct elements). If status is omitted or
// empty, it defaults to SecondaryRangeActive ("ACTIVE"). Status is
// case-insensitive. If multiple entries specify the same range name, the last
// entry's status takes precedence while preserving original order.
func ParseSecondaryRangeConfigs(entries []string) []SecondaryRangeConfig {
	if len(entries) == 0 {
		return nil
	}
	var names []string
	statusMap := make(map[string]SecondaryRangeStatus)

	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var name string
			status := SecondaryRangeActive

			if idx := strings.Index(part, "="); idx != -1 {
				name = strings.TrimSpace(part[:idx])
				rawStatus := strings.ToUpper(strings.TrimSpace(part[idx+1:]))
				switch rawStatus {
				case string(SecondaryRangeDraining):
					status = SecondaryRangeDraining
				case string(SecondaryRangeActive), "":
					status = SecondaryRangeActive
				default:
					klog.Warningf("Unknown secondary range status %q for range %q; defaulting to %s", rawStatus, name, SecondaryRangeActive)
					status = SecondaryRangeActive
				}
			} else {
				name = part
			}

			if name == "" {
				continue
			}

			if _, exists := statusMap[name]; !exists {
				names = append(names, name)
			}
			statusMap[name] = status
		}
	}

	var configs []SecondaryRangeConfig
	for _, name := range names {
		configs = append(configs, SecondaryRangeConfig{
			Name:   name,
			Status: statusMap[name],
		})
	}
	return configs
}

// FilterActiveSecondaryRanges returns only the names of secondary ranges that
// are in ACTIVE status.
func FilterActiveSecondaryRanges(configs []SecondaryRangeConfig) []string {
	var active []string
	for _, cfg := range configs {
		if cfg.Status == SecondaryRangeActive {
			active = append(active, cfg.Name)
		}
	}
	return active
}

// StaticRangeProvider provides a static list of candidate secondary range
// names, typically configured via CLI flag.
type StaticRangeProvider struct {
	configs []SecondaryRangeConfig
	active  []string
}

// NewStaticRangeProvider returns a CandidateRangeProvider backed by a fixed
// slice of secondary ranges with optional lifecycle statuses (e.g.
// "range1=ACTIVE", "range2=DRAINING", or "range1"). Only ACTIVE ranges will be
// returned as candidate ranges for allocation.
func NewStaticRangeProvider(ranges []string) *StaticRangeProvider {
	configs := ParseSecondaryRangeConfigs(ranges)
	active := FilterActiveSecondaryRanges(configs)
	return &StaticRangeProvider{
		configs: configs,
		active:  active,
	}
}

// GetCandidateRanges returns the active candidate secondary ranges for pod IP
// allocation.
func (s *StaticRangeProvider) GetCandidateRanges(ctx context.Context) ([]string, error) {
	return append([]string{}, s.active...), nil
}

// Configs returns all configured secondary ranges and their statuses.
func (s *StaticRangeProvider) Configs() []SecondaryRangeConfig {
	return append([]SecondaryRangeConfig{}, s.configs...)
}

// Invalidate is a no-op for static ranges.
func (s *StaticRangeProvider) Invalidate() {}

// Run terminates when stopCh is closed.
func (s *StaticRangeProvider) Run(stopCh <-chan struct{}) {
	<-stopCh
}

// ContainerClusterLoader is a function type that queries the Container API for
// a cluster object.
type ContainerClusterLoader func(ctx context.Context) (*container.Cluster, error)

// PodRangeRefresher periodically refreshes candidate pod secondary ranges
// from the GKE Container API.
// Note: This refresher is a stop-gap solution until the
// --multi-secondary-ranges CLI flag is populated directly by GKE Cluster
// Server.
type PodRangeRefresher struct {
	loader          ContainerClusterLoader
	refreshInterval time.Duration
	clock           clock.WithTicker

	mu              sync.RWMutex
	cachedRanges    []string
	lastRefreshed   time.Time
	lastInvalidated time.Time

	invalidateCh chan struct{}
}

// NewPodRangeRefresher constructs a PodRangeRefresher using the given GCE
// cloud provider and cluster name.
func NewPodRangeRefresher(
	gceCloud *gce.Cloud,
	clusterName string,
	refreshInterval time.Duration,
	clk clock.WithTicker,
) *PodRangeRefresher {
	if clk == nil {
		clk = clock.RealClock{}
	}
	if refreshInterval <= 0 {
		refreshInterval = DefaultPodRangeRefreshInterval
	}

	projectID := gceCloud.ProjectID()
	location := gceCloud.Region()
	if !gceCloud.Regional() {
		location = gceCloud.LocalZone()
	}

	loader := func(ctx context.Context) (*container.Cluster, error) {
		svc := gceCloud.ContainerService()
		if svc == nil {
			return nil, fmt.Errorf("container service not available on GCE Cloud")
		}
		if clusterName == "" {
			return nil, fmt.Errorf("cluster name is empty")
		}
		clusterPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterName)
		klog.V(4).Infof("Fetching cluster configuration from Container API: %s", clusterPath)
		return svc.Projects.Locations.Clusters.Get(clusterPath).Context(ctx).Do()
	}

	return NewPodRangeRefresherWithLoader(loader, refreshInterval, clk)
}

// NewPodRangeRefresherWithLoader constructs a PodRangeRefresher with a custom
// loader function (useful for testing).
func NewPodRangeRefresherWithLoader(
	loader ContainerClusterLoader,
	refreshInterval time.Duration,
	clk clock.WithTicker,
) *PodRangeRefresher {
	if clk == nil {
		clk = clock.RealClock{}
	}
	if refreshInterval <= 0 {
		refreshInterval = DefaultPodRangeRefreshInterval
	}

	return &PodRangeRefresher{
		loader:          loader,
		refreshInterval: refreshInterval,
		clock:           clk,
		invalidateCh:    make(chan struct{}, 1),
	}
}

// GetCandidateRanges returns the currently cached candidate ranges. If no
// ranges have been cached yet, it triggers a synchronous refresh before
// returning.
func (r *PodRangeRefresher) GetCandidateRanges(ctx context.Context) ([]string, error) {
	r.mu.RLock()
	if !r.lastRefreshed.IsZero() {
		ranges := make([]string, len(r.cachedRanges))
		copy(ranges, r.cachedRanges)
		r.mu.RUnlock()
		return ranges, nil
	}
	r.mu.RUnlock()

	// Cache has not been populated yet: perform initial synchronous fetch
	return r.refresh(ctx)
}

// Invalidate triggers an on-demand refresh of candidate ranges, rate-limited
// by minInvalidateInterval.
func (r *PodRangeRefresher) Invalidate() {
	r.mu.Lock()
	now := r.clock.Now()
	if now.Sub(r.lastInvalidated) < minInvalidateInterval {
		r.mu.Unlock()
		return
	}
	r.lastInvalidated = now
	r.mu.Unlock()

	select {
	case r.invalidateCh <- struct{}{}:
	default:
	}
}

// Run starts the background refresh loop until stopCh is closed.
func (r *PodRangeRefresher) Run(stopCh <-chan struct{}) {
	klog.Info("Starting PodRangeRefresher background loop")

	// Perform initial refresh if not yet populated
	r.mu.RLock()
	uninitialized := r.lastRefreshed.IsZero()
	r.mu.RUnlock()
	if uninitialized {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if _, err := r.refresh(ctx); err != nil {
			klog.Errorf("Initial fetch in PodRangeRefresher failed: %v", err)
		}
		cancel()
	}

	ticker := r.clock.NewTicker(r.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			klog.Info("Stopping PodRangeRefresher background loop")
			return
		case <-ticker.C():
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, err := r.refresh(ctx); err != nil {
				klog.Errorf("Periodic refresh in PodRangeRefresher failed: %v", err)
			}
			cancel()
		case <-r.invalidateCh:
			klog.V(2).Info("PodRangeRefresher received invalidation signal; refreshing immediately")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, err := r.refresh(ctx); err != nil {
				klog.Errorf("On-demand refresh in PodRangeRefresher failed: %v", err)
			}
			cancel()
		}
	}
}

func (r *PodRangeRefresher) refresh(ctx context.Context) ([]string, error) {
	cluster, err := r.loader(ctx)
	if err != nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		if len(r.cachedRanges) > 0 {
			klog.Warningf("Failed to refresh candidate pod secondary ranges from Container API, retaining last known cached ranges (%v): %v", r.cachedRanges, err)
			ranges := make([]string, len(r.cachedRanges))
			copy(ranges, r.cachedRanges)
			return ranges, nil
		}
		return nil, fmt.Errorf("failed to fetch cluster from Container API: %w", err)
	}

	ranges := extractPodSecondaryRanges(cluster)

	r.mu.Lock()
	r.cachedRanges = ranges
	r.lastRefreshed = r.clock.Now()
	r.mu.Unlock()

	klog.V(2).Infof("Refreshed candidate pod secondary ranges: %v", ranges)
	result := make([]string, len(ranges))
	copy(result, ranges)
	return result, nil
}

// extractPodSecondaryRanges parses the primary and additional pod secondary
// range names from a cluster object. Service ranges (e.g.
// ServicesSecondaryRangeName) are explicitly excluded.
func extractPodSecondaryRanges(cluster *container.Cluster) []string {
	if cluster == nil || cluster.IpAllocationPolicy == nil {
		return nil
	}

	var rawRanges []string
	ipPolicy := cluster.IpAllocationPolicy

	// 1. Primary pod secondary range
	if ipPolicy.ClusterSecondaryRangeName != "" {
		rawRanges = append(rawRanges, ipPolicy.ClusterSecondaryRangeName)
	}

	// 2. Additional pod secondary ranges
	if ipPolicy.AdditionalPodRangesConfig != nil {
		for _, name := range ipPolicy.AdditionalPodRangesConfig.PodRangeNames {
			if name != "" {
				rawRanges = append(rawRanges, name)
			}
		}
	}

	return deduplicateAndFilter(rawRanges)
}

func deduplicateAndFilter(ranges []string) []string {
	seen := sets.NewString()
	var result []string
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r != "" && !seen.Has(r) {
			seen.Insert(r)
			result = append(result, r)
		}
	}
	return result
}
