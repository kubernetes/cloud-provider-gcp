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
	"database/sql"
	"errors"
	"fmt"
	"net"
	"slices"

	"github.com/containernetworking/cni/pkg/skel"
	"k8s.io/metis/pkg/store"
)

// Pod lifecycle operations in the integration test suite execute through the
// Metis CNI IPAM plugin (k8s.io/metis/pkg/cni) via CmdAddResult and
// CmdDelResult rather than bypassing CNI to talk directly to the daemon.
// Each node handle maintains an isolated CNI plugin instance configured with
// its local daemon socket and SQLite database path, exercising the complete
// CNI network configuration and IPAM allocation path end-to-end.

// cidrFromIPNet constructs a canonical CIDR notation string from an IP address
// and its network mask.
func cidrFromIPNet(ip net.IP, mask net.IPMask) string {
	return (&net.IPNet{
		IP:   ip.Mask(mask),
		Mask: mask,
	}).String()
}

// AllocatePod invokes the Metis CNI plugin's CmdAdd workflow to allocate an IP
// for a pod, returning the assigned IP address and its associated CIDR block.
func (n *NodeHandle) AllocatePod(id string) (ip, cidr string) {
	n.fixture.t.Helper()

	args := &skel.CmdArgs{
		ContainerID: id,
		Netns:       "/var/run/netns/" + id,
		IfName:      "eth0",
		Args:        fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=default", id),
		StdinData:   n.cniNetConf,
	}

	result, err := n.cniPlugin.CmdAddResult(args)
	if err != nil {
		n.fixture.t.Fatalf("CNI CmdAdd(%s) on node %s failed: %v", id, n.name, err)
	}

	if len(result.IPs) == 0 {
		n.fixture.t.Fatalf("CNI CmdAdd(%s) returned 0 IPs", id)
	}

	if n.fixture.cfg.stackType == StackTypeIPv6Only {
		for _, ipConf := range result.IPs {
			if ipConf.Address.IP.To4() == nil {
				ip = ipConf.Address.IP.String()
				cidr = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
				return ip, cidr
			}
		}
		n.fixture.t.Fatalf("CNI CmdAdd(%s) IPv6Only mode returned no IPv6 address: %v", id, result.IPs)
	}

	for _, ipConf := range result.IPs {
		if ipConf.Address.IP.To4() != nil {
			ip = ipConf.Address.IP.String()
			cidr = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
			return ip, cidr
		}
	}
	n.fixture.t.Fatalf("CNI CmdAdd(%s) returned no IPv4 address: %v", id, result.IPs)
	return "", ""
}

// AllocatePod calls the Metis CNI plugin's CmdAdd workflow on the default node.
func (f *Fixture) AllocatePod(id string) (ip, cidr string) {
	return f.DefaultNode().AllocatePod(id)
}

// AllocateDualStackPod calls CNI CmdAdd explicitly returning both IPv4 and
// IPv6 addresses.
func (n *NodeHandle) AllocateDualStackPod(id string) (ipv4, cidrv4, ipv6, cidrv6 string) {
	n.fixture.t.Helper()

	args := &skel.CmdArgs{
		ContainerID: id,
		Netns:       "/var/run/netns/" + id,
		IfName:      "eth0",
		Args:        fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=default", id),
		StdinData:   n.cniNetConf,
	}

	result, err := n.cniPlugin.CmdAddResult(args)
	if err != nil {
		n.fixture.t.Fatalf("CNI CmdAdd(%s) dual-stack on node %s failed: %v", id, n.name, err)
	}

	for _, ipConf := range result.IPs {
		if ipConf.Address.IP.To4() != nil {
			ipv4 = ipConf.Address.IP.String()
			cidrv4 = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
		} else {
			ipv6 = ipConf.Address.IP.String()
			cidrv6 = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
		}
	}
	if ipv4 == "" || ipv6 == "" {
		n.fixture.t.Fatalf("CNI CmdAdd(%s) dual-stack missing IP: v4=%s v6=%s", id, ipv4, ipv6)
	}
	return ipv4, cidrv4, ipv6, cidrv6
}

// AllocateDualStackPod calls CNI CmdAdd on the default node returning both
// IPv4 and IPv6 addresses.
func (f *Fixture) AllocateDualStackPod(id string) (ipv4, cidrv4, ipv6, cidrv6 string) {
	return f.DefaultNode().AllocateDualStackPod(id)
}

// TryAllocatePod calls the CNI plugin's CmdAdd workflow and returns the error
// rather than failing the test. Used to assert that unready states block
// allocations.
func (n *NodeHandle) TryAllocatePod(id string) (cidr string, err error) {
	args := &skel.CmdArgs{
		ContainerID: id,
		Netns:       "/var/run/netns/" + id,
		IfName:      "eth0",
		Args:        fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=default", id),
		StdinData:   n.cniNetConf,
	}

	result, err := n.cniPlugin.CmdAddResult(args)
	if err != nil {
		return "", err
	}

	if n.fixture.cfg.stackType == StackTypeIPv6Only {
		for _, ipConf := range result.IPs {
			if ipConf.Address.IP.To4() == nil {
				cidr = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
				return cidr, nil
			}
		}
		return "", fmt.Errorf("no IPv6 address returned in CNI result")
	}

	for _, ipConf := range result.IPs {
		if ipConf.Address.IP.To4() != nil {
			cidr = cidrFromIPNet(ipConf.Address.IP, ipConf.Address.Mask)
			return cidr, nil
		}
	}
	return "", fmt.Errorf("no matching IP found in CNI result")
}

// TryAllocatePod calls the CNI plugin's CmdAdd workflow on the default node.
func (f *Fixture) TryAllocatePod(id string) (cidr string, err error) {
	return f.DefaultNode().TryAllocatePod(id)
}

// DeletePod invokes the CNI plugin's CmdDel workflow to deallocate a pod's IP.
func (n *NodeHandle) DeletePod(id string) {
	n.fixture.t.Helper()

	args := &skel.CmdArgs{
		ContainerID: id,
		Netns:       "/var/run/netns/" + id,
		IfName:      "eth0",
		Args:        fmt.Sprintf("K8S_POD_NAME=%s;K8S_POD_NAMESPACE=default", id),
		StdinData:   n.cniNetConf,
	}

	if err := n.cniPlugin.CmdDelResult(args); err != nil {
		n.fixture.t.Fatalf("CNI CmdDel(%s) on node %s failed: %v", id, n.name, err)
	}
}

// DeletePod invokes the CNI plugin's CmdDel workflow on the default node.
func (f *Fixture) DeletePod(id string) {
	f.DefaultNode().DeletePod(id)
}

// BlockState queries this node's SQLite database directly for the state of a
// CIDR block.
func (n *NodeHandle) BlockState(cidr string) store.CidrBlockState {
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

// BlockState queries the default daemon's SQLite database directly for the
// state of a CIDR block.
func (f *Fixture) BlockState(cidr string) store.CidrBlockState {
	return f.DefaultNode().BlockState(cidr)
}

// IPv6BlockState queries this node's SQLite database for an IPv6 block's state.
func (n *NodeHandle) IPv6BlockState(cidr string) store.CidrBlockState {
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

// IPv6BlockState queries the default daemon's SQLite database for an IPv6
// block's state.
func (f *Fixture) IPv6BlockState(cidr string) store.CidrBlockState {
	return f.DefaultNode().IPv6BlockState(cidr)
}

// BlockExists checks whether a CIDR block is present in this node's SQLite
// store.
func (n *NodeHandle) BlockExists(cidr string) bool {
	n.fixture.t.Helper()
	var exists bool
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT EXISTS(SELECT 1 FROM cidr_blocks WHERE cidr = ?)", cidr)
	if err := row.Scan(&exists); err != nil {
		n.fixture.t.Fatalf("failed to query block existence for cidr %s on %s: %v", cidr, n.name, err)
	}
	return exists
}

// BlockExists checks whether a CIDR block is present in the default daemon's
// SQLite store.
func (f *Fixture) BlockExists(cidr string) bool {
	return f.DefaultNode().BlockExists(cidr)
}

// BlockAllocatedIPs queries SQLite for the allocated_ips count of a CIDR block
// on this node.
func (n *NodeHandle) BlockAllocatedIPs(cidr string) int {
	n.fixture.t.Helper()
	var allocated int
	row := n.store.DB().QueryRowContext(n.fixture.ctx, "SELECT allocated_ips FROM cidr_blocks WHERE cidr = ?", cidr)
	if err := row.Scan(&allocated); err != nil {
		n.fixture.t.Fatalf("failed to query allocated_ips for cidr %s on %s: %v", cidr, n.name, err)
	}
	return allocated
}

// BlockAllocatedIPs queries SQLite for the allocated_ips count of a CIDR block
// on the default node.
func (f *Fixture) BlockAllocatedIPs(cidr string) int {
	return f.DefaultNode().BlockAllocatedIPs(cidr)
}

// Usage returns the current IPv4 IP usage for the default network on this node.
func (n *NodeHandle) Usage() store.NetworkIPUsage {
	n.fixture.t.Helper()
	u, err := n.store.GetIPUsage(n.fixture.ctx, defaultNetwork, store.IPv4)
	if err != nil {
		n.fixture.t.Fatalf("GetIPUsage(IPv4) failed on %s: %v", n.name, err)
	}
	return u
}

// Usage returns the current IPv4 IP usage for the default network on the
// default node.
func (f *Fixture) Usage() store.NetworkIPUsage {
	return f.DefaultNode().Usage()
}

// IPv6Usage returns current IPv6 IP usage for the default network on this node.
func (n *NodeHandle) IPv6Usage() store.NetworkIPUsage {
	n.fixture.t.Helper()
	u, err := n.store.GetIPUsage(n.fixture.ctx, defaultNetwork, store.IPv6)
	if err != nil {
		n.fixture.t.Fatalf("GetIPUsage(IPv6) failed on %s: %v", n.name, err)
	}
	return u
}

// IPv6Usage returns current IPv6 IP usage for the default network on the
// default node.
func (f *Fixture) IPv6Usage() store.NetworkIPUsage {
	return f.DefaultNode().IPv6Usage()
}

// EqualSets checks if two string slices contain the same elements, regardless
// of order.
func EqualSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := slices.Clone(a)
	bCopy := slices.Clone(b)
	slices.Sort(aCopy)
	slices.Sort(bCopy)
	return slices.Equal(aCopy, bCopy)
}
