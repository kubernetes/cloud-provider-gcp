-- cidr_blocks tracks the lifecycle and utilization of all IP alias ranges 
-- provisioned to this node by GCE.
CREATE TABLE IF NOT EXISTS cidr_blocks (
    -- Unique identifier for the CIDR block.
    -- Note: SQLite INTEGER uses variable length encoding with a max of 8 bytes.
    -- This provides a theoretical max of 2^63, which is sufficient for 
    -- 1 pod allocation per second for ~6 billion years.
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- The actual IP range allocated from GCE. 
    -- Example: '10.0.1.0/28'
    cidr TEXT NOT NULL,

    -- The logical network this block belongs to, matching the CNI networkName.
    -- Example: 'gke-pod-network'
    network TEXT NOT NULL,

    -- The protocol family of the IP range.
    -- Example: 'ipv4' or 'ipv6'
    ip_family TEXT NOT NULL,

    -- The total number of IP addresses contained within this block.
    -- Note: SQLite does not support unsigned integers, so standard INTEGER is used.
    -- Example: 16
    total_ips INTEGER NOT NULL,

    -- The current count of IPs within this block that are actively assigned to pods.
    -- Example: 5
    allocated_ips INTEGER DEFAULT 0,

    -- The current operational state of the block. 
    -- Expected values: 'Ready', 'Draining', 'Deleting'
    state TEXT NOT NULL DEFAULT 'Ready',

    -- Unix epoch timestamp in milliseconds when this block was successfully pulled from the CRD.
    created_at INTEGER DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),

    -- Unix epoch timestamp in milliseconds of the last mutation to this block's state or capacity.
    updated_at INTEGER DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),

    UNIQUE(cidr, network)
);

-- ip_addresses tracks the assignment state, ownership, and cooldown periods 
-- of every individual IP address managed by the daemon.
CREATE TABLE IF NOT EXISTS ip_addresses (
    -- Unique identifier for the individual IP record.
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- The specific IP address string.
    -- Example: '10.0.1.2'
    address TEXT NOT NULL,

    -- The parent CIDR block this IP belongs to. 
    -- Enforces cascading deletes if the parent block is removed by the daemon.
    cidr_block_id INTEGER NOT NULL,

    -- The CNI_CONTAINER_ID of the pod currently holding this IP.
    -- Example: 'f093u09jfioj...'
    container_id TEXT,

    -- The Kubernetes Pod Name holding this IP.
    pod_name TEXT,

    -- The Kubernetes Pod Namespace holding this IP.
    pod_namespace TEXT,

    -- The CNI_IFNAME inside the container holding this IP.
    -- Example: 'eth0'
    interface_name TEXT,

    -- Represents whether the IP is currently held by an active pod.
    is_allocated BOOLEAN DEFAULT FALSE,

    -- Unix epoch timestamp in milliseconds indicating when a released IP has 
    -- finished its "cool-down" period and is safe to be reassigned.
    release_at INTEGER, 

    -- Unix epoch timestamp in milliseconds when the IP was assigned to its current container_id.
    allocated_at INTEGER,

    -- Unix epoch timestamp in milliseconds of the last mutation to this IP record.
    updated_at INTEGER DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),

    FOREIGN KEY (cidr_block_id) REFERENCES cidr_blocks(id) ON DELETE CASCADE,
    UNIQUE(address, cidr_block_id)
);

-- Index to optimize the daemon's search for the next available IP address.
CREATE INDEX IF NOT EXISTS idx_available_ips 
    ON ip_addresses(cidr_block_id, id) 
    WHERE is_allocated = FALSE;

-- Composite index to guarantee fast, idempotent lookups during CNI cmdAdd retries.
CREATE INDEX IF NOT EXISTS idx_ip_idempotency
    ON ip_addresses(container_id, interface_name);

-- Automatically update the updated_at timestamp on cidr_blocks mutations.
CREATE TRIGGER IF NOT EXISTS update_cidr_blocks_updated_at
    AFTER UPDATE ON cidr_blocks FOR EACH ROW BEGIN
    UPDATE cidr_blocks SET updated_at = CAST(unixepoch('subsec') * 1000 AS INTEGER) WHERE id = OLD.id;
    END;

-- Automatically update the updated_at timestamp on ip_addresses mutations.
CREATE TRIGGER IF NOT EXISTS update_ip_addresses_updated_at
    AFTER UPDATE ON ip_addresses FOR EACH ROW BEGIN
        UPDATE ip_addresses SET updated_at = CAST(unixepoch('subsec') * 1000 AS INTEGER) WHERE id = OLD.id;
    END;
