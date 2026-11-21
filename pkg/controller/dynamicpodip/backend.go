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

// AdditionDemandFunc reports additions that have become necessary since a
// Mutate call began -- typically because the node daemon raised
// spec.allocations while the backend was still creating the original batch.
//
// Each call returns only the increment not previously returned; the
// implementation tracks what it has already handed out. A nil, nil result
// means there is no new demand.
type AdditionDemandFunc func(ctx context.Context) ([]string, error)

// TopUpBackend is implemented by backends whose additions are independent
// units of work (one cloud operation each) and can therefore absorb new
// demand into an in-flight batch instead of leaving it to a follow-up
// reconcile.
//
// Without this, a spec bump that lands during a mutation waits for the whole
// batch to finish and for the status to be republished, then pays a full
// extra round of cloud operation latency on its own. With it, the extra
// additions start as soon as a concurrency slot frees up.
type TopUpBackend interface {
	PodIPBackend
	// MutateWithTopUp behaves like Mutate, additionally consulting more
	// while additions are in flight and creating whatever it returns. more
	// may be nil, in which case it is equivalent to Mutate.
	MutateWithTopUp(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string, more AdditionDemandFunc) error
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
