-- Remote File Manager Database Schema
-- Version: 001
-- Description: Initial database schema with 5 core tables

-- 1. devices table: Store registered device information
CREATE TABLE IF NOT EXISTS devices (
    device_id TEXT PRIMARY KEY,
    device_name TEXT NOT NULL,
    platform TEXT NOT NULL,
    version TEXT NOT NULL,
    ip TEXT,
    last_seen INTEGER NOT NULL,
    status TEXT NOT NULL,
    allowed_roots TEXT NOT NULL,  -- JSON array
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen);

-- 2. uploaded_objects table: Store uploaded file objects
CREATE TABLE IF NOT EXISTS uploaded_objects (
    object_id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    source_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    storage_path TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME,
    expires_at DATETIME,
    FOREIGN KEY (device_id) REFERENCES devices(device_id)
);

CREATE INDEX IF NOT EXISTS idx_objects_device_id ON uploaded_objects(device_id);
CREATE INDEX IF NOT EXISTS idx_objects_created_at ON uploaded_objects(created_at);
CREATE INDEX IF NOT EXISTS idx_objects_expires_at ON uploaded_objects(expires_at);

-- 3. download_tokens table: Store temporary download tokens
CREATE TABLE IF NOT EXISTS download_tokens (
    token TEXT PRIMARY KEY,
    object_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    FOREIGN KEY (object_id) REFERENCES uploaded_objects(object_id)
);

CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON download_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_tokens_object_id ON download_tokens(object_id);

-- 4. audit_logs table: Store audit logs for admin operations
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    admin TEXT NOT NULL,
    action TEXT NOT NULL,
    device_id TEXT,
    path TEXT,
    result TEXT NOT NULL,
    error TEXT,
    ip TEXT,
    FOREIGN KEY (device_id) REFERENCES devices(device_id)
);

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_device_id ON audit_logs(device_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);

-- 5. admin_sessions table: Store admin session information
CREATE TABLE IF NOT EXISTS admin_sessions (
    session_id TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    ip TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON admin_sessions(expires_at);
