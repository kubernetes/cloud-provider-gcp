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

	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

// BackendType identifies the Pod IP allocation backend strategy.
// Additional modes (such as a hybrid strategy that switches from ANEs to
// alias IP ranges once a per-node threshold is reached) can be added here
// and implemented as a composite PodIPBackend.
type BackendType string

const (
	// BackendTypeAliasRanges allocates Pod IPs via GCE VM alias IP ranges.
	BackendTypeAliasRanges BackendType = "alias-ranges"
	// BackendTypeANE allocates Pod IPs using GCE Alias Network Endpoints.
	BackendTypeANE BackendType = "ane"
)

// PodIPBackend abstracts the cloud IP allocation operations (such as GCE Alias
// IP ranges, Alias Network Endpoints, or a hybrid combination of both).
type PodIPBackend interface {
	// GetNetworkInterfaces returns the network interfaces with allocated
	// Pod CIDRs.
	GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error)
	// Mutate applies additions and removals of Pod IPs on the cloud
	// provider.
	Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error
	// CalculateAdditions determines the CIDR allocations needed to satisfy
	// the demand for neededIPs given the current interface allocations.
	CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string
}

// newPodIPBackend constructs the PodIPBackend corresponding to
// opts.PodIPBackend. Future modes (such as a hybrid switchover mode) can be
// added here by composing NewAliasRangesBackend and NewANEBackend.
func newPodIPBackend(opts Options, gceCloud *gce.Cloud) (PodIPBackend, error) {
	switch opts.PodIPBackend {
	case "", BackendTypeAliasRanges:
		return NewAliasRangesBackend(gceCloud), nil
	case BackendTypeANE:
		klog.Info("Using AliasNetworkEndpoint (ANE) backend for dynamic Pod IP controller")
		return NewANEBackend(gceCloud), nil
	default:
		return nil, fmt.Errorf("unsupported dynamic pod IP backend %q (supported: %q, %q)",
			opts.PodIPBackend, BackendTypeAliasRanges, BackendTypeANE)
	}
}
