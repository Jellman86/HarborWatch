DELETE FROM git_deployments
WHERE rowid NOT IN (
    SELECT MIN(rowid)
    FROM git_deployments
    GROUP BY git_source_id, compose_path
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_git_deployments_source_compose
ON git_deployments(git_source_id, compose_path);
