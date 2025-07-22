-- Resources table for storing all Kubernetes-like resources
CREATE TABLE IF NOT EXISTS resources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    namespace TEXT DEFAULT 'default',
    name TEXT NOT NULL,
    spec TEXT NOT NULL,  -- JSON blob
    status TEXT,         -- JSON blob
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(kind, namespace, name)
);

-- Nodes table for tracking cluster nodes
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    address TEXT NOT NULL,
    status TEXT NOT NULL,  -- JSON blob
    last_heartbeat DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Pod assignments for tracking which pods run on which nodes
CREATE TABLE IF NOT EXISTS pod_assignments (
    pod_id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES nodes(id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_resources_kind ON resources(kind);
CREATE INDEX IF NOT EXISTS idx_resources_namespace ON resources(namespace);
CREATE INDEX IF NOT EXISTS idx_resources_name ON resources(name);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_pod_assignments_node ON pod_assignments(node_id);