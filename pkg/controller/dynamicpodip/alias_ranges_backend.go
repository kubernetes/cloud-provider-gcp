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

	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

// aliasRangesBackend implements PodIPBackend using GCE VM instance alias IP
// ranges.
type aliasRangesBackend struct {
	gceCloud *gce.Cloud
}

// NewAliasRangesBackend constructs a PodIPBackend using standard GCE Alias IP
// ranges.
func NewAliasRangesBackend(gceCloud *gce.Cloud) PodIPBackend {
	return &aliasRangesBackend{gceCloud: gceCloud}
}

func (b *aliasRangesBackend) GetNetworkInterfaces(ctx context.Context, providerID string) ([]*networkInterface, error) {
	gceIfaces, err := b.gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
	if err != nil {
		return nil, err
	}
	return toNetworkInterfaces(gceIfaces), nil
}

func (b *aliasRangesBackend) Mutate(ctx context.Context, providerID, networkURL string, additions []string, removals []string, candidateRanges []string) error {
	return b.gceCloud.UpdateInstanceAliasIPRanges(ctx, providerID, networkURL, additions, removals, candidateRanges)
}

func (b *aliasRangesBackend) CalculateAdditions(network string, neededIPs int, currentIfaces []*networkInterface) []string {
	blocksNeeded := (neededIPs + DefaultCapacity - 1) / DefaultCapacity
	var additions []string
	for i := 0; i < blocksNeeded; i++ {
		additions = append(additions, DefaultBlockSize)
	}
	return additions
}
