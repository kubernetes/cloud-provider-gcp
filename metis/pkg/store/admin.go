package store

import (
	"context"
	"fmt"
)

// IPAddress holds the metadata for an IP address.
type IPAddress struct {
	ID            int64
	Address       string
	CIDRBlockID   int64
	ContainerID   string
	PodName       string
	PodNamespace  string
	InterfaceName string
	IsAllocated   bool
}

// ListCIDRBlocks fetches all CIDR blocks.
func (s *Store) ListCIDRBlocks(ctx context.Context) ([]CIDRBlock, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, total_ips, allocated_ips, cidr, network, ip_family, state FROM cidr_blocks")
	if err != nil {
		return nil, fmt.Errorf("failed to list cidr blocks: %w", err)
	}
	defer rows.Close()

	var result []CIDRBlock
	for rows.Next() {
		var r CIDRBlock
		var ipFamily, state string
		if err := rows.Scan(&r.ID, &r.TotalIPs, &r.AllocatedIPs, &r.CIDR, &r.Network, &ipFamily, &state); err != nil {
			return nil, fmt.Errorf("failed to scan cidr block: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	return result, nil
}

// ListIPAddresses fetches all IP addresses.
func (s *Store) ListIPAddresses(ctx context.Context) ([]IPAddress, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, address, cidr_block_id, COALESCE(container_id, ''), COALESCE(pod_name, ''), COALESCE(pod_namespace, ''), COALESCE(interface_name, ''), is_allocated FROM ip_addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to list ip addresses: %w", err)
	}
	defer rows.Close()

	var result []IPAddress
	for rows.Next() {
		var r IPAddress
		if err := rows.Scan(&r.ID, &r.Address, &r.CIDRBlockID, &r.ContainerID, &r.PodName, &r.PodNamespace, &r.InterfaceName, &r.IsAllocated); err != nil {
			return nil, fmt.Errorf("failed to scan ip address: %w", err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	return result, nil
}
