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

package dal

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/metis/pkg/store"
)

// DataAccess provides a thread-safe wrapper around the IPAM store.
type DataAccess struct {
	mu    sync.Mutex
	store *store.Store
	log   logr.Logger
}

// NewDataAccess creates a new DataAccess instance.
func NewDataAccess(log logr.Logger, store *store.Store) *DataAccess {
	return &DataAccess{
		log:   log,
		store: store,
	}
}

// AllocateIPv4 checks for existing pod allocations using idx_ip_idempotency.
// If it exists, returns it directly. Otherwise, delegates to store.AllocateIPv4.
func (m *DataAccess) AllocateIPv4(network, interfaceName, containerID, podName, podNamespace string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Idempotency check using query matching idx_ip_idempotency and joining with cidr_blocks
	var address string
	var cidrRange string
	err := m.store.DB().QueryRow(`
		SELECT i.address, c.cidr 
		FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE i.container_id = ? AND i.interface_name = ? AND i.is_allocated = TRUE AND c.ip_family = 'ipv4'
		LIMIT 1
	`, containerID, interfaceName).Scan(&address, &cidrRange)

	if err == nil {
		m.log.Info("Idempotency check hit, returning existing allocation", "containerID", containerID, "interfaceName", interfaceName, "podName", podName, "podNamespace", podNamespace, "address", address, "cidr", cidrRange)
		return address, cidrRange, nil // Found it!
	}

	m.log.Info("Idempotency check miss, proceeding to new allocation", "containerID", containerID, "interfaceName", interfaceName, "podName", podName, "podNamespace", podNamespace)

	rows, err := m.store.DB().Query(`
		SELECT id FROM cidr_blocks 
		WHERE network = ? AND ip_family = 'ipv4' AND total_ips > allocated_ips AND state = 'Ready'
	`, network)
	if err != nil {
		return "", "", fmt.Errorf("failed to query available cidr blocks: %w", err)
	}
	defer rows.Close()

	var cidrBlockIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return "", "", fmt.Errorf("failed to scan cidr block id: %w", err)
		}
		cidrBlockIDs = append(cidrBlockIDs, id)
	}

	if len(cidrBlockIDs) == 0 {
		return "", "", fmt.Errorf("no available cidr blocks found for network %s", network)
	}

	// We iterate through multiple CIDR blocks because some blocks might have available IPs,
	// but they could be within release cooldown (hence unusable).
	for _, cidrBlockID := range cidrBlockIDs {
		ip, cidr, err := m.store.AllocateIPv4(cidrBlockID, interfaceName, containerID)
		if err == nil {
			m.log.Info("Successfully allocated IP address", "network", network, "ip", ip, "cidr", cidr, "podName", podName, "podNamespace", podNamespace)
			return ip, cidr, nil
		}
		m.log.Info("Failed to allocate IP from cidr block, trying next one", "cidrBlockID", cidrBlockID, "podName", podName, "podNamespace", podNamespace, "error", err)
	}

	return "", "", fmt.Errorf("failed to allocate ipv4 in any cidr block for network %s", network)
}

// AddCIDR checks if a CIDR exists and adds it if not, thread-safely.
func (m *DataAccess) AddCIDR(network, cidr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exists, err := m.store.GetCIDRBlockByCIDR(cidr)
	if err != nil {
		return err
	}
	if exists {
		m.log.Info("CIDR block already exists, skipping", "cidr", cidr)
		return nil
	}

	if err := m.store.AddCIDR(network, cidr); err != nil {
		return err
	}
	m.log.Info("Successfully added CIDR block to store", "network", network, "cidr", cidr)
	return nil
}

// ReleaseIPsByOwner delegates to store to mark IP allocations as unallocated and starts their release cooldown. It returns the count of released IPs.
func (m *DataAccess) ReleaseIPsByOwner(network, containerID, interfaceName string, releaseCooldown time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count, err := m.store.ReleaseIPByOwner(network, containerID, interfaceName, releaseCooldown)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		m.log.Info("No IP addresses found to release for owner", "network", network, "containerID", containerID, "interfaceName", interfaceName)
	} else {
		m.log.Info("Successfully released IP addresses for owner", "network", network, "containerID", containerID, "interfaceName", interfaceName, "count", count)
	}
	return count, nil
}
