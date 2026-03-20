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
	"database/sql"
	"os"
	"path/filepath"
	"syscall"
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
	expectedIndexes := []string{"idx_ip_avail", "idx_ip_idempotency"}
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

// TestNewStore_FileLocking strictly verifies the mutex mechanism.
// It manually locks the .lock file and asserts that NewStore blocks
// execution until the lock is explicitly released, simulating the
// exact race condition between the CNI plugin and the daemon.
func TestNewStore_FileLocking(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "lock_test.db")
	lockPath := dbPath + ".lock"
	logger := logr.Discard()

	// Manually acquire the OS lock first to simulate another active process.
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		t.Fatalf("Failed to create manual lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("Failed to acquire manual file lock: %v", err)
	}

	// Start NewStore in a background goroutine.
	storeChan := make(chan *Store)
	errChan := make(chan error)

	go func() {
		store, err := NewStore(logger, dbPath)
		if err != nil {
			errChan <- err
			return
		}
		storeChan <- store
	}()

	// Assert that NewStore is actively blocked by the mutex.
	// We wait 500ms. If NewStore returns a store, the lock failed.
	select {
	case <-storeChan:
		t.Fatal("NewStore returned immediately, but it should be blocked by the lock!")
	case err := <-errChan:
		t.Fatalf("NewStore failed unexpectedly while waiting for lock: %v", err)
	case <-time.After(500 * time.Millisecond):
		// Success! The goroutine is blocked exactly as we designed it to be.
	}

	// Release the manual lock (Acting as the bouncer unhooking the rope).
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("Failed to release manual lock: %v", err)
	}

	// Assert that NewStore immediately completes now that the path is clear.
	select {
	case store := <-storeChan:
		// Clean up the successfully initialized store
		store.Close()
	case err := <-errChan:
		t.Fatalf("NewStore failed after lock was released: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("NewStore remained blocked even after the lock was released!")
	}
}
