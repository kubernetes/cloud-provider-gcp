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

// allocatePod calls the metis daemon's gRPC AllocatePodIP RPC for this node,
// adapting the request to IPv4, DualStack, or IPv6Only based on the fixture configuration.
func (n *nodeHandle) allocatePod(id string) (ip, cidr string) {
	n.fixture.t.Helper()
	ctx, cancel := context.WithTimeout(n.fixture.ctx, allocateTimeout)
	defer cancel()

	var v4Config *pb.IPConfig
	var v6Config *pb.IPConfig

	if n.fixture.cfg.stackType != StackTypeIPv6Only {
		v4Config = &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: n.fixture.cfg.initialIPv4CIDR,
		}
	}
	if n.fixture.cfg.stackType == StackTypeDualStack || n.fixture.cfg.stackType == StackTypeIPv6Only {
		v6Config = &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: n.fixture.cfg.initialIPv6CIDR,
		}
	}

	resp, err := n.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config:   v4Config,
		Ipv6Config:   v6Config,
		PodName:      id,
		PodNamespace: "default",
	})
	if err != nil {
		n.fixture.t.Fatalf("AllocatePodIP(%s) on node %s failed: %v", id, n.name, err)
	}

	if n.fixture.cfg.stackType == StackTypeIPv6Only {
		return resp.GetIpv6().GetIpAddress(), resp.GetIpv6().GetCidr()
	}
	return resp.GetIpv4().GetIpAddress(), resp.GetIpv4().GetCidr()
}

// allocatePod calls the metis daemon's gRPC AllocatePodIP RPC for the primary node.
func (f *fixture) allocatePod(id string) (ip, cidr string) {
	return f.nodes[nodeName].allocatePod(id)
}

// allocateDualStackPod calls AllocatePodIP explicitly requesting both IPv4 and IPv6 addresses.
func (n *nodeHandle) allocateDualStackPod(id string) (ipv4, cidrv4, ipv6, cidrv6 string) {
	n.fixture.t.Helper()
	ctx, cancel := context.WithTimeout(n.fixture.ctx, allocateTimeout)
	defer cancel()

	resp, err := n.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: n.fixture.cfg.initialIPv4CIDR,
		},
		Ipv6Config: &pb.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    id,
			InitialPodCidr: n.fixture.cfg.initialIPv6CIDR,
		},
		PodName:      id,
		PodNamespace: "default",
	})
	if err != nil {
		n.fixture.t.Fatalf("AllocatePodIP(%s) dual-stack on node %s failed: %v", id, n.name, err)
	}
	return resp.GetIpv4().GetIpAddress(), resp.GetIpv4().GetCidr(),
		resp.GetIpv6().GetIpAddress(), resp.GetIpv6().GetCidr()
}

// allocateDualStackPod calls AllocatePodIP explicitly requesting both IPv4 and IPv6 addresses on primary node.
func (f *fixture) allocateDualStackPod(id string) (ipv4, cidrv4, ipv6, cidrv6 string) {
	return f.nodes[nodeName].allocateDualStackPod(id)
}

// tryAllocatePod calls AllocatePodIP with a short timeout and returns the error
// rather than failing the test. Used to assert that unready states block allocations.
func (n *nodeHandle) tryAllocatePod(id string) (cidr string, err error) {
	ctx, cancel := context.WithTimeout(n.fixture.ctx, allocateTimeout)
	defer cancel()
	var v4Config *pb.IPConfig
	var v6Config *pb.IPConfig
	if n.fixture.cfg.stackType != StackTypeIPv6Only {
		v4Config = &pb.IPConfig{InterfaceName: "eth0", ContainerId: id, InitialPodCidr: n.fixture.cfg.initialIPv4CIDR}
	}
	if n.fixture.cfg.stackType == StackTypeDualStack || n.fixture.cfg.stackType == StackTypeIPv6Only {
		v6Config = &pb.IPConfig{InterfaceName: "eth0", ContainerId: id, InitialPodCidr: n.fixture.cfg.initialIPv6CIDR}
	}
	resp, err := n.ipam.AllocatePodIP(ctx, &pb.AllocatePodIPRequest{
		Ipv4Config: v4Config,
		Ipv6Config: v6Config,
		PodName:    id,
	})
	if err != nil {
		return "", err
	}
	if n.fixture.cfg.stackType == StackTypeIPv6Only {
		return resp.GetIpv6().GetCidr(), nil
	}
	return resp.GetIpv4().GetCidr(), nil
}

// tryAllocatePod calls AllocatePodIP with a short timeout on the primary node.
func (f *fixture) tryAllocatePod(id string) (cidr string, err error) {
	return f.nodes[nodeName].tryAllocatePod(id)
}

// deletePod calls the metis daemon's DeallocatePodIP RPC for this node.
func (n *nodeHandle) deletePod(id string) {
	n.fixture.t.Helper()
	ctx, cancel := context.WithTimeout(n.fixture.ctx, allocateTimeout)
	defer cancel()
	if _, err := n.ipam.DeallocatePodIP(ctx, &pb.DeallocatePodIPRequest{InterfaceName: "eth0", ContainerId: id, PodName: id}); err != nil {
		n.fixture.t.Fatalf("DeallocatePodIP(%s) on node %s failed: %v", id, n.name, err)
	}
}

// deletePod calls the metis daemon's DeallocatePodIP RPC on the primary node.
func (f *fixture) deletePod(id string) {
	f.nodes[nodeName].deletePod(id)
}

// blockState queries this node's SQLite database directly for the state of a CIDR block.
func (n *nodeHandle) blockState(cidr string) store.CidrBlockState {
	n.fixture.t.Helper()
	var state string
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT state FROM cidr_blocks WHERE cidr = ?", cidr)
	if err := row.Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		n.fixture.t.Fatalf("failed to query block state for cidr %s on %s: %v", cidr, n.name, err)
	}
	return store.CidrBlockState(state)
}

// blockState queries the primary daemon's SQLite database directly for the state of a CIDR block.
func (f *fixture) blockState(cidr string) store.CidrBlockState {
	return f.nodes[nodeName].blockState(cidr)
}

// ipv6BlockState queries this node's SQLite database for an IPv6 block's state.
func (n *nodeHandle) ipv6BlockState(cidr string) store.CidrBlockState {
	n.fixture.t.Helper()
	var state string
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT state FROM cidr_blocks WHERE cidr = ? AND ip_family = 'ipv6'", cidr)
	if err := row.Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		n.fixture.t.Fatalf("failed to query block state for ipv6 cidr %s on %s: %v", cidr, n.name, err)
	}
	return store.CidrBlockState(state)
}

// ipv6BlockState queries the primary daemon's SQLite database for an IPv6 block's state.
func (f *fixture) ipv6BlockState(cidr string) store.CidrBlockState {
	return f.nodes[nodeName].ipv6BlockState(cidr)
}

// blockExists checks whether a CIDR block is present in this node's SQLite store.
func (n *nodeHandle) blockExists(cidr string) bool {
	n.fixture.t.Helper()
	var exists bool
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT EXISTS(SELECT 1 FROM cidr_blocks WHERE cidr = ?)", cidr)
	if err := row.Scan(&exists); err != nil {
		n.fixture.t.Fatalf("failed to query block existence for cidr %s on %s: %v", cidr, n.name, err)
	}
	return exists
}

// blockExists checks whether a CIDR block is present in the primary daemon's SQLite store.
func (f *fixture) blockExists(cidr string) bool {
	return f.nodes[nodeName].blockExists(cidr)
}

// blockAllocatedIPs queries SQLite for the allocated_ips count of a CIDR block on this node.
func (n *nodeHandle) blockAllocatedIPs(cidr string) int {
	n.fixture.t.Helper()
	var allocated int
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT allocated_ips FROM cidr_blocks WHERE cidr = ?", cidr)
	if err := row.Scan(&allocated); err != nil {
		n.fixture.t.Fatalf("failed to query allocated_ips for cidr %s on %s: %v", cidr, n.name, err)
	}
	return allocated
}

// blockAllocatedIPs queries SQLite for the allocated_ips count of a CIDR block on the primary node.
func (f *fixture) blockAllocatedIPs(cidr string) int {
	return f.nodes[nodeName].blockAllocatedIPs(cidr)
}

// usage returns the current IPv4 IP usage for the default network on this node.
func (n *nodeHandle) usage() store.NetworkIPUsage {
	n.fixture.t.Helper()
	u, err := n.store.GetIPUsage(n.fixture.ctx, network, store.IPv4)
	if err != nil {
		n.fixture.t.Fatalf("GetIPUsage(IPv4) failed on %s: %v", n.name, err)
	}
	return u
}

// usage returns the current IPv4 IP usage for the default network on the primary node.
func (f *fixture) usage() store.NetworkIPUsage {
	return f.nodes[nodeName].usage()
}

// ipv6Usage returns current IPv6 IP usage for the default network on this node.
func (n *nodeHandle) ipv6Usage() store.NetworkIPUsage {
	n.fixture.t.Helper()
	u, err := n.store.GetIPUsage(n.fixture.ctx, network, store.IPv6)
	if err != nil {
		n.fixture.t.Fatalf("GetIPUsage(IPv6) failed on %s: %v", n.name, err)
	}
	return u
}

// ipv6Usage returns current IPv6 IP usage for the default network on the primary node.
func (f *fixture) ipv6Usage() store.NetworkIPUsage {
	return f.nodes[nodeName].ipv6Usage()
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
