package gitops

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// GitSource Operations

func (s *Store) ListSources(ctx context.Context) ([]GitSource, error) {
	query := `SELECT id, name, url, branch, target_dir, auth_method, auth_secret, sync_interval_mins, last_commit_hash, last_sync_error, last_sync_at, created_at FROM git_sources ORDER BY name ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]GitSource, 0)
	for rows.Next() {
		var src GitSource
		var authSecret, lastCommitHash, lastSyncError sql.NullString
		if err := rows.Scan(
			&src.ID, &src.Name, &src.URL, &src.Branch, &src.TargetDir,
			&src.AuthMethod, &authSecret, &src.SyncIntervalMins,
			&lastCommitHash, &lastSyncError, &src.LastSyncAt, &src.CreatedAt,
		); err != nil {
			return nil, err
		}
		if authSecret.Valid {
			src.AuthSecret = authSecret.String
		}
		if lastCommitHash.Valid {
			src.LastCommitHash = lastCommitHash.String
		}
		if lastSyncError.Valid {
			src.LastSyncError = lastSyncError.String
		}
		sources = append(sources, src)
	}
	return sources, nil
}

func (s *Store) GetSource(ctx context.Context, id string) (GitSource, error) {
	query := `SELECT id, name, url, branch, target_dir, auth_method, auth_secret, sync_interval_mins, last_commit_hash, last_sync_error, last_sync_at, created_at FROM git_sources WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var src GitSource
	var authSecret, lastCommitHash, lastSyncError sql.NullString
	err := row.Scan(
		&src.ID, &src.Name, &src.URL, &src.Branch, &src.TargetDir,
		&src.AuthMethod, &authSecret, &src.SyncIntervalMins,
		&lastCommitHash, &lastSyncError, &src.LastSyncAt, &src.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return GitSource{}, fmt.Errorf("git source not found: %s", id)
		}
		return GitSource{}, err
	}
	if authSecret.Valid {
		src.AuthSecret = authSecret.String
	}
	if lastCommitHash.Valid {
		src.LastCommitHash = lastCommitHash.String
	}
	if lastSyncError.Valid {
		src.LastSyncError = lastSyncError.String
	}
	return src, nil
}

func (s *Store) CreateSource(ctx context.Context, src GitSource) error {
	query := `INSERT INTO git_sources (id, name, url, branch, target_dir, auth_method, auth_secret, sync_interval_mins, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().Unix()

	var authSecret sql.NullString
	if src.AuthSecret != "" {
		authSecret.Valid = true
		authSecret.String = src.AuthSecret
	}

	_, err := s.db.ExecContext(ctx, query,
		src.ID, src.Name, src.URL, src.Branch, src.TargetDir,
		src.AuthMethod, authSecret, src.SyncIntervalMins, now,
	)
	return err
}

func (s *Store) UpdateSourceSyncStatus(ctx context.Context, id, commitHash, syncError string) error {
	query := `UPDATE git_sources SET last_commit_hash = ?, last_sync_error = ?, last_sync_at = ? WHERE id = ?`

	var lastCommitHash, lastSyncError sql.NullString
	if commitHash != "" {
		lastCommitHash.Valid = true
		lastCommitHash.String = commitHash
	}
	if syncError != "" {
		lastSyncError.Valid = true
		lastSyncError.String = syncError
	}

	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, query, lastCommitHash, lastSyncError, now, id)
	return err
}

func (s *Store) DeleteSource(ctx context.Context, id string) error {
	// Foreign key constraints will cascade delete deployments
	query := `DELETE FROM git_sources WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// GitDeployment Operations

func (s *Store) ListDeploymentsForSource(ctx context.Context, sourceID string) ([]GitDeployment, error) {
	query := `SELECT id, git_source_id, compose_path, env_vars_json, env_file_path, env_inline_content, env_inline_enabled, auto_created, enabled, last_deployed_hash, last_deployed_at, last_error FROM git_deployments WHERE git_source_id = ? ORDER BY compose_path ASC`
	rows, err := s.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deps := make([]GitDeployment, 0)
	for rows.Next() {
		var dep GitDeployment
		var envVars, envFilePath, envInlineContent, lastDepHash, lastError sql.NullString
		if err := rows.Scan(
			&dep.ID, &dep.GitSourceID, &dep.ComposePath, &envVars, &envFilePath, &envInlineContent, &dep.EnvInlineEnabled, &dep.AutoCreated, &dep.Enabled,
			&lastDepHash, &dep.LastDeployedAt, &lastError,
		); err != nil {
			return nil, err
		}
		if envVars.Valid {
			dep.EnvVarsJSON = envVars.String
		}
		if lastDepHash.Valid {
			dep.LastDeployedHash = lastDepHash.String
		}
		if envFilePath.Valid {
			dep.EnvFilePath = envFilePath.String
		}
		if envInlineContent.Valid {
			dep.EnvInlineContent = envInlineContent.String
		}
		if lastError.Valid {
			dep.LastError = lastError.String
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

func (s *Store) CreateDeployment(ctx context.Context, dep GitDeployment) error {
	query := `INSERT INTO git_deployments (id, git_source_id, compose_path, env_vars_json, env_file_path, env_inline_content, env_inline_enabled, auto_created, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var envVars, envFilePath, envInlineContent sql.NullString
	if dep.EnvVarsJSON != "" {
		envVars.Valid = true
		envVars.String = dep.EnvVarsJSON
	}
	if dep.EnvFilePath != "" {
		envFilePath.Valid = true
		envFilePath.String = dep.EnvFilePath
	}
	if dep.EnvInlineContent != "" {
		envInlineContent.Valid = true
		envInlineContent.String = dep.EnvInlineContent
	}

	_, err := s.db.ExecContext(ctx, query, dep.ID, dep.GitSourceID, dep.ComposePath, envVars, envFilePath, envInlineContent, dep.EnvInlineEnabled, dep.AutoCreated, dep.Enabled)
	return err
}

func (s *Store) CreateDeploymentIfMissing(ctx context.Context, dep GitDeployment) (bool, error) {
	query := `INSERT INTO git_deployments (id, git_source_id, compose_path, env_vars_json, env_file_path, env_inline_content, env_inline_enabled, auto_created, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(git_source_id, compose_path) DO NOTHING`

	var envVars, envFilePath, envInlineContent sql.NullString
	if dep.EnvVarsJSON != "" {
		envVars.Valid = true
		envVars.String = dep.EnvVarsJSON
	}
	if dep.EnvFilePath != "" {
		envFilePath.Valid = true
		envFilePath.String = dep.EnvFilePath
	}
	if dep.EnvInlineContent != "" {
		envInlineContent.Valid = true
		envInlineContent.String = dep.EnvInlineContent
	}

	res, err := s.db.ExecContext(ctx, query, dep.ID, dep.GitSourceID, dep.ComposePath, envVars, envFilePath, envInlineContent, dep.EnvInlineEnabled, dep.AutoCreated, dep.Enabled)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Store) GetDeployment(ctx context.Context, id string) (GitDeployment, error) {
	query := `SELECT id, git_source_id, compose_path, env_vars_json, env_file_path, env_inline_content, env_inline_enabled, auto_created, enabled, last_deployed_hash, last_deployed_at, last_error FROM git_deployments WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var dep GitDeployment
	var envVars, envFilePath, envInlineContent, lastDepHash, lastError sql.NullString
	if err := row.Scan(
		&dep.ID, &dep.GitSourceID, &dep.ComposePath, &envVars, &envFilePath, &envInlineContent, &dep.EnvInlineEnabled, &dep.AutoCreated, &dep.Enabled,
		&lastDepHash, &dep.LastDeployedAt, &lastError,
	); err != nil {
		if err == sql.ErrNoRows {
			return GitDeployment{}, fmt.Errorf("git deployment not found: %s", id)
		}
		return GitDeployment{}, err
	}
	if envVars.Valid {
		dep.EnvVarsJSON = envVars.String
	}
	if envFilePath.Valid {
		dep.EnvFilePath = envFilePath.String
	}
	if envInlineContent.Valid {
		dep.EnvInlineContent = envInlineContent.String
	}
	if lastDepHash.Valid {
		dep.LastDeployedHash = lastDepHash.String
	}
	if lastError.Valid {
		dep.LastError = lastError.String
	}
	return dep, nil
}

func (s *Store) UpdateDeployment(ctx context.Context, dep GitDeployment) error {
	query := `UPDATE git_deployments SET compose_path = ?, env_vars_json = ?, env_file_path = ?, env_inline_content = ?, env_inline_enabled = ?, auto_created = ?, enabled = ? WHERE id = ?`

	var envVars, envFilePath, envInlineContent sql.NullString
	if dep.EnvVarsJSON != "" {
		envVars.Valid = true
		envVars.String = dep.EnvVarsJSON
	}
	if dep.EnvFilePath != "" {
		envFilePath.Valid = true
		envFilePath.String = dep.EnvFilePath
	}
	if dep.EnvInlineContent != "" {
		envInlineContent.Valid = true
		envInlineContent.String = dep.EnvInlineContent
	}

	res, err := s.db.ExecContext(ctx, query, dep.ComposePath, envVars, envFilePath, envInlineContent, dep.EnvInlineEnabled, dep.AutoCreated, dep.Enabled, dep.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("git deployment not found: %s", dep.ID)
	}
	return nil
}

func (s *Store) UpdateDeploymentStatus(ctx context.Context, id, deployedHash, deployError string) error {
	query := `UPDATE git_deployments SET last_deployed_hash = ?, last_error = ?, last_deployed_at = ? WHERE id = ?`

	var lastDepHash, lastError sql.NullString
	if deployedHash != "" {
		lastDepHash.Valid = true
		lastDepHash.String = deployedHash
	}
	if deployError != "" {
		lastError.Valid = true
		lastError.String = deployError
	}

	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, query, lastDepHash, lastError, now, id)
	return err
}

func (s *Store) DeleteDeployment(ctx context.Context, id string) error {
	query := `DELETE FROM git_deployments WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *Store) UpdateDeploymentEnabled(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE git_deployments SET enabled = ?, auto_created = CASE WHEN enabled != ? THEN 0 ELSE auto_created END WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, enabled, enabled, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("git deployment not found: %s", id)
	}
	return err
}
