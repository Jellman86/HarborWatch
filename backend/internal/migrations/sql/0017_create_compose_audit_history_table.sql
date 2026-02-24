CREATE TABLE IF NOT EXISTS compose_audit_history (
	id TEXT PRIMARY KEY,
	container_id TEXT NOT NULL,
	container_name TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	headline TEXT NOT NULL DEFAULT '',
	compose_config TEXT NOT NULL,
	analysis_markdown TEXT NOT NULL,
	analysis_html TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_compose_audit_container_created ON compose_audit_history(container_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_compose_audit_created ON compose_audit_history(created_at DESC);
