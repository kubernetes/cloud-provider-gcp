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

package ipam

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	_ "github.com/ncruces/go-sqlite3/driver" // SQLite driver
)

// TestNewStore_SuccessAndClose verifies that a new Store can be created,
// the schema is successfully initialized, and the database closes cleanly.
func TestNewStore_SuccessAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ipam.db")
	logger := logr.Discard()

	store, err := NewStore(logger, dbPath)
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
	store1, err := NewStore(logger, dbPath)
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
	store2, err := NewStore(logger, dbPath)
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

	store, err := NewStore(logger, dbPath)
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
	s, err := NewStore(logr.Discard(), dbPath)
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
	s, err := NewStore(logr.Discard(), dbPath)
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
