CREATE TABLE git_sources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    branch TEXT NOT NULL,
    target_dir TEXT NOT NULL,
    auth_method TEXT NOT NULL, -- 'none', 'http_token', 'ssh_key'
    auth_secret TEXT,
    sync_interval_mins INTEGER NOT NULL DEFAULT 5,
    last_commit_hash TEXT,
    last_sync_error TEXT,
    last_sync_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE git_deployments (
    id TEXT PRIMARY KEY,
    git_source_id TEXT NOT NULL,
    compose_path TEXT NOT NULL,
    env_vars_json TEXT, -- JSON string map of overrides
    last_deployed_hash TEXT,
    last_deployed_at INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    FOREIGN KEY(git_source_id) REFERENCES git_sources(id) ON DELETE CASCADE
);

CREATE INDEX idx_git_deployments_source ON git_deployments(git_source_id);
