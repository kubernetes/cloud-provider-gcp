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

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// TestNewStore_SuccessAndClose verifies that a new Store can be created,
// the schema is successfully initialized, and the database closes cleanly.
func TestNewStore_SuccessAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ipam.db")
	logger := logr.Discard()

	store, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("Expected a valid Store instance, got nil")
	}

	// Verify tables were actually created.
	var tableName string
	err = store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='cidr_blocks';").Scan(&tableName)
	if err != nil || tableName != "cidr_blocks" {
		t.Errorf("Expected table 'cidr_blocks' to exist, got error: %v", err)
	}

	// Verify the database connection is alive.
	if err := store.db.Ping(); err != nil {
		t.Errorf("Database ping failed: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Store.Close() failed: %v", err)
	}
}

// TestNewStore_Idempotency verifies idempotency by ensuring a second
// initialization safely skips schema creation. By intentionally dropping an
// index after the first run, the test deterministically proves that the second
// execution short-circuits and leaves the index uncreated.
func TestNewStore_Idempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "idempotency_test.db")
	logger := logr.Discard()

	// Initial creation sets up the full schema and user_version = 1.
	store1, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("First NewStore call failed: %v", err)
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("Failed to close first store: %v", err)
	}

	// Sabotage the schema slightly to prove the bypass works. Manually connect
	// and drop an index that initSchema would normally create.
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB for manual intervention: %v", err)
	}
	if _, err := db.Exec("DROP INDEX idx_ip_idempotency;"); err != nil {
		t.Fatalf("Failed to manually drop index: %v", err)
	}
	db.Close()

	// If the short-circuit works, it will see user_version=1 and return early,
	// meaning it will NOT execute the CREATE statements to fix the missing index.
	store2, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Second NewStore call failed: %v", err)
	}
	defer store2.Close()

	// Verify the index is still missing.
	var name string
	query := "SELECT name FROM sqlite_master WHERE type='index' AND name='idx_ip_idempotency';"
	err = store2.db.QueryRow(query).Scan(&name)

	if err == nil {
		// If err is nil, it means the query found the index, which means the
		// schema block executed and recreated it.
		t.Errorf("Expected index 'idx_ip_idempotency' to be missing, but it was recreated. Short-circuit failed.")
	} else if err != sql.ErrNoRows {
		// If an error other than ErrNoRows occurs, the query failed unexpectedly.
		t.Fatalf("Unexpected error querying sqlite_master: %v", err)
	}
}

// TestNewStore_SchemaVerification rigorously checks the sqlite_master table
// to ensure all expected tables, indexes, and triggers were successfully
// created during initialization.
func TestNewStore_SchemaVerification(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "schema_test.db")
	logger := logr.Discard()

	store, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Define the expected schema components.
	expectedTables := []string{"cidr_blocks", "ip_addresses"}
	expectedIndexes := []string{"idx_available_ips", "idx_ip_idempotency"}
	expectedTriggers := []string{"update_cidr_blocks_updated_at", "update_ip_addresses_updated_at"}

	// Verify Tables.
	for _, table := range expectedTables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?;"
		if err := store.db.QueryRow(query, table).Scan(&name); err != nil {
			t.Errorf("Schema verification failed: expected table '%s' not found: %v", table, err)
		}
	}

	// Verify Indexes.
	for _, index := range expectedIndexes {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='index' AND name=?;"
		if err := store.db.QueryRow(query, index).Scan(&name); err != nil {
			t.Errorf("Schema verification failed: expected index '%s' not found: %v", index, err)
		}
	}

	// Verify Triggers.
	for _, trigger := range expectedTriggers {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='trigger' AND name=?;"
		if err := store.db.QueryRow(query, trigger).Scan(&name); err != nil {
			t.Errorf("Schema verification failed: expected trigger '%s' not found: %v", trigger, err)
		}
	}
}

// TestStore_Concurrency verifies that the SQLite connection pool and
// transaction locks (_txlock=immediate, MaxOpenConns=10) can successfully
// handle high bursts of concurrent requests without throwing SQLITE_BUSY
// errors. It achieves maximum contention by spawning multiple goroutines and
// using a broadcast channel as a "starting gun" to release them at the exact
// same logical moment.
func TestStore_Concurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ipam-concurrency.db")
	s, err := NewStore(context.Background(), logr.Discard(), dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer s.Close()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Create the Starting Line channel.
	startLine := make(chan struct{})

	// Simulate 10 concurrent gRPC requests hitting the database.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Block this goroutine until the starting gun fires.
			<-startLine

			// Simulate an Insert.
			cidr := fmt.Sprintf("10.0.%d.0/24", id)
			insertQuery := `INSERT INTO cidr_blocks (cidr, network, ip_family, total_ips, allocated_ips, state) 
							VALUES (?, 'test-network', 'ipv4', 256, 0, 'Ready')`

			_, err := s.db.Exec(insertQuery, cidr)
			if err != nil {
				t.Errorf("Goroutine %d failed to insert: %v", id, err)
				return
			}

			// Simulate a Read.
			var state string
			readQuery := `SELECT state FROM cidr_blocks WHERE cidr = ?`
			err = s.db.QueryRow(readQuery, cidr).Scan(&state)
			if err != nil {
				t.Errorf("Goroutine %d failed to read: %v", id, err)
				return
			}
		}(i)
	}

	// Fire the starting gun!
	// Closing the channel instantly releases all 10 blocked goroutines at the exact same time.
	close(startLine)

	// Wait for all goroutines to finish.
	wg.Wait()
}

// TestStore_MaxOpenConns_Limit verifies that the connection pool can fan out
// to the configured maxOpenConns limit. It proves this by holding
// (maxOpenConns - 1) read connections hostage and ensuring the final allowed
// concurrent query can still execute successfully without hitting a pool bottleneck.
func TestStore_MaxOpenConns_Limit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ipam-pool-limit.db")
	s, err := NewStore(context.Background(), logr.Discard(), dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer s.Close()

	// Dynamically scale the test based on the Store's configuration
	hostageCount := s.db.Stats().MaxOpenConnections - 1
	connAcquired := make(chan struct{}, hostageCount)
	releaseHostages := make(chan struct{})
	var wg sync.WaitGroup

	// 1. Take (maxOpenConns - 1) connections hostage using unclosed read queries
	for i := range hostageCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			rows, err := s.db.Query(`SELECT state FROM cidr_blocks`)
			if err != nil {
				t.Errorf("Goroutine %d failed to execute read query: %v", id, err)
				return
			}
			defer rows.Close()

			connAcquired <- struct{}{}
			<-releaseHostages
		}(i)
	}

	// Wait for all hostage connections to be checked out of the pool.
	// We use a 1-second timeout to prevent the test from hanging indefinitely
	// if the connection pool is configured smaller than hostageCount.
	timeout := time.After(1 * time.Second)
	for i := 0; i < hostageCount; i++ {
		select {
		case <-connAcquired:
			// Connection successfully acquired
		case <-timeout:
			t.Fatalf("Test timed out waiting to acquire %d connections. MaxOpenConns is likely configured lower than the expected limit.", hostageCount)
		}
	}

	// 2. Execute the final allowed query to reach maxOpenConns
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var state string
	err = s.db.QueryRowContext(ctx, `SELECT state FROM cidr_blocks LIMIT 1`).Scan(&state)

	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Final concurrent query failed (pool limit reached prematurely): %v", err)
	}

	// 3. Clean up
	close(releaseHostages)
	wg.Wait()
}

func TestStore_AddCIDR(t *testing.T) {
	logger := logr.Discard() // Use discard logger to avoid klog dependency in tests

	// Use testing.T.TempDir() which is standard in modern Go and cleans up automatically!
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "metis.sqlite")
	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore returned unexpected error: %v", err)
	}
	defer s.Close()

	network := "gke-pod-network-addcidr"
	cidr := "10.0.1.0/29" // 8 IPs: 10.0.1.0 to 10.0.1.7

	err = s.AddCIDR(context.Background(), network, cidr)
	if err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	// 1. Verify cidr_block table insertion
	var totalIPs, allocatedIPs int
	var state string
	err = s.db.QueryRow(`SELECT total_ips, allocated_ips, state FROM cidr_blocks WHERE cidr = ?`, cidr).Scan(&totalIPs, &allocatedIPs, &state)
	if err != nil {
		t.Fatalf("Failed to query inserted cidr_block: %v", err)
	}

	if totalIPs != 8 {
		t.Errorf("Expected total_ips 8, got %d", totalIPs)
	}
	if allocatedIPs != 3 {
		t.Errorf("Expected allocated_ips 3 (first two and last one reserved), got %d", allocatedIPs)
	}
	if state != "Ready" {
		t.Errorf("Expected state Ready, got %s", state)
	}

	// 2. Verify ip_addresses table insertion and allocations
	rows, err := s.db.Query(`SELECT address, is_allocated FROM ip_addresses WHERE cidr_block_id = (SELECT id FROM cidr_blocks WHERE cidr = ?) ORDER BY address`, cidr)
	if err != nil {
		t.Fatalf("Failed to query inserted ip_addresses: %v", err)
	}
	defer rows.Close()

	var addresses []string
	var allocations []bool
	for rows.Next() {
		var addr string
		var isAlloc bool
		if err := rows.Scan(&addr, &isAlloc); err != nil {
			t.Fatalf("Failed to scan ip_address: %v", err)
		}
		addresses = append(addresses, addr)
		allocations = append(allocations, isAlloc)
	}

	expectedAddrs := []string{
		"10.0.1.0", "10.0.1.1", "10.0.1.2", "10.0.1.3",
		"10.0.1.4", "10.0.1.5", "10.0.1.6", "10.0.1.7",
	}
	expectedAllocs := []bool{
		true, true, false, false,
		false, false, false, true,
	}

	if len(addresses) != len(expectedAddrs) {
		t.Fatalf("Expected %d addresses, got %d", len(expectedAddrs), len(addresses))
	}

	for i := range expectedAddrs {
		if addresses[i] != expectedAddrs[i] {
			t.Errorf("[%d] Expected address %s, got %s", i, expectedAddrs[i], addresses[i])
		}
		if allocations[i] != expectedAllocs[i] {
			t.Errorf("[%d] Expected allocation %v for address %s, got %v", i, expectedAllocs[i], addresses[i], allocations[i])
		}
	}
}

func TestStore_AllocateIPv4_SingleCIDR_EdgeCases(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "metis_allocate_network_single.sqlite")
	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore returned unexpected error: %v", err)
	}
	defer s.Close()

	network := "gke-pod-network-allocate"
	cidr := "10.0.2.0/29" // 8 IPs: .0 to .7. Reserved: .0, .1, .7. Available: .2, .3, .4, .5, .6.

	// Test Case 1: Error - No CIDR blocks found (DB empty)
	_, _, err = s.AllocateIPv4(context.Background(), network, "eth0", "container-1")
	if err == nil {
		t.Error("Expected error for no CIDR blocks, got nil")
	} else if !errors.Is(err, ErrNoAvailableIPs) {
		t.Errorf("Expected ErrNoAvailableIPs, got %v", err)
	}

	// Add the CIDR
	if err := s.AddCIDR(context.Background(), network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	// Test Case 2: Happy path - First allocation
	ip1, cidrRange1, err := s.AllocateIPv4(context.Background(), network, "eth0", "container-1")
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}
	if ip1 != "10.0.2.2" {
		t.Errorf("Expected IP 10.0.2.2, got %s", ip1)
	}
	if cidrRange1 != cidr {
		t.Errorf("Expected CIDR range %s, got %s", cidr, cidrRange1)
	}

	// Verify DB state for first allocation
	var isAlloc bool
	var containerID, interfaceName string
	err = s.db.QueryRow(`SELECT is_allocated, container_id, interface_name FROM ip_addresses WHERE address = '10.0.2.2'`).Scan(&isAlloc, &containerID, &interfaceName)
	if err != nil {
		t.Fatalf("Failed to query DB for IP status: %v", err)
	}
	if !isAlloc {
		t.Error("Expected IP 10.0.2.2 to be marked as allocated")
	}
	if containerID != "container-1" || interfaceName != "eth0" {
		t.Errorf("Expected container-1/eth0, got %s/%s", containerID, interfaceName)
	}

	// Test Case 3: Happy path - Second allocation
	ip2, _, err := s.AllocateIPv4(context.Background(), network, "eth0", "container-2")
	if err != nil {
		t.Fatalf("Second allocation failed: %v", err)
	}
	if ip2 != "10.0.2.3" {
		t.Errorf("Expected IP 10.0.2.3, got %s", ip2)
	}

	// Test Case 4: Exhaust allocation
	// We had 5 available IPs: .2, .3, .4, .5, .6.
	// We already allocated .2 and .3.
	// Let's allocate .4, .5, .6.
	for i := 4; i <= 6; i++ {
		expectedIP := fmt.Sprintf("10.0.2.%d", i)
		ip, _, err := s.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
		if err != nil {
			t.Fatalf("Allocation failed for %s: %v", expectedIP, err)
		}
		if ip != expectedIP {
			t.Errorf("Expected IP %s, got %s", expectedIP, ip)
		}
	}

	// Now it should be exhausted. Next allocation should fail.
	ipEx, _, err := s.AllocateIPv4(context.Background(), network, "eth0", "container-exhaust")
	if err == nil {
		t.Errorf("Expected error for exhausted CIDR, got nil. Returned IP: %s", ipEx)
	} else if !errors.Is(err, ErrNoAvailableIPs) {
		t.Errorf("Expected ErrNoAvailableIPs, got %v", err)
	}

	// Test Case 5: Error - No available IP address found (desync)
	newNetwork := "gke-pod-network-2"
	newCIDR := "10.0.3.0/29"
	if err := s.AddCIDR(context.Background(), newNetwork, newCIDR); err != nil {
		t.Fatalf("Failed to add new CIDR: %v", err)
	}

	var newCidrBlockID int64
	err = s.db.QueryRow("SELECT id FROM cidr_blocks WHERE cidr = ?", newCIDR).Scan(&newCidrBlockID)
	if err != nil {
		t.Fatalf("Failed to get cidr_block_id: %v", err)
	}

	// Manually mark all IPs as allocated in `ip_addresses` to simulate desync
	_, err = s.db.Exec(`UPDATE ip_addresses SET is_allocated = TRUE WHERE cidr_block_id = ?`, newCidrBlockID)
	if err != nil {
		t.Fatalf("Failed to manually corrupt DB: %v", err)
	}

	ipDesync, _, err := s.AllocateIPv4(context.Background(), newNetwork, "eth0", "container-desync")
	if err == nil {
		t.Errorf("Expected error due to IP address desync, got nil. Returned IP: %s", ipDesync)
	} else if !errors.Is(err, ErrNoAvailableIPs) {
		t.Errorf("Expected ErrNoAvailableIPs, got %v", err)
	}
}

func TestStore_ReleaseIPByOwner(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_release_test.sqlite")

	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	network := "test-network"
	cidr := "10.0.1.0/24"

	if err := s.AddCIDR(context.Background(), network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	var cidrBlockID int64
	err = s.db.QueryRow("SELECT id FROM cidr_blocks WHERE cidr = ?", cidr).Scan(&cidrBlockID)
	if err != nil {
		t.Fatalf("Failed to query cidr_block_id: %v", err)
	}

	containerID := "test-container"
	interfaceName := "eth0"

	ip, _, err := s.AllocateIPv4(context.Background(), network, interfaceName, containerID)
	if err != nil {
		t.Fatalf("AllocateIPv4 failed: %v", err)
	}

	var allocatedIPs int
	err = s.db.QueryRow("SELECT allocated_ips FROM cidr_blocks WHERE id = ?", cidrBlockID).Scan(&allocatedIPs)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if allocatedIPs != 4 { // 3 reserved + 1 allocated
		t.Errorf("Expected 4 allocated IPs, got %d", allocatedIPs)
	}

	cooloff := 1 * time.Minute
	count, err := s.ReleaseIPByOwner(context.Background(), network, containerID, interfaceName, cooloff)
	if err != nil {
		t.Fatalf("ReleaseIPByOwner failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 IP to be released, got %d", count)
	}

	err = s.db.QueryRow("SELECT allocated_ips FROM cidr_blocks WHERE id = ?", cidrBlockID).Scan(&allocatedIPs)
	if err != nil {
		t.Fatalf("QueryRow failed after release: %v", err)
	}
	if allocatedIPs != 3 { // Back to 3!
		t.Errorf("Expected 3 allocated IPs after release, got %d", allocatedIPs)
	}

	var isAlloc bool
	var releaseAt sql.NullTime
	err = s.db.QueryRow("SELECT is_allocated, release_at FROM ip_addresses WHERE address = ?", ip).Scan(&isAlloc, &releaseAt)
	if err != nil {
		t.Fatalf("QueryRow failed for IP status: %v", err)
	}
	if isAlloc {
		t.Error("Expected IP to be unallocated")
	}
	if !releaseAt.Valid {
		t.Error("Expected release_at to be valid")
	}
}

func TestStore_AllocateIPv4_FallbackAndCooldown(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fallback_test.sqlite")

	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	network := "test-network"
	cidr1 := "10.0.1.0/29" // 5 available addresses (.2 to .6)

	if err := s.AddCIDR(context.Background(), network, cidr1); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	// 1. Allocate 5 IPs to exhaust the first CIDR
	for i := 1; i <= 5; i++ {
		_, _, err := s.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate container-%d: %v", i, err)
		}
	}

	// 2. Attempting to allocate another should FAIL because the first CIDR is full
	_, _, err = s.AllocateIPv4(context.Background(), network, "eth0", "container-6")
	if err == nil {
		t.Error("Expected AllocateIPv4 to fail when first CIDR is full, got nil")
	}

	// 3. Add a second CIDR block
	cidr2 := "10.0.2.0/29"
	if err := s.AddCIDR(context.Background(), network, cidr2); err != nil {
		t.Fatalf("Failed to add second CIDR block: %v", err)
	}

	// 4. Try allocating again, it should succeed by falling back to the second CIDR
	ip, cidr, err := s.AllocateIPv4(context.Background(), network, "eth0", "container-7")
	if err != nil {
		t.Fatalf("AllocateIPv4 failed after adding second CIDR: %v", err)
	}

	if ip != "10.0.2.2" { // First available in second CIDR
		t.Errorf("Expected IP 10.0.2.2 from second CIDR, got %s", ip)
	}
	if cidr != cidr2 {
		t.Errorf("Expected CIDR %s, got %s", cidr2, cidr)
	}

	// 5. Release one IP with cooldown
	_, err = s.ReleaseIPByOwner(context.Background(), network, "container-1", "eth0", 1*time.Hour)
	if err != nil {
		t.Fatalf("ReleaseIPByOwner failed: %v", err)
	}

	// 6. Try to re-allocate for a NEW container. It should NOT pick the released IP (since it's in cooldown).
	// It should pick the next available in the second CIDR (since first CIDR is full except for the cooled-down one).
	ipNew, _, err := s.AllocateIPv4(context.Background(), network, "eth0", "container-new")
	if err != nil {
		t.Fatalf("AllocateIPv4 failed after release with cooldown: %v", err)
	}

	if ipNew == "10.0.1.2" {
		t.Errorf("Expected different IP from 10.0.1.2 which should be in release cooldown")
	}
}

func TestStore_AllocateIPv4_Idempotency_Concurrency(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "idempotency_concurrency_test.sqlite")

	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	network := "test-network"
	cidr := "10.0.1.0/24"

	if err := s.AddCIDR(context.Background(), network, cidr); err != nil {
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
			addr, _, err := s.AllocateIPv4(context.Background(), network, interfaceName, containerID)
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
	err = s.DB().QueryRow("SELECT COUNT(*) FROM ip_addresses WHERE container_id = ? AND interface_name = ?", containerID, interfaceName).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query DB for count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 row for container %s, got %d", containerID, count)
	}
}

func TestStore_AllocateIPv4_Concurrency_DifferentContainers(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrency_diff_containers.sqlite")

	s, err := NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	network := "test-network"
	cidr := "10.0.1.0/24" // 253 available IPs

	if err := s.AddCIDR(context.Background(), network, cidr); err != nil {
		t.Fatalf("AddCIDR failed: %v", err)
	}

	const numGoroutines = 50 // High contention
	var wg sync.WaitGroup
	ips := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addr, _, err := s.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", idx))
			ips[idx] = addr
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all succeeded and IPs are unique
	uniqueIPs := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		if errs[i] != nil {
			t.Errorf("Goroutine %d failed: %v", i, errs[i])
		}
		if ips[i] == "" {
			t.Errorf("Goroutine %d returned empty IP", i)
		} else {
			if uniqueIPs[ips[i]] {
				t.Errorf("Duplicate IP allocated: %s", ips[i])
			}
			uniqueIPs[ips[i]] = true
		}
	}

	if len(uniqueIPs) != numGoroutines {
		t.Errorf("Expected %d unique IPs, got %d", numGoroutines, len(uniqueIPs))
	}

	// Verify DB stats
	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM ip_addresses WHERE is_allocated = TRUE AND container_id LIKE 'container-%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query DB for count: %v", err)
	}
	if count != numGoroutines {
		t.Errorf("Expected %d allocated rows in DB, got %d", numGoroutines, count)
	}
}
