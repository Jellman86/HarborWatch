ALTER TABLE git_deployments ADD COLUMN env_file_path TEXT;
ALTER TABLE git_deployments ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
