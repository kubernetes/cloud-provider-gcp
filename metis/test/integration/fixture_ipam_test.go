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

package integration

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	pb "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/store"
)

// allocatePod calls the metis daemon's gRPC AllocatePodIP RPC as a CNI plugin
// would, passing initialPodCIDR on every call to match the host-local CNI's
// behaviour. Fails the test if allocation errors.
func (f *fixture) allocatePod(id string) (ip, cidr string) {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	resp, err := f.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: initialPodCIDR,
		},
		PodName:      id,
		PodNamespace: "default",
	})
	if err != nil {
		f.t.Fatalf("AllocatePodIP(%s) failed: %v", id, err)
	}
	return resp.GetIpv4().GetIpAddress(), resp.GetIpv4().GetCidr()
}

// tryAllocatePod calls AllocatePodIP with a short timeout and returns the error
// rather than failing the test. Used to assert that unready states block allocations.
func (f *fixture) tryAllocatePod(id string) (cidr string, err error) {
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	resp, err := f.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: &pb.IPConfig{InterfaceName: "eth0", ContainerId: id, InitialPodCidr: initialPodCIDR},
		PodName:    id,
	})
	if err != nil {
		return "", err
	}
	return resp.GetIpv4().GetCidr(), nil
}

// deletePod calls the metis daemon's DeallocatePodIP RPC.
func (f *fixture) deletePod(id string) {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, allocateTimeout)
	defer cancel()
	if _, err := f.ipam.DeallocatePodIP(ctx, &pb.DeallocatePodIPRequest{InterfaceName: "eth0", ContainerId: id, PodName: id}); err != nil {
		f.t.Fatalf("DeallocatePodIP(%s) failed: %v", id, err)
	}
}

// blockState queries the daemon's SQLite database directly for the state of a
// CIDR block, returning an empty state if not found.
func (f *fixture) blockState(cidr string) store.CidrBlockState {
	f.t.Helper()
	var state string
	row := f.store.DB().QueryRowContext(f.ctx, "SELECT state FROM cidr_blocks WHERE cidr = ?", cidr)
	if err := row.Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		f.t.Fatalf("failed to query block state for cidr %s: %v", cidr, err)
	}
	return store.CidrBlockState(state)
}

// blockExists checks whether a CIDR block is present in the daemon's SQLite store.
func (f *fixture) blockExists(cidr string) bool {
	f.t.Helper()
	var exists bool
	row := f.store.DB().QueryRowContext(f.ctx, "SELECT EXISTS(SELECT 1 FROM cidr_blocks WHERE cidr = ?)", cidr)
	if err := row.Scan(&exists); err != nil {
		f.t.Fatalf("failed to query block existence for cidr %s: %v", cidr, err)
	}
	return exists
}

// blockAllocatedIPs queries SQLite for the allocated_ips count of a CIDR block.
func (f *fixture) blockAllocatedIPs(cidr string) int {
	f.t.Helper()
	var allocated int
	row := f.store.DB().QueryRowContext(f.ctx, "SELECT allocated_ips FROM cidr_blocks WHERE cidr = ?", cidr)
	if err := row.Scan(&allocated); err != nil {
		f.t.Fatalf("failed to query allocated_ips for cidr %s: %v", cidr, err)
	}
	return allocated
}

// usage returns the current IP usage for the default network from the daemon's SQLite store.
func (f *fixture) usage() store.NetworkIPUsage {
	f.t.Helper()
	u, err := f.store.GetIPUsage(f.ctx, network, store.IPv4)
	if err != nil {
		f.t.Fatalf("GetIPUsage failed: %v", err)
	}
	return u
}

// equalSets checks if two string slices contain the same elements, regardless of order.
func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := slices.Clone(a)
	bCopy := slices.Clone(b)
	slices.Sort(aCopy)
	slices.Sort(bCopy)
	return slices.Equal(aCopy, bCopy)
}
