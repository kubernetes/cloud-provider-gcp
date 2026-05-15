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
	_ "embed"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/mattn/go-sqlite3" // SQLite driver
)

//go:embed schema.sql
var schemaSQL string

const (
	// dbSchemaVersion tracks the SQLite schema version to allow safe local
	// migrations and prevent state corruption across daemon restarts.
	dbSchemaVersion = 1
	maxOpenConns    = 10
	maxIdleConns    = 10
	// DefaultBusyTimeout is the default timeout for SQLite busy handler.
	DefaultBusyTimeout = 5000 * time.Millisecond
	// ipv6PopulationBatchSize is the number of IPv6 addresses to populate at once
	// when a CIDR block has no available IPs in the table.
	ipv6PopulationBatchSize = 64
)

var (
	// ErrCidrAlreadyExists is returned when a CIDR block already exists in the store.
	ErrCidrAlreadyExists = errors.New("cidr block already exists")

	// ErrNoAvailableIPs is returned when no available IPs can be found in any CIDR block.
	ErrNoAvailableIPs = errors.New("no available IPs in store")

	// ErrCidrBlockExhausted is returned when an IPv6 CIDR block cannot be expanded further.
	ErrCidrBlockExhausted = errors.New("ipv6 cidr block exhausted and cannot be expanded")
)

// IPFamily represents the IP protocol family.
type IPFamily string

const (
	IPv4 IPFamily = "ipv4"
	IPv6 IPFamily = "ipv6"
)

type CidrBlockState string

const (
	StateReady    CidrBlockState = "Ready"
	StateDraining CidrBlockState = "Draining"
	StateDeleting CidrBlockState = "Deleting"
)

// Store manages database operations for IPAM.
type Store struct {
	db  *sql.DB
	log logr.Logger
}

// NewStore creates a new Store instance and initializes the database.
func NewStore(ctx context.Context, log logr.Logger, dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath cannot be empty: an absolute path must be explicitly provided")
	}

	log.Info("Opening or creating database", "path", dbPath)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

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
		// Sets the busy timeout. If the database is locked
		// by another transaction, this tells the SQLite driver to wait for up
		// to this duration before giving up and returning a locked error.
		// See: https://www.sqlite.org/pragma.html#pragma_busy_timeout
		fmt.Sprintf("&_busy_timeout=%d", DefaultBusyTimeout.Milliseconds()) +
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

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	// Sets the maximum amount of time a connection may be reused to infinity
	// (0). This guarantees the single connection never expires.
	db.SetConnMaxLifetime(0)

	store := &Store{
		db:  db,
		log: log,
	}

	// Only a single process enters this execution block at a time.
	if err := store.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Info("Initialized or updated database schema", "path", dbPath)

	return store, nil
}

// initSchema creates the necessary tables if they don't exist.
func (s *Store) initSchema(ctx context.Context) error {
	var currentVersion int
	err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	if currentVersion == dbSchemaVersion {
		s.log.V(4).Info("Database schema already initialized", "version", currentVersion)
		return nil
	}

	s.log.Info("Initializing DB schema", "currentVersion", currentVersion, "expectedVersion", dbSchemaVersion)

	// 1. Begin an atomic transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Safe to defer; Rollback does nothing if Commit() is successful
	defer tx.Rollback()

	// 2. Execute the embedded schema.sql file
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema.sql: %w", err)
	}

	// 3. Set User Version
	setVersion := fmt.Sprintf("PRAGMA user_version = %d;", dbSchemaVersion)
	if _, err := tx.ExecContext(ctx, setVersion); err != nil {
		return fmt.Errorf("failed to set user_version: %w", err)
	}

	// 4. Commit everything atomically
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema transaction: %w", err)
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

// DB returns the underlying sql.DB connection for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// AllocateIPParams wraps the parameters for AllocateIP.
type AllocateIPParams struct {
	Network       string
	InterfaceName string
	ContainerID   string
	IPFamily      IPFamily
}

// AllocateIP finds the first available IP from Ready CIDR blocks for a given network and allocates it.
// It decides which path to take (IPv4 or IPv6) based on the IPFamily in params.
func (s *Store) AllocateIP(ctx context.Context, params AllocateIPParams) (string, string, error) {
	return s.allocateIP(ctx, params)
}

// GetCIDRBlockByCIDRAndNetwork checks if a CIDR block exists for the specific network and returns its ID.
func (s *Store) GetCIDRBlockByCIDRAndNetwork(ctx context.Context, cidr, network string) (int64, bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM cidr_blocks WHERE cidr = ? AND network = ? LIMIT 1
	`, cidr, network).Scan(&id)

	if err == nil {
		return id, true, nil
	}
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return 0, false, fmt.Errorf("failed to query cidr_blocks: %w", err)
}

// AddCIDR parses the CIDR, determines family, and inserts it + its constituent IP addresses into the store.
// For IPv4, it populates all IPs. For IPv6, it only adds the CIDR block.
func (s *Store) AddCIDR(ctx context.Context, network, cidr string) error {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("failed to parse cidr %s: %w", cidr, err)
	}

	ipFamily := IPv4
	isIPv6 := false
	if prefix.Addr().Is6() {
		ipFamily = IPv6
		isIPv6 = true
	}

	var totalIPs int64
	bits := prefix.Bits()
	totalBits := 32
	if isIPv6 {
		totalBits = 128
	}
	freeBits := totalBits - bits
	if freeBits >= 62 {
		totalIPs = 0x7fffffffffffffff // Max int64 for large IPv6 ranges
	} else {
		totalIPs = 1 << freeBits
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert into cidr_blocks
	res, err := tx.ExecContext(ctx, `
		INSERT INTO cidr_blocks (cidr, network, ip_family, total_ips, allocated_ips, state) 
		VALUES (?, ?, ?, ?, 0, 'Ready')
	`, cidr, network, ipFamily, totalIPs)

	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok {
			if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
				return fmt.Errorf("%w: %v", ErrCidrAlreadyExists, err)
			}
		}
		return fmt.Errorf("failed to insert cidr_block: %w", err)
	}

	cidrBlockID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last inserted id: %w", err)
	}

	if !isIPv6 {
		// We cannot use cidrBlockID == 1 to determine if it's the first block
		// for a network because cidrBlockID is globally auto-incrementing.
		// A new network's first block would have ID > 1. Thus, we must query
		// if other blocks already exist for this specific network.
		// Check if this is the first CIDR block for this network
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM cidr_blocks WHERE network = ?
		`, network).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check existing cidr blocks: %w", err)
		}
		isFirstBlock := (count == 1)

		// Generate the list of all IPs in this CIDR range (IPv4 only)
		var ips []string
		addr := prefix.Addr()
		for ; prefix.Contains(addr); addr = addr.Next() {
			ips = append(ips, addr.String())
		}

		if len(ips) == 0 {
			return fmt.Errorf("cidr range is empty: %s", cidr)
		}

		// Insert IP addresses and determine allocation status
		var allocatedCount int
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO ip_addresses (cidr_block_id, address, is_allocated, container_id, interface_name) 
			VALUES (?, ?, ?, '', '')
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare insert statement: %w", err)
		}
		defer stmt.Close()

		for idx, addr := range ips {
			isAllocated := false
			// For small CIDRs (smaller than /30, i.e., /31 and /32), we do not reserve
			// the first two and the last IPs. The IPs returned will still be routable
			// by the underlying infrastructure.
			// The first two IPs and the last IP are automatically marked as allocated.
			// We only do this reservation for the first CIDR block in the network.
			if isFirstBlock && len(ips) >= 4 && (idx == 0 || idx == 1 || idx == len(ips)-1) {
				isAllocated = true
				allocatedCount++
			}

			_, err = stmt.ExecContext(ctx, cidrBlockID, addr, isAllocated)
			if err != nil {
				return fmt.Errorf("failed to insert ip_address %s: %w", addr, err)
			}
		}

		// Update allocated_ips to reflect the defaults we just reserved
		_, err = tx.ExecContext(ctx, `
			UPDATE cidr_blocks SET allocated_ips = ? WHERE id = ?
		`, allocatedCount, cidrBlockID)
		if err != nil {
			return fmt.Errorf("failed to update allocated_ips: %w", err)
		}
	} else {
		// For IPv6, populate the first batch of IPs immediately.
		var ips []string
		addr := prefix.Addr()
		for i := 0; i < ipv6PopulationBatchSize; i++ {
			ips = append(ips, addr.String())
			addr = addr.Next()
		}

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO ip_addresses (cidr_block_id, address, is_allocated, container_id, interface_name) 
			VALUES (?, ?, FALSE, '', '')
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare insert statement: %w", err)
		}
		defer stmt.Close()

		for _, addr := range ips {
			_, err = stmt.ExecContext(ctx, cidrBlockID, addr)
			if err != nil {
				return fmt.Errorf("failed to insert ip_address %s: %w", addr, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("Successfully added CIDR to store", "cidr", cidr, "network", network, "isIPv6", isIPv6)
	return nil
}

// ReleaseIPByOwner updates all IP addresses matching the network, container id and interface name to be is_allocated = FALSE, and sets release_at timestamp to be now + releaseCooldown. It also decrements allocated_ips count in cidr_blocks.
func (s *Store) ReleaseIPByOwner(ctx context.Context, network, containerID, interfaceName string, releaseCooldown time.Duration) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var releaseAt interface{}
	if releaseCooldown > 0 {
		releaseAt = time.Now().Add(releaseCooldown)
	} else {
		releaseAt = nil
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT i.id, i.cidr_block_id 
		FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE c.network = ? AND i.container_id = ? AND i.interface_name = ? AND i.is_allocated = TRUE
	`, network, containerID, interfaceName)

	if err != nil {
		return 0, fmt.Errorf("failed to query matching IP owners: %w", err)
	}
	defer rows.Close()

	type release struct {
		id          int64
		cidrBlockID int64
	}
	var releases []release

	for rows.Next() {
		var r release
		if err := rows.Scan(&r.id, &r.cidrBlockID); err != nil {
			return 0, fmt.Errorf("failed to scan affected IP details: %w", err)
		}
		releases = append(releases, r)
	}

	for _, r := range releases {
		_, err = tx.ExecContext(ctx, `
			UPDATE ip_addresses 
			SET is_allocated = FALSE, release_at = ?, updated_at = CURRENT_TIMESTAMP 
			WHERE id = ?
		`, releaseAt, r.id)
		if err != nil {
			return 0, fmt.Errorf("failed to release IP %d: %w", r.id, err)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE cidr_blocks 
			SET allocated_ips = allocated_ips - 1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = ?
		`, r.cidrBlockID)
		if err != nil {
			return 0, fmt.Errorf("failed to update cidr_block %d count: %w", r.cidrBlockID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit release transaction: %w", err)
	}

	return len(releases), nil
}

// UndrainCIDRBlocks changes the state of any Draining CIDR blocks for a network back to Ready.
func (s *Store) UndrainCIDRBlocks(ctx context.Context, network string) (int64, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE cidr_blocks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE network = ? AND state = ?", StateReady, network, StateDraining)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeletingCIDRBlock holds the metadata for a CIDR block currently scheduled for deletion.
type DeletingCIDRBlock struct {
	ID       int64
	TotalIPs int
	CIDR     string
	Network  string
}

// GetDeletingCIDRBlocks fetches all CIDR blocks in Deleting state for a specific network.
func (s *Store) GetDeletingCIDRBlocks(ctx context.Context, network string) ([]DeletingCIDRBlock, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, total_ips, cidr, network FROM cidr_blocks WHERE state = ? AND network = ?", StateDeleting, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DeletingCIDRBlock
	for rows.Next() {
		var r DeletingCIDRBlock
		if err := rows.Scan(&r.ID, &r.TotalIPs, &r.CIDR, &r.Network); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// DeleteCIDRBlock deletes a specific CIDR block from the local store if it is in Deleting state.
func (s *Store) DeleteCIDRBlock(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM cidr_blocks WHERE id = ? AND state = ?", id, StateDeleting)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("cannot delete CIDR block %d: block is not in Deleting status", id)
	}
	return nil
}

// GetCooldownIPCount queries the number of IPs currently in cooldown for a network.
func (s *Store) GetCooldownIPCount(ctx context.Context, network string) (int, error) {
	var cooldownCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(i.id) FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE c.network = ? AND i.is_allocated = FALSE AND i.release_at > CURRENT_TIMESTAMP
	`, network).Scan(&cooldownCount)
	if err != nil {
		return 0, err
	}
	return cooldownCount, nil
}

// DrainCIDRBlock transitions a CIDR block to the Draining state.
func (s *Store) DrainCIDRBlock(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE cidr_blocks SET state = 'Draining', updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}

// DrainingCIDRBlock holds metadata for a CIDR block that is currently in the Draining state.
type DrainingCIDRBlock struct {
	ID       int64
	TotalIPs int
	CIDR     string
}

// FindAndMarkExpiredDrainingCIDRBlocks fetches CIDR blocks that have been Draining for longer than the specified expiration duration and marks them as Deleting atomically.
func (s *Store) FindAndMarkExpiredDrainingCIDRBlocks(ctx context.Context, network string, expiration time.Duration) ([]DrainingCIDRBlock, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	expStr := fmt.Sprintf("+%d seconds", int(expiration.Seconds()))
	rows, err := tx.QueryContext(ctx, `
		SELECT id, total_ips, cidr FROM cidr_blocks 
		WHERE network = ? AND state = ? AND allocated_ips = 0 AND datetime(updated_at, ?) <= CURRENT_TIMESTAMP
	`, network, StateDraining, expStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DrainingCIDRBlock
	for rows.Next() {
		var r DrainingCIDRBlock
		if err := rows.Scan(&r.ID, &r.TotalIPs, &r.CIDR); err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	for _, r := range result {
		_, err = tx.ExecContext(ctx, "UPDATE cidr_blocks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", StateDeleting, r.ID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

// allocateIPTx is a helper that executes the IP allocation within an existing transaction.
// It returns sql.ErrNoRows if the CIDR block is full or not found, allowing the caller to try another block.
func (s *Store) allocateIPTx(ctx context.Context, tx *sql.Tx, cidrBlockID int64, interfaceName, containerID string) (string, string, error) {
	// 1. Fetch CIDR range for the given ID and verify it is not full
	var cidrRange string
	err := tx.QueryRowContext(ctx, `
		SELECT cidr FROM cidr_blocks 
		WHERE id = ? AND total_ips > allocated_ips AND state = 'Ready'
	`, cidrBlockID).Scan(&cidrRange)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", sql.ErrNoRows
		}
		return "", "", fmt.Errorf("failed to query cidr_block: %w", err)
	}

	// 2. Find the first available entry and mark it as allocated
	var address string
	err = tx.QueryRowContext(ctx, `
		UPDATE ip_addresses 
		SET is_allocated = TRUE, container_id = ?, interface_name = ?, allocated_at = CURRENT_TIMESTAMP 
		WHERE id = (
			SELECT id FROM ip_addresses 
			WHERE cidr_block_id = ? AND is_allocated = FALSE AND (release_at IS NULL OR release_at <= CURRENT_TIMESTAMP)
			ORDER BY id ASC
			LIMIT 1
		)
		RETURNING address
	`, containerID, interfaceName, cidrBlockID).Scan(&address)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", sql.ErrNoRows
		}
		return "", "", fmt.Errorf("failed to allocate ip: %w", err)
	}

	// Also increment allocated_ips in cidr_blocks to keep it in sync
	_, err = tx.ExecContext(ctx, `
		UPDATE cidr_blocks 
		SET allocated_ips = allocated_ips + 1 
		WHERE id = ?
	`, cidrBlockID)

	if err != nil {
		return "", "", fmt.Errorf("failed to update allocated_ips in cidr_blocks: %w", err)
	}

	return address, cidrRange, nil
}

// allocateIP provides the generalized logic for managing IP allocations across address families.
func (s *Store) allocateIP(ctx context.Context, params AllocateIPParams) (string, string, error) {
	// 1. Idempotency check (Fast Path - outside write transaction)
	var address string
	var cidrRange string
	err := s.db.QueryRowContext(ctx, `
		SELECT i.address, c.cidr 
		FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE i.container_id = ? AND i.interface_name = ? AND i.is_allocated = TRUE AND c.ip_family = ?
		LIMIT 1
	`, params.ContainerID, params.InterfaceName, params.IPFamily).Scan(&address, &cidrRange)

	if err == nil {
		s.log.Info("Idempotency check hit (fast path), returning existing allocation", "containerID", params.ContainerID, "interfaceName", params.InterfaceName, "address", address, "cidr", cidrRange)
		return address, cidrRange, nil
	}
	if err != sql.ErrNoRows {
		return "", "", fmt.Errorf("failed during fast-path idempotency check: %w", err)
	}

	// 2. Query available CIDRs (Outside write transaction)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM cidr_blocks 
		WHERE network = ? AND ip_family = ? AND total_ips > allocated_ips AND state = 'Ready'
	`, params.Network, params.IPFamily)
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
		return "", "", fmt.Errorf("%w: no available cidr blocks found for network %s", ErrNoAvailableIPs, params.Network)
	}

	// 3. Loop and try to allocate with short transactions
	for _, cidrBlockID := range cidrBlockIDs {
		ip, cidr, err := s.tryAllocateIPInBlock(ctx, params, cidrBlockID)
		if err == nil {
			return ip, cidr, nil
		}
		if err == sql.ErrNoRows {
			s.log.V(4).Info("No available IPs in cidr block, tried next one", "cidrBlockID", cidrBlockID)
			continue
		}
		return "", "", err
	}

	if params.IPFamily == IPv6 && len(cidrBlockIDs) > 0 {
		// No IPs found in any block, try to expand one of them.
		for _, cidrBlockID := range cidrBlockIDs {
			err := s.expandIPv6Block(ctx, cidrBlockID)
			if err == nil {
				break // Successfully expanded one block!
			}
			if errors.Is(err, ErrCidrBlockExhausted) {
				s.log.Info("CIDR block exhausted, trying next one for expansion", "cidrBlockID", cidrBlockID)
				continue
			}
			return "", "", fmt.Errorf("failed to expand IPv6 block %d: %w", cidrBlockID, err)
		}

		// Whether we expanded it ourselves or another concurrent worker did,
		// we must retry the allocation once more.
		return s.allocateIP(ctx, params)
	}

	return "", "", fmt.Errorf("%w: failed to allocate %s in any cidr block for network %s", ErrNoAvailableIPs, params.IPFamily, params.Network)
}

// tryAllocateIPInBlock attempts to allocate an IP in a specific CIDR block.
// It handles transaction management and slow-path idempotency check.
func (s *Store) tryAllocateIPInBlock(ctx context.Context, params AllocateIPParams, cidrBlockID int64) (string, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var address, cidrRange string
	err = tx.QueryRowContext(ctx, `
		SELECT i.address, c.cidr 
		FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE i.container_id = ? AND i.interface_name = ? AND i.is_allocated = TRUE AND c.ip_family = ?
		LIMIT 1
	`, params.ContainerID, params.InterfaceName, params.IPFamily).Scan(&address, &cidrRange)

	if err == nil {
		s.log.Info("Idempotency check hit (slow path), returning existing allocation", "containerID", params.ContainerID, "interfaceName", params.InterfaceName, "address", address, "cidr", cidrRange)
		return address, cidrRange, nil
	}
	if err != sql.ErrNoRows {
		return "", "", fmt.Errorf("failed during slow-path idempotency check: %w", err)
	}

	ip, cidr, err := s.allocateIPTx(ctx, tx, cidrBlockID, params.InterfaceName, params.ContainerID)
	if err != nil {
		return "", "", err // Propagates sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return ip, cidr, nil
}

// getNextIPv6StartAddr finds the last inserted IP for a CIDR block and returns the next address to use.
// If no entries exist, it returns the CIDR base address.
func (s *Store) getNextIPv6StartAddr(ctx context.Context, tx *sql.Tx, cidrBlockID int64, prefix netip.Prefix) (netip.Addr, error) {
	var lastAddressStr string
	err := tx.QueryRowContext(ctx, `
		SELECT address FROM ip_addresses 
		WHERE cidr_block_id = ? 
		ORDER BY id DESC 
		LIMIT 1
	`, cidrBlockID).Scan(&lastAddressStr)

	switch err {
	case nil:
		startIP, parseErr := netip.ParseAddr(lastAddressStr)
		if parseErr != nil {
			return netip.Addr{}, fmt.Errorf("failed to parse last address %s: %w", lastAddressStr, parseErr)
		}
		return startIP.Next(), nil // Start from the next one
	case sql.ErrNoRows:
		return prefix.Addr(), nil // No entries yet, start from CIDR base address
	default:
		return netip.Addr{}, fmt.Errorf("failed to query last inserted ip: %w", err)
	}
}

// expandIPv6Block populates ipv6PopulationBatchSize entries for a given CIDR block in a new transaction.
func (s *Store) expandIPv6Block(ctx context.Context, cidrBlockID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Fetch CIDR range for the given ID
	var cidrRange string
	err = tx.QueryRowContext(ctx, `
		SELECT cidr FROM cidr_blocks 
		WHERE id = ? AND ip_family = 'ipv6' AND total_ips > allocated_ips AND state = 'Ready'
	`, cidrBlockID).Scan(&cidrRange)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrCidrBlockExhausted
		}
		return fmt.Errorf("failed to query cidr_block: %w", err)
	}

	prefix, err := netip.ParsePrefix(cidrRange)
	if err != nil {
		return fmt.Errorf("failed to parse cidr %s: %w", cidrRange, err)
	}

	// 2. Find the last inserted IP
	startIP, err := s.getNextIPv6StartAddr(ctx, tx, cidrBlockID, prefix)
	if err != nil {
		return err
	}

	// 3. Generate ipv6PopulationBatchSize IPs
	var ips []string
	curr := startIP
	for i := 0; i < ipv6PopulationBatchSize; i++ {
		if !prefix.Contains(curr) {
			break
		}
		ips = append(ips, curr.String())
		curr = curr.Next()
	}

	if len(ips) == 0 {
		return ErrCidrBlockExhausted
	}

	// 4. Insert them
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ip_addresses (cidr_block_id, address, is_allocated, container_id, interface_name) 
		VALUES (?, ?, FALSE, '', '')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, addr := range ips {
		_, err = stmt.ExecContext(ctx, cidrBlockID, addr)
		if err != nil {
			return fmt.Errorf("failed to insert ip_address %s: %w", addr, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info(fmt.Sprintf("Successfully expanded IPv6 block by %d entries", ipv6PopulationBatchSize), "cidrBlockID", cidrBlockID)
	return nil
}

// MarkCIDRBlockAsDeleting transitions a CIDR block to the Deleting state.
func (s *Store) MarkCIDRBlockAsDeleting(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE cidr_blocks SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", StateDeleting, id)
	return err
}

// GetInitialIPCount queries the total number of IPs in the first CIDR block for a network.
func (s *Store) GetInitialIPCount(ctx context.Context, network string) (int, error) {
	var initialIPs int
	err := s.db.QueryRowContext(ctx, "SELECT total_ips FROM cidr_blocks WHERE network = ? ORDER BY id ASC LIMIT 1", network).Scan(&initialIPs)
	if err != nil {
		return 0, err
	}
	return initialIPs, nil
}

// NetworkIPUsage holds the allocated, cooldown, total, and draining IP counts for a network.
type NetworkIPUsage struct {
	Allocated int
	Cooldown  int
	Total     int
	Draining  int
}

// GetIPUsageByNetwork fetches the allocated, cooldown, total, and draining IP counts for a specific network.
// CIDR blocks marked as Deleting are excluded from all counts since they are scheduled for removal by GCE.
func (s *Store) GetIPUsageByNetwork(ctx context.Context, network string) (NetworkIPUsage, error) {
	var usage NetworkIPUsage
	err := s.db.QueryRowContext(ctx, `
		SELECT 
			IFNULL(SUM(allocated_ips), 0) AS allocated,
			(
				SELECT COUNT(i.id) 
				FROM ip_addresses i 
				JOIN cidr_blocks cb ON i.cidr_block_id = cb.id 
				WHERE cb.network = ? AND cb.state != ? AND i.is_allocated = FALSE AND i.release_at > CURRENT_TIMESTAMP
			) AS cooldown,
			IFNULL(SUM(total_ips), 0) AS total_ips,
			IFNULL(SUM(CASE WHEN state = ? THEN total_ips ELSE 0 END), 0) AS draining_ips
		FROM cidr_blocks c
		WHERE network = ? AND c.state != ?
	`, network, StateDeleting, StateDraining, network, StateDeleting).Scan(&usage.Allocated, &usage.Cooldown, &usage.Total, &usage.Draining)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NetworkIPUsage{}, nil
		}
		return NetworkIPUsage{}, fmt.Errorf("failed to query IP usage for network %s: %w", network, err)
	}
	return usage, nil
}

// ReadyCIDRBlock holds metadata for a CIDR block in Ready state.
type ReadyCIDRBlock struct {
	ID           int64
	TotalIPs     int
	AllocatedIPs int
	CIDR         string
}

// GetReadyCIDRBlocksSorted fetches all Ready CIDR blocks for a network, sorted by created_at DESC.
func (s *Store) GetReadyCIDRBlocksSorted(ctx context.Context, network string) ([]ReadyCIDRBlock, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, total_ips, allocated_ips, cidr FROM cidr_blocks WHERE network = ? AND state = 'Ready' ORDER BY created_at DESC, id DESC", network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ReadyCIDRBlock
	for rows.Next() {
		var r ReadyCIDRBlock
		if err := rows.Scan(&r.ID, &r.TotalIPs, &r.AllocatedIPs, &r.CIDR); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// GetAllNetworks fetches all unique networks from cidr_blocks, excluding those in Deleting state.
func (s *Store) GetAllNetworks(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT network FROM cidr_blocks WHERE state != 'Deleting'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var network string
		if err := rows.Scan(&network); err != nil {
			return nil, err
		}
		result = append(result, network)
	}
	return result, nil
}

// CheckAllocation verifies that an IP address is assigned to the specified container interface on a given network.
func (s *Store) CheckAllocation(ctx context.Context, network, containerID, interfaceName string) error {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT i.id 
		FROM ip_addresses i 
		JOIN cidr_blocks c ON i.cidr_block_id = c.id 
		WHERE c.network = ? AND i.container_id = ? AND i.interface_name = ? AND i.is_allocated = TRUE
		LIMIT 1
	`, network, containerID, interfaceName).Scan(&id)

	if err == sql.ErrNoRows {
		return fmt.Errorf("no active allocation found for container %s interface %s on network %s", containerID, interfaceName, network)
	}
	if err != nil {
		return fmt.Errorf("failed to check active allocation: %w", err)
	}
	return nil
}
