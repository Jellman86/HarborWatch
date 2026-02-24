CREATE TABLE IF NOT EXISTS container_intel_overrides (
	container_id TEXT PRIMARY KEY,
	container_name TEXT NOT NULL DEFAULT '',
	repository_url TEXT NOT NULL DEFAULT '',
	changelog_url TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0
);
