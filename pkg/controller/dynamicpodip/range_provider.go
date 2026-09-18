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
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

// CandidateRangeProvider returns candidate secondary range names for alias IP
// range allocations.
type CandidateRangeProvider interface {
	// GetCandidateRanges returns candidate secondary range names for pod IP
	// allocation in the cluster's default subnetwork.
	GetCandidateRanges(ctx context.Context) ([]string, error)
	// Invalidate marks the candidate range cache as stale, triggering an
	// immediate refresh.
	Invalidate()
	// Run runs background maintenance (e.g. periodic refresh) until stopCh is
	// closed.
	Run(stopCh <-chan struct{})
}

// SubnetworkCandidateRangeProvider extends CandidateRangeProvider to allow
// querying candidate secondary ranges for a specific subnetwork.
type SubnetworkCandidateRangeProvider interface {
	CandidateRangeProvider
	// GetCandidateRangesForSubnetwork returns candidate secondary range names
	// for pod IP allocation in a specific subnetwork.
	GetCandidateRangesForSubnetwork(subnetwork string) []string
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

// GetCandidateRanges returns active candidate secondary ranges for pod IP
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

// GetCandidateRangesForSubnetwork returns active candidate secondary ranges
// for a specific subnetwork. If subnetwork is empty or matches the default
// subnetwork, it returns candidates for the default subnetwork.
func (s *StaticRangeProvider) GetCandidateRangesForSubnetwork(subnetwork string) []string {
	canonical := canonicalSubnetwork(subnetwork)
	if canonical == "" || (s.defaultSubnet != "" && canonical == s.defaultSubnet) {
		ranges, _ := s.GetCandidateRanges(context.Background())
		return ranges
	}
	if ranges, ok := s.activeBySubnet[canonical]; ok {
		return append([]string{}, ranges...)
	}
	// Fallback: if defaultSubnet was unspecified (empty), unqualified ranges
	// apply as default ranges across interfaces:
	if s.defaultSubnet == "" {
		if ranges, ok := s.activeBySubnet[""]; ok {
			return append([]string{}, ranges...)
		}
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
