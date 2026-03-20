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
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/go-logr/logr"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Store manages database operations for IPAM.
type Store struct {
	db  *sql.DB
	log logr.Logger
}

// NewStore creates a new Store instance and initializes the database.
func NewStore(log logr.Logger, dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = "./ipam.db"
	}

	log.Info("Opening or creating database", "path", dbPath)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// A side-file is locked to achieve strict serialization during the
	// Initialization Phase. Given that both the CNI plugin(s) and the Adaptive
	// IPAM Daemon might race to initialize this SQLite database when a node
	// boots, a mutex mechanism is required. This prevents multiple processes
	// from fighting over the initial setup and avoids "database is locked"
	// errors during the critical initialization phase.
	lockPath := dbPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire Exclusive Lock (Blocks until available). This tells the OS
	// to pause any other process reaching this line until it is released.
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("failed to acquire file lock: %w", err)
	}
	// Ensure the lock is released.
	defer func() {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// SQLite is configured directly through the DSN string. This approach
	// guarantees every new connection spawned by the sql.DB pool inherits these
	// exact configurations natively.
	dsn := dbPath +
		// Enables Write-Ahead Logging (WAL) mode. This significantly improves
		// concurrency by allowing multiple readers to access the database
		// simultaneously without blocking a writer, which is critical for burst
		// requests.
		// See: https://www.sqlite.org/pragma.html#pragma_journal_mode
		"?_journal_mode=WAL" +
		// Enforces foreign key constraints. SQLite ignores these by default.
		// This is required to ensure ON DELETE CASCADE functions correctly on the
		// ip_addresses table when a draining CIDR block is officially removed.
		// See: https://www.sqlite.org/pragma.html#pragma_foreign_keys
		"&_foreign_keys=on" +
		// Sets the busy timeout to 5000 milliseconds. If the database is locked
		// by another transaction, this tells the SQLite driver to wait for up
		// to 5 seconds before giving up and returning a locked error.
		// See: https://www.sqlite.org/pragma.html#pragma_busy_timeout
		"&_busy_timeout=5000" +
		// Instructs the Go driver to send "BEGIN IMMEDIATE" instead of standard
		// "BEGIN" when starting a transaction. This grabs a write lock instantly,
		// preventing deadlocks when concurrent requests try to upgrade their
		// read locks to write locks simultaneously. Note: This is a go-sqlite3
		// driver feature, not a native SQLite PRAGMA.
		// See: https://github.com/mattn/go-sqlite3#connection-string
		"&_txlock=immediate" +
		// Maps to PRAGMA synchronous = NORMAL. In WAL mode, this is the optimal
		// setting for high-concurrency daemons. It prevents database corruption
		// during power loss or hard crashes while offering much faster write
		// performance than FULL mode, sacrificing only a few milliseconds of
		// un-checkpointed durability.
		// See: https://www.sqlite.org/pragma.html#pragma_synchronous
		"&_synchronous=1"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// Sets the maximum amount of time a connection may be reused to infinity
	// (0). This guarantees the single connection never expires.
	db.SetConnMaxLifetime(0)

	store := &Store{
		db:  db,
		log: log,
	}

	// Only a single process enters this execution block at a time.
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Info("Initialized or updated database schema", "path", dbPath)

	return store, nil
}

// initSchema creates the necessary tables if they don't exist.
func (s *Store) initSchema() error {
	// Tracks the SQLite schema version to allow safe local migrations
	// and prevent state corruption across daemon restarts.
	const expectedSchemaVersion = 1
	var currentVersion int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}
	// If the current schema version matches the expected version, the database
	// has already been fully initialized during a previous startup. Because all
	// connection-level configurations are handled natively via the DSN string
	// upon opening the connection, no further action is required.
	if currentVersion == expectedSchemaVersion {
		return nil
	}

	s.log.Info("Initializing DB schema", "currentVersion", currentVersion, "expectedVersion", expectedSchemaVersion)

	// Set User Version.
	setVersion := fmt.Sprintf("PRAGMA user_version = %d;", expectedSchemaVersion)

	// TABLES
	cidrBlocksTable := `
	CREATE TABLE IF NOT EXISTS cidr_blocks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cidr TEXT UNIQUE NOT NULL,
		network TEXT NOT NULL,
		ip_family TEXT NOT NULL,
		network_ip TEXT NOT NULL,
		broadcast_ip TEXT NOT NULL,
		total_ips INTEGER NOT NULL,
		allocated_ips INTEGER DEFAULT 0,
		state TEXT NOT NULL DEFAULT '` + string(StateReady) + `',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	ipAddressesTable := `
	CREATE TABLE IF NOT EXISTS ip_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		address TEXT UNIQUE NOT NULL,
		cidr_block_id INTEGER NOT NULL,
		container_id TEXT,
		interface_name TEXT,
		is_allocated BOOLEAN DEFAULT FALSE,
		release_at TIMESTAMP, 
		allocated_at TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (cidr_block_id) REFERENCES cidr_blocks(id) ON DELETE CASCADE
	);`

	// INDEXES
	idxIPAddressesAvailable := `
	CREATE INDEX IF NOT EXISTS idx_ip_avail 
		ON ip_addresses(cidr_block_id, id) 
		WHERE is_allocated = FALSE;`

	// Used to ensure idempotency when the CNI retries a cmdAdd/cmdDel request.
	idxIdempotencyLookup := `
	CREATE INDEX IF NOT EXISTS idx_ip_idempotency
		ON ip_addresses(container_id, interface_name);`

	// TRIGGERS
	cidrTrigger := `
	CREATE TRIGGER IF NOT EXISTS update_cidr_blocks_updated_at
		AFTER UPDATE ON cidr_blocks FOR EACH ROW BEGIN
		UPDATE cidr_blocks SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
		END;`

	ipTrigger := `
	CREATE TRIGGER IF NOT EXISTS update_ip_addresses_updated_at
		AFTER UPDATE ON ip_addresses FOR EACH ROW BEGIN
			UPDATE ip_addresses SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
		END;`

	executables := []string{
		cidrBlocksTable,
		ipAddressesTable,
		idxIPAddressesAvailable,
		idxIdempotencyLookup,
		cidrTrigger,
		ipTrigger,
		setVersion,
	}

	for _, stmt := range executables {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w\nStatement:\n%s", err, stmt)
		}
	}

	s.log.Info("Database schema initialized or updated successfully")
	return nil
}

// Close safely closes the database connection and releases any file locks.
// This should be called during the daemon's graceful shutdown sequence.
func (s *Store) Close() error {
	s.log.Info("Closing IPAM database connection")

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	return nil
}
