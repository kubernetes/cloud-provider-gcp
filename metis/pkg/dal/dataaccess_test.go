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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/metis/pkg/store"
)

func TestDataAccess_AddCIDR(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dal_test.sqlite")

	store, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	dal := NewDataAccess(logger, store)

	network := "test-network"
	cidr := "10.0.1.0/24"

	// 1. Happy path: Add CIDR for the first time
	err = dal.AddCIDR(network, cidr)
	if err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	// Verify it was added by querying the store
	exists, err := store.GetCIDRBlockByCIDR(cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed during verification: %v", err)
	}
	if !exists {
		t.Error("Expected CIDR block to exist")
	}

	// 2. Happy path: Add the same CIDR again (should be a no-op/skip)
	err = dal.AddCIDR(network, cidr)
	if err != nil {
		t.Fatalf("AddCIDR failed on duplicate attempt: %v", err)
	}

	// Verify it still exists
	exists, err = store.GetCIDRBlockByCIDR(cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed during second verification: %v", err)
	}
	if !exists {
		t.Error("Expected CIDR block to still exist after duplicate add")
	}

	// 3. Error case: Invalid CIDR string
	err = dal.AddCIDR(network, "invalid-cidr-string")
	if err == nil {
		t.Error("Expected error when adding invalid CIDR, got nil")
	}
}

func TestDataAccess_AddCIDR_Concurrency(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dal_concurrency_test.sqlite")

	store, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	dal := NewDataAccess(logger, store)

	network := "test-concurrency-network"
	cidr := "10.0.1.0/24"

	const numGoroutines = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := dal.AddCIDR(network, cidr); err != nil {
				t.Errorf("AddCIDR failed in concurrency test: %v", err)
			}
		}()
	}

	wg.Wait()

	// Verify the CIDR exists
	exists, err := store.GetCIDRBlockByCIDR(cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed during verification: %v", err)
	}
	if !exists {
		t.Error("Expected CIDR block to exist")
	}

	// Verify it was only inserted once
	var count int
	err = store.DB().QueryRow("SELECT COUNT(*) FROM cidr_blocks WHERE cidr = ?", cidr).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query DB for count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 row for CIDR %s, got %d", cidr, count)
	}
}

func TestDataAccess_AllocateIPv4(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dal_allocate_test.sqlite")

	store, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	dal := NewDataAccess(logger, store)

	network := "test-network"
	cidr := "10.0.1.0/24"

	if err := dal.AddCIDR(network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	containerID := "test-container"
	interfaceName := "eth0"

	// 1. First allocation (miss)
	addr1, cidr1, err := dal.AllocateIPv4(network, interfaceName, containerID, "test-pod", "default")
	if err != nil {
		t.Fatalf("AllocateIPv4 failed: %v", err)
	}
	if addr1 == "" {
		t.Fatal("Expected IP address, got empty string")
	}

	// 2. Second allocation (hit)
	addr2, cidr2, err := dal.AllocateIPv4(network, interfaceName, containerID, "test-pod", "default")
	if err != nil {
		t.Fatalf("Second AllocateIPv4 failed: %v", err)
	}
	if addr2 != addr1 {
		t.Errorf("Expected idempotency hit, got %s, want %s", addr2, addr1)
	}
	if cidr2 != cidr1 {
		t.Errorf("Expected same CIDR %s, want %s", cidr1, cidr2)
	}
}

func TestDataAccess_AllocateIPv4_Concurrency(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "dal_allocate_concurrency_test.sqlite")

	store, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	dal := NewDataAccess(logger, store)

	network := "test-concurrency-network"
	cidr := "10.0.1.0/24"

	if err := dal.AddCIDR(network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	containerID := "test-concurrent-container"
	interfaceName := "eth0"

	const numGoroutines = 10
	var wg sync.WaitGroup
	ips := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addr, _, err := dal.AllocateIPv4(network, interfaceName, containerID, "test-pod", "default")
			ips[idx] = addr
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	var firstIP string
	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("Goroutine %d failed: %v", i, errs[i])
		}
		if ips[i] == "" {
			t.Errorf("Goroutine %d returned empty IP", i)
		} else {
			if firstIP == "" {
				firstIP = ips[i]
			} else if ips[i] != firstIP {
				t.Errorf("Goroutine %d got different IP: %s, want %s", i, ips[i], firstIP)
			}
		}
	}

	// Double check the DB to ensure only 1 row was created
	var count int
	err = store.DB().QueryRow("SELECT COUNT(*) FROM ip_addresses WHERE container_id = ? AND interface_name = ?", containerID, interfaceName).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query DB for count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 row for container %s, got %d", containerID, count)
	}
}

func TestDataAccess_ReleaseIPsByOwner(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_release_dal_test.sqlite")

	s, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	dal := NewDataAccess(logger, s)

	network := "test-network"
	cidr := "10.0.1.0/24"

	// Pre-release check before anything is added
	initCount, err := dal.ReleaseIPsByOwner(network, "test-container-notfound", "eth0", 0)
	if err != nil {
		t.Fatalf("Initial ReleaseIPsByOwner failed: %v", err)
	}
	if initCount != 0 {
		t.Errorf("Expected 0 IPs released initially, got %d", initCount)
	}

	if err := dal.AddCIDR(network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	containerID := "test-container"
	interfaceName := "eth0"
	podName := "test-pod"
	podNamespace := "default"

	// 1. Allocate an IP address
	ip, _, err := dal.AllocateIPv4(network, interfaceName, containerID, podName, podNamespace)
	if err != nil {
		t.Fatalf("AllocateIPv4 failed: %v", err)
	}

	// 2. Release it!
	count, err := dal.ReleaseIPsByOwner(network, containerID, interfaceName, 0)
	if err != nil {
		t.Fatalf("ReleaseIPsByOwner failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 IP to be released, got %d", count)
	}

	// 3. Verify it is unallocated
	var isAlloc bool
	err = s.DB().QueryRow("SELECT is_allocated FROM ip_addresses WHERE address = ?", ip).Scan(&isAlloc)
	if err != nil {
		t.Fatalf("QueryRow failed for IP status: %v", err)
	}
	if isAlloc {
		t.Error("Expected IP to be unallocated")
	}
}

func TestDataAccess_ReleaseCooldown(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_dal_cooldown_test.sqlite")

	s, err := store.NewStore(logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	dal := NewDataAccess(logger, s)

	network := "test-network-cooldown"
	cidr := "10.0.1.0/29" // 5 available addresses (.2 to .6)

	if err := dal.AddCIDR(network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	var cidrBlockID int64
	err = s.DB().QueryRow("SELECT id FROM cidr_blocks WHERE cidr = ?", cidr).Scan(&cidrBlockID)
	if err != nil {
		t.Fatalf("Failed to query cidr_block_id: %v", err)
	}

	// 1. Allocate an IP
	ip, _, err := dal.AllocateIPv4(network, "eth0", "container-1", "pod-1", "default")
	if err != nil {
		t.Fatalf("AllocateIPv4 failed: %v", err)
	}

	// 2. Release with cooldown using dal.ReleaseIPsByOwner
	count, err := dal.ReleaseIPsByOwner(network, "container-1", "eth0", 1*time.Hour)
	if err != nil {
		t.Fatalf("ReleaseIPsByOwner failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 IP to be released, got %d", count)
	}

	// 3. Secondary allocation should NOT pick the first IP, should pick the second one (.3)
	ip2, _, err := dal.AllocateIPv4(network, "eth0", "container-2", "pod-2", "default")
	if err != nil {
		t.Fatalf("AllocateIPv4 for second container failed: %v", err)
	}

	if ip2 == ip {
		t.Errorf("Expected different IP from %s which should be in release cooldown", ip)
	}

	// 4. Allocate remaining available IPs in /29 (total 5 available pod IPs: .2 to .6)
	// We already allocated container-1 and container-2. Let's allocate container-3, container-4, container-5.
	if _, _, err = dal.AllocateIPv4(network, "eth0", "container-3", "pod-3", "default"); err != nil {
		t.Fatalf("Failed to allocate container-3: %v", err)
	}
	if _, _, err = dal.AllocateIPv4(network, "eth0", "container-4", "pod-4", "default"); err != nil {
		t.Fatalf("Failed to allocate container-4: %v", err)
	}
	if _, _, err = dal.AllocateIPv4(network, "eth0", "container-5", "pod-5", "default"); err != nil {
		t.Fatalf("Failed to allocate container-5: %v", err)
	}

	// Now release all 5 with 1-hour cooldown using dal.ReleaseIPsByOwner
	dal.ReleaseIPsByOwner(network, "container-1", "eth0", 1*time.Hour)
	dal.ReleaseIPsByOwner(network, "container-2", "eth0", 1*time.Hour)
	dal.ReleaseIPsByOwner(network, "container-3", "eth0", 1*time.Hour)
	dal.ReleaseIPsByOwner(network, "container-4", "eth0", 1*time.Hour)
	dal.ReleaseIPsByOwner(network, "container-5", "eth0", 1*time.Hour)

	// Attempting to allocate container-6 should FAIL because all IPs are in cooldown!
	if _, _, err = dal.AllocateIPv4(network, "eth0", "container-6", "pod-6", "default"); err == nil {
		t.Error("Expected AllocateIPv4 to fail when all IPs are in cooldown")
	}

	// 5. Add another CIDR block to verify fallback allocation
	newCidr := "10.0.2.0/24"
	if err := dal.AddCIDR(network, newCidr); err != nil {
		t.Fatalf("Failed to add new CIDR block: %v", err)
	}

	// Try allocating again (we'll use container-7 to ensure no idempotency hits from failed attempts)
	newIP, _, err := dal.AllocateIPv4(network, "eth0", "container-7", "pod-7", "default")
	if err != nil {
		t.Fatalf("AllocateIPv4 failed after adding new CIDR block: %v", err)
	}

	if newIP == "" {
		t.Error("Expected to receive a valid IP address from the new CIDR block")
	}
}
