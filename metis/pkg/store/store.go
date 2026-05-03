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
	"net"
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
	ipv6PopulationBatchSize = 32
)

var (
	// ErrCidrAlreadyExists is returned when a CIDR block already exists in the store.
	ErrCidrAlreadyExists = errors.New("cidr block already exists")

	// ErrNoAvailableIPs is returned when no available IPs can be found in any CIDR block.
	ErrNoAvailableIPs = errors.New("no available IPs in store")

	// ErrCidrBlockExhausted is returned when an IPv6 CIDR block cannot be expanded further.
	ErrCidrBlockExhausted = errors.New("cidr block exhausted and cannot be expanded")
)

// IPFamily represents the IP protocol family.
type IPFamily string

const (
	IPv4 IPFamily = "ipv4"
	IPv6 IPFamily = "ipv6"
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

// GetCIDRBlockByCIDR checks if a CIDR block already exists in the database.
func (s *Store) GetCIDRBlockByCIDR(ctx context.Context, cidr string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM cidr_blocks WHERE cidr = ? LIMIT 1
	`, cidr).Scan(&id)

	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("failed to query cidr_blocks: %w", err)
}

// AddCIDR parses the CIDR, determines family, and inserts it + its constituent IP addresses into the store.
// For IPv4, it populates all IPs. For IPv6, it only adds the CIDR block.
func (s *Store) AddCIDR(ctx context.Context, network, cidr string) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("failed to parse cidr %s: %w", cidr, err)
	}

	ipFamily := IPv4
	isIPv6 := false
	if ip.To4() == nil {
		ipFamily = IPv6
		isIPv6 = true
	}

	var totalIPs int64
	if !isIPv6 {
		ones, bits := ipnet.Mask.Size()
		totalIPs = 1 << (bits - ones)
	} else {
		ones, _ := ipnet.Mask.Size()
		bits := 128 - ones
		if bits >= 62 {
			totalIPs = 0x7fffffffffffffff // Max int64 for large IPv6 ranges
		} else {
			totalIPs = 1 << bits
		}
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
		for curr := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(curr); curr = incIP(curr) {
			ips = append(ips, curr.String())
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
		startIP := ipnet.IP.Mask(ipnet.Mask)

		var ips []string
		curr := startIP
		for i := 0; i < ipv6PopulationBatchSize; i++ {
			ips = append(ips, curr.String())
			curr = incIP(curr)
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
			SET is_allocated = FALSE, release_at = ? 
			WHERE id = ?
		`, releaseAt, r.id)
		if err != nil {
			return 0, fmt.Errorf("failed to release IP %d: %w", r.id, err)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE cidr_blocks 
			SET allocated_ips = allocated_ips - 1 
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

	_, ipnet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return fmt.Errorf("failed to parse cidr %s: %w", cidrRange, err)
	}

	// 2. Find the last inserted IP
	var lastAddressStr string
	err = tx.QueryRowContext(ctx, `
		SELECT address FROM ip_addresses 
		WHERE cidr_block_id = ? 
		ORDER BY id DESC 
		LIMIT 1
	`, cidrBlockID).Scan(&lastAddressStr)

	var startIP net.IP
	if err == nil {
		startIP = net.ParseIP(lastAddressStr)
		startIP = incIP(startIP) // Start from the next one
	} else if err == sql.ErrNoRows {
		// No entries yet, start from CIDR base address
		startIP = ipnet.IP.Mask(ipnet.Mask)
	} else {
		return fmt.Errorf("failed to query last inserted ip: %w", err)
	}

	// 3. Generate ipv6PopulationBatchSize IPs
	var ips []string
	curr := startIP
	for i := 0; i < ipv6PopulationBatchSize; i++ {
		if !ipnet.Contains(curr) {
			break
		}
		ips = append(ips, curr.String())
		curr = incIP(curr)
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

// incIP increments an IP address.
func incIP(ip net.IP) net.IP {
	newIP := make(net.IP, len(ip))
	copy(newIP, ip)
	for i := len(newIP) - 1; i >= 0; i-- {
		newIP[i]++
		if newIP[i] > 0 {
			break
		}
	}
	return newIP
}
