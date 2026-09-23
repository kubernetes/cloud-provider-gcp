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
	"regexp"
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

// ContainerClusterLoader is a function type that queries the Container API for
// a cluster object.
type ContainerClusterLoader func(ctx context.Context) (*container.Cluster, error)

// PodRangeRefresher periodically refreshes candidate pod secondary ranges
// for the default subnetwork from the GKE Container API.
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
		targetClusterName := extractContainerClusterName(clusterName)
		clusterPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, targetClusterName)
		klog.V(4).Infof("Fetching cluster configuration from Container API: %s", clusterPath)
		cluster, err := svc.Projects.Locations.Clusters.Get(clusterPath).Context(ctx).Do()
		if err != nil && targetClusterName != clusterName {
			klog.V(4).Infof("Fetching cluster with extracted name %q failed (%v); falling back to raw cluster name %q", targetClusterName, err, clusterName)
			fallbackPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", projectID, location, clusterName)
			cluster, err = svc.Projects.Locations.Clusters.Get(fallbackPath).Context(ctx).Do()
		}
		return cluster, err
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
	r.mu.RLock()
	if !r.lastRefreshed.IsZero() {
		ranges := append([]string{}, r.cachedRanges...)
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
			return append([]string{}, r.cachedRanges...), nil
		}
		return nil, fmt.Errorf("failed to fetch cluster from Container API: %w", err)
	}

	ranges := extractPodSecondaryRanges(cluster)

	r.mu.Lock()
	r.cachedRanges = ranges
	r.lastRefreshed = r.clock.Now()
	r.mu.Unlock()

	klog.V(2).Infof("Refreshed candidate pod secondary ranges from Container API: %v", ranges)
	return append([]string{}, ranges...), nil
}

// extractPodSecondaryRanges parses pod secondary range names for the default
// subnetwork from a cluster object. It extracts the primary pod range and any
// additional pod secondary ranges configured in IpAllocationPolicy.
func extractPodSecondaryRanges(cluster *container.Cluster) []string {
	if cluster == nil || cluster.IpAllocationPolicy == nil {
		return nil
	}

	ipPolicy := cluster.IpAllocationPolicy
	var ranges []string

	// 1. Primary pod secondary range (default subnetwork)
	if ipPolicy.ClusterSecondaryRangeName != "" {
		ranges = append(ranges, ipPolicy.ClusterSecondaryRangeName)
	}

	// 2. Additional pod secondary ranges (default subnetwork)
	if ipPolicy.AdditionalPodRangesConfig != nil {
		for _, name := range ipPolicy.AdditionalPodRangesConfig.PodRangeNames {
			if name != "" {
				ranges = append(ranges, name)
			}
		}
	}

	return deduplicateAndFilter(ranges)
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

var gkeInstancePrefixRegex = regexp.MustCompile(`^gke-(.+)-[0-9a-fA-F]{8}$`)

// extractContainerClusterName extracts the user-facing GKE cluster name from an
// instance prefix or cluster name formatted as "gke-<cluster-name>-<8-char-hash>".
// In GKE, --cluster-name passed to CCM is names.InstancePrefix(o.Cluster) ("gke-%s-%s").
// The Container API expects the user-facing cluster name ("%s").
// If the input does not match this format, it is returned unchanged.
func extractContainerClusterName(clusterName string) string {
	matches := gkeInstancePrefixRegex.FindStringSubmatch(clusterName)
	if len(matches) == 2 && len(matches[1]) > 0 {
		return matches[1]
	}
	return clusterName
}

