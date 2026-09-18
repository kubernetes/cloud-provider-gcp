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
	// GetCandidateRanges returns the candidate secondary range names for pod
	// IP allocation in the cluster's default subnetwork.
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

// SecondaryRangeConfig represents a secondary range name, its optional
// subnetwork, and its lifecycle status.
type SecondaryRangeConfig struct {
	Subnetwork string
	Name       string
	Status     SecondaryRangeStatus
}

// ParseSecondaryRangeConfigs parses a slice of "range", "subnetwork/range",
// "subnetwork:range", or "range=STATUS" entries (comma-separated or distinct
// elements). If status is omitted or empty, it defaults to SecondaryRangeActive
// ("ACTIVE"). Status is case-insensitive. If multiple entries specify the same
// subnetwork and range name, the last entry's status takes precedence while
// preserving original order.
func ParseSecondaryRangeConfigs(entries []string) []SecondaryRangeConfig {
	if len(entries) == 0 {
		return nil
	}
	var keys []string
	cfgMap := make(map[string]SecondaryRangeConfig)

	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			status := SecondaryRangeActive
			target := part

			if idx := strings.Index(part, "="); idx != -1 {
				target = strings.TrimSpace(part[:idx])
				rawStatus := strings.ToUpper(strings.TrimSpace(part[idx+1:]))
				switch rawStatus {
				case string(SecondaryRangeDraining):
					status = SecondaryRangeDraining
				case string(SecondaryRangeActive), "":
					status = SecondaryRangeActive
				default:
					klog.Warningf("Unknown secondary range status %q for range %q; defaulting to %s", rawStatus, target, SecondaryRangeActive)
					status = SecondaryRangeActive
				}
			}

			var subnetwork, rangeName string
			if idx := strings.LastIndex(target, ":"); idx != -1 {
				subnetwork = canonicalSubnetwork(target[:idx])
				rangeName = strings.TrimSpace(target[idx+1:])
			} else if idx := strings.LastIndex(target, "/"); idx != -1 {
				subnetwork = canonicalSubnetwork(target[:idx])
				rangeName = strings.TrimSpace(target[idx+1:])
			} else {
				subnetwork = ""
				rangeName = strings.TrimSpace(target)
			}

			if rangeName == "" {
				continue
			}

			key := subnetwork + "/" + rangeName
			if _, exists := cfgMap[key]; !exists {
				keys = append(keys, key)
			}
			cfgMap[key] = SecondaryRangeConfig{
				Subnetwork: subnetwork,
				Name:       rangeName,
				Status:     status,
			}
		}
	}

	var configs []SecondaryRangeConfig
	for _, key := range keys {
		configs = append(configs, cfgMap[key])
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
// names, grouped by subnetwork, typically configured via CLI flag.
type StaticRangeProvider struct {
	defaultSubnet  string
	configs        []SecondaryRangeConfig
	activeBySubnet map[string][]string
}

// NewStaticRangeProvider returns a CandidateRangeProvider backed by a fixed
// slice of secondary ranges with optional subnetwork and lifecycle statuses
// (e.g. "subnet1/range1=ACTIVE,subnet2/range2=DRAINING" or "range1=ACTIVE").
// Only ACTIVE ranges will be returned as candidate ranges for allocation.
func NewStaticRangeProvider(ranges []string) *StaticRangeProvider {
	return NewStaticRangeProviderWithDefaultSubnet(ranges, "")
}

// NewStaticRangeProviderWithDefaultSubnet returns a CandidateRangeProvider
// backed by a fixed slice of secondary ranges and an explicit default
// subnetwork name.
func NewStaticRangeProviderWithDefaultSubnet(ranges []string, defaultSubnet string) *StaticRangeProvider {
	configs := ParseSecondaryRangeConfigs(ranges)
	activeBySubnet := make(map[string][]string)
	for _, cfg := range configs {
		if cfg.Status == SecondaryRangeActive {
			subnet := canonicalSubnetwork(cfg.Subnetwork)
			activeBySubnet[subnet] = append(activeBySubnet[subnet], cfg.Name)
		}
	}
	return &StaticRangeProvider{
		defaultSubnet:  canonicalSubnetwork(defaultSubnet),
		configs:        configs,
		activeBySubnet: activeBySubnet,
	}
}

// GetCandidateRanges returns the active candidate secondary ranges for pod IP
// allocation in the default subnetwork.
func (s *StaticRangeProvider) GetCandidateRanges(ctx context.Context) ([]string, error) {
	var candidates []string
	seen := sets.NewString()

	addRange := func(r string) {
		if !seen.Has(r) {
			seen.Insert(r)
			candidates = append(candidates, r)
		}
	}

	// 1. Explicitly qualified ranges matching the default subnetwork:
	if s.defaultSubnet != "" {
		if ranges, ok := s.activeBySubnet[s.defaultSubnet]; ok {
			for _, r := range ranges {
				addRange(r)
			}
		}
	}

	// 2. Unqualified ranges (which default to the default subnetwork):
	if ranges, ok := s.activeBySubnet[""]; ok {
		for _, r := range ranges {
			addRange(r)
		}
	}

	// 3. Fallback if defaultSubnet was unspecified and all ranges were
	// qualified with a single subnetwork:
	if s.defaultSubnet == "" && len(candidates) == 0 && len(s.activeBySubnet) == 1 {
		for _, ranges := range s.activeBySubnet {
			for _, r := range ranges {
				addRange(r)
			}
		}
	}

	return candidates, nil
}

// GetCandidateRangesForSubnetwork returns the active candidate secondary
// ranges for a specific subnetwork. If subnetwork is empty or matches the
// default subnetwork, it returns candidates for the default subnetwork.
func (s *StaticRangeProvider) GetCandidateRangesForSubnetwork(subnetwork string) []string {
	canonical := canonicalSubnetwork(subnetwork)
	if canonical == "" || canonical == s.defaultSubnet {
		ranges, _ := s.GetCandidateRanges(context.Background())
		return ranges
	}
	if ranges, ok := s.activeBySubnet[canonical]; ok {
		return append([]string{}, ranges...)
	}
	return nil
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
// from the GKE Container API for each subnetwork.
// Note: This refresher is a stop-gap solution until the
// --multi-secondary-ranges CLI flag is populated directly by GKE Cluster
// Server.
type PodRangeRefresher struct {
	loader          ContainerClusterLoader
	refreshInterval time.Duration
	clock           clock.WithTicker

	mu              sync.RWMutex
	cachedBySubnet  map[string][]string
	defaultSubnet   string
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

// GetCandidateRanges returns the currently cached candidate ranges for the
// cluster's default subnetwork. If no ranges have been cached yet, it triggers
// a synchronous refresh before returning.
func (r *PodRangeRefresher) GetCandidateRanges(ctx context.Context) ([]string, error) {
	return r.GetCandidateRangesForSubnetwork(ctx, "")
}

// GetCandidateRangesForSubnetwork returns the currently cached candidate ranges
// for the specified subnetwork. If subnetwork is empty, it returns the
// candidate ranges for the default subnetwork.
func (r *PodRangeRefresher) GetCandidateRangesForSubnetwork(ctx context.Context, subnetwork string) ([]string, error) {
	r.mu.RLock()
	if !r.lastRefreshed.IsZero() {
		ranges := lookupRanges(r.cachedBySubnet, r.defaultSubnet, subnetwork)
		r.mu.RUnlock()
		return ranges, nil
	}
	r.mu.RUnlock()

	// Cache has not been populated yet: perform initial synchronous fetch
	cachedBySubnet, defaultSubnet, err := r.refresh(ctx)
	if err != nil {
		return nil, err
	}
	return lookupRanges(cachedBySubnet, defaultSubnet, subnetwork), nil
}

func lookupRanges(cachedBySubnet map[string][]string, defaultSubnet, subnetwork string) []string {
	canonical := canonicalSubnetwork(subnetwork)

	// 1. If a specific subnetwork was requested and cached, return its
	// candidates.
	if canonical != "" {
		if ranges, ok := cachedBySubnet[canonical]; ok {
			return append([]string{}, ranges...)
		}
	}

	// 2. If subnetwork is empty or matches the default subnetwork:
	if canonical == "" || canonical == defaultSubnet {
		if ranges, ok := cachedBySubnet[""]; ok {
			return append([]string{}, ranges...)
		}
		if defaultSubnet != "" {
			if ranges, ok := cachedBySubnet[defaultSubnet]; ok {
				return append([]string{}, ranges...)
			}
		}
	}

	// 3. Backwards compatibility for single-subnet clusters:
	// If only default ranges exist (key ""), return them.
	if len(cachedBySubnet) == 1 {
		if ranges, ok := cachedBySubnet[""]; ok {
			return append([]string{}, ranges...)
		}
	}

	return nil
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
		if _, _, err := r.refresh(ctx); err != nil {
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
			if _, _, err := r.refresh(ctx); err != nil {
				klog.Errorf("Periodic refresh in PodRangeRefresher failed: %v", err)
			}
			cancel()
		case <-r.invalidateCh:
			klog.V(2).Info("PodRangeRefresher received invalidation signal; refreshing immediately")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if _, _, err := r.refresh(ctx); err != nil {
				klog.Errorf("On-demand refresh in PodRangeRefresher failed: %v", err)
			}
			cancel()
		}
	}
}

func (r *PodRangeRefresher) refresh(ctx context.Context) (map[string][]string, string, error) {
	cluster, err := r.loader(ctx)
	if err != nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		if len(r.cachedBySubnet) > 0 {
			klog.Warningf("Failed to refresh candidate pod secondary ranges from Container API, retaining last known cached ranges (%v): %v", r.cachedBySubnet, err)
			copyMap := make(map[string][]string, len(r.cachedBySubnet))
			for k, v := range r.cachedBySubnet {
				copyMap[k] = append([]string{}, v...)
			}
			return copyMap, r.defaultSubnet, nil
		}
		return nil, "", fmt.Errorf("failed to fetch cluster from Container API: %w", err)
	}

	rangesBySubnet, defaultSubnet := extractPodSecondaryRanges(cluster)

	r.mu.Lock()
	r.cachedBySubnet = rangesBySubnet
	r.defaultSubnet = defaultSubnet
	r.lastRefreshed = r.clock.Now()
	r.mu.Unlock()

	klog.V(2).Infof("Refreshed candidate pod secondary ranges (default subnet %q): %v", defaultSubnet, rangesBySubnet)
	copyMap := make(map[string][]string, len(rangesBySubnet))
	for k, v := range rangesBySubnet {
		copyMap[k] = append([]string{}, v...)
	}
	return copyMap, defaultSubnet, nil
}

// extractPodSecondaryRanges parses pod secondary range names associated with
// their subnetworks from a cluster object. It returns a map of canonical
// subnetwork names to their list of candidate secondary ranges, and the
// canonical name of the default subnetwork.
// The map also contains an entry under key "" pointing to the default
// subnetwork ranges for backwards compatibility. Service ranges (e.g.
// ServicesSecondaryRangeName) are explicitly excluded.
func extractPodSecondaryRanges(cluster *container.Cluster) (map[string][]string, string) {
	if cluster == nil || cluster.IpAllocationPolicy == nil {
		return nil, ""
	}

	ipPolicy := cluster.IpAllocationPolicy
	result := make(map[string][]string)

	// Determine cluster's default subnetwork
	var defaultSubnet string
	if cluster.Subnetwork != "" {
		defaultSubnet = canonicalSubnetwork(cluster.Subnetwork)
	} else if ipPolicy.SubnetworkName != "" {
		defaultSubnet = canonicalSubnetwork(ipPolicy.SubnetworkName)
	}

	// 1. Primary pod secondary range (default subnetwork)
	var defaultRanges []string
	if ipPolicy.ClusterSecondaryRangeName != "" {
		defaultRanges = append(defaultRanges, ipPolicy.ClusterSecondaryRangeName)
	}

	// 2. Additional pod secondary ranges (default subnetwork)
	if ipPolicy.AdditionalPodRangesConfig != nil {
		for _, name := range ipPolicy.AdditionalPodRangesConfig.PodRangeNames {
			if name != "" {
				defaultRanges = append(defaultRanges, name)
			}
		}
	}

	defaultRanges = deduplicateAndFilter(defaultRanges)
	if len(defaultRanges) > 0 {
		result[""] = defaultRanges
		if defaultSubnet != "" {
			result[defaultSubnet] = append([]string{}, defaultRanges...)
		}
	}

	// 3. Additional subnetworks (Multi-Subnet Clusters)
	for _, additional := range ipPolicy.AdditionalIpRangesConfigs {
		if additional == nil {
			continue
		}
		// If subnetwork is DRAINING, exclude from candidate ranges
		if strings.EqualFold(additional.Status, string(SecondaryRangeDraining)) {
			continue
		}
		subnetName := canonicalSubnetwork(additional.Subnetwork)
		if subnetName == "" {
			continue
		}
		filtered := deduplicateAndFilter(additional.PodIpv4RangeNames)
		if len(filtered) > 0 {
			result[subnetName] = deduplicateAndFilter(append(result[subnetName], filtered...))
		}
	}

	return result, defaultSubnet
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
