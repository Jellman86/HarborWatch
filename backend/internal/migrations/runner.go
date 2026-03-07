package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type RunResult struct {
	AppliedCount    int
	CurrentVersion  int
	AppliedVersions []int
}

func Run(ctx context.Context, db *sql.DB) (RunResult, error) {
	if db == nil {
		return RunResult{}, fmt.Errorf("nil db")
	}
	if err := ensureTable(ctx, db); err != nil {
		return RunResult{}, err
	}

	migs := defaultMigrations()
	if err := validateMigrations(migs); err != nil {
		return RunResult{}, err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{}
	for _, m := range migs {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return RunResult{}, err
		}
		result.AppliedCount++
		result.AppliedVersions = append(result.AppliedVersions, m.Version)
	}

	version, err := CurrentVersion(ctx, db)
	if err != nil {
		return RunResult{}, err
	}
	result.CurrentVersion = version
	return result, nil
}

func CurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("nil db")
	}
	if err := ensureTable(ctx, db); err != nil {
		return 0, err
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("query current schema version: %w", err)
	}
	return version, nil
}

func ensureTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at INTEGER NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func validateMigrations(migs []Migration) error {
	if len(migs) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(migs))
	versions := make([]int, 0, len(migs))
	for _, m := range migs {
		if m.Version <= 0 {
			return fmt.Errorf("invalid migration version %d (%s)", m.Version, m.Name)
		}
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("empty migration name for version %d", m.Version)
		}
		if strings.TrimSpace(m.SQL) == "" && m.ApplyTx == nil {
			return fmt.Errorf("migration %d (%s) has no SQL or ApplyTx", m.Version, m.Name)
		}
		if _, ok := seen[m.Version]; ok {
			return fmt.Errorf("duplicate migration version %d", m.Version)
		}
		seen[m.Version] = struct{}{}
		versions = append(versions, m.Version)
	}
	if !sort.IntsAreSorted(versions) {
		return fmt.Errorf("migrations must be sorted by version")
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied schema migrations: %w", err)
	}
	defer rows.Close()

	out := map[int]struct{}{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema migration version: %w", err)
		}
		out[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migration versions: %w", err)
	}
	return out, nil
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.Version, err)
	}
	defer func() { _ = tx.Rollback() }()

	if strings.TrimSpace(m.SQL) != "" {
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("execute migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	if m.ApplyTx != nil {
		if err := m.ApplyTx(ctx, tx); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO schema_migrations(version, name, applied_at)
VALUES(?, ?, ?)
`, m.Version, m.Name, time.Now().UTC().Unix()); err != nil {
		return fmt.Errorf("record migration %d (%s): %w", m.Version, m.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d (%s): %w", m.Version, m.Name, err)
	}
	return nil
}

func hasColumnTx(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func ensureColumnTx(ctx context.Context, tx *sql.Tx, table, column, ddl string) error {
	ok, err := hasColumnTx(ctx, tx, table, column)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
	return err
}

func migrateContainerRulesColumns(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"container_name", "TEXT NOT NULL DEFAULT ''"},
		{"validate_mode", "TEXT DEFAULT 'both'"},
		{"validate_timeout_sec", "INTEGER DEFAULT 45"},
		{"validate_interval_sec", "INTEGER DEFAULT 2"},
		{"ai_validate_logs", "INTEGER DEFAULT 0"},
		{"bypass_ai", "INTEGER DEFAULT 0"},
		{"skip_health_check", "INTEGER DEFAULT 0"},
		{"inherit_automation", "INTEGER DEFAULT 1"},
		{"upgrades_automation", "INTEGER DEFAULT 1"},
		{"maintenance_automation", "INTEGER DEFAULT 1"},
		{"security_automation", "INTEGER DEFAULT 1"},
		{"restart_on_unhealthy", "INTEGER DEFAULT 0"},
		{"unhealthy_restart_cooldown_sec", "INTEGER DEFAULT 0"},
	}
	for _, c := range columns {
		if err := ensureColumnTx(ctx, tx, "container_rules", c.name, c.ddl); err != nil {
			return fmt.Errorf("ensure container_rules.%s: %w", c.name, err)
		}
	}
	return nil
}

func migrateContainerRulesDependentRestartColumns(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"restart_dependents_after_upgrade", "INTEGER DEFAULT 0"},
		{"dependent_restart_delay_sec", "INTEGER DEFAULT 20"},
	}
	for _, c := range columns {
		if err := ensureColumnTx(ctx, tx, "container_rules", c.name, c.ddl); err != nil {
			return fmt.Errorf("ensure container_rules.%s: %w", c.name, err)
		}
	}
	return nil
}

func migrateUpdateRunsColumnsAndIndexes(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"progress", "INTEGER NOT NULL DEFAULT 0"},
		{"container_id", "TEXT NOT NULL DEFAULT ''"},
		{"container_name", "TEXT NOT NULL DEFAULT ''"},
		{"ai_analysis", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range columns {
		if err := ensureColumnTx(ctx, tx, "update_runs", c.name, c.ddl); err != nil {
			return fmt.Errorf("ensure update_runs.%s: %w", c.name, err)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_update_runs_container_updated ON update_runs(container_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_update_runs_container_name_updated ON update_runs(container_name, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_update_runs_status_created ON update_runs(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_update_steps_run_id_id ON update_steps(run_id, id)`,
	}
	for _, stmt := range indexes {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func migrateUniqueActiveUpdateIndexBestEffort(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_update_runs_one_running_per_container ON update_runs(container_id) WHERE status IN ('running','queued')`)
	if err != nil {
		// Existing installs may contain duplicate active rows. Keep startup/migrations non-blocking,
		// matching previous store.Init best-effort behavior.
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil
		}
		return err
	}
	return nil
}

func migrateScanningColumnsAndIndexes(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"malware_scan_results", "container_name", "TEXT NOT NULL DEFAULT ''"},
		{"scan_jobs", "container_name", "TEXT NOT NULL DEFAULT ''"},
		{"scan_jobs", "progress", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, c := range columns {
		if err := ensureColumnTx(ctx, tx, c.table, c.name, c.ddl); err != nil {
			return fmt.Errorf("ensure %s.%s: %w", c.table, c.name, err)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_scan_results_target_scanned ON scan_results(target, scanned_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_malware_results_target_scanned ON malware_scan_results(target, scanned_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_malware_results_container_scanned ON malware_scan_results(container_name, scanned_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_jobs_type_started ON scan_jobs(type, started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_jobs_status_started ON scan_jobs(status, started_at DESC)`,
	}
	for _, stmt := range indexes {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func migrateContainerIntelColumns(ctx context.Context, tx *sql.Tx) error {
	if err := ensureColumnTx(ctx, tx, "container_intel_overrides", "container_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("ensure container_intel_overrides.container_name: %w", err)
	}
	return nil
}

func migrateGitOpsInlineEnvColumns(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"env_inline_content", "TEXT"},
		{"env_inline_enabled", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, c := range columns {
		if err := ensureColumnTx(ctx, tx, "git_deployments", c.name, c.ddl); err != nil {
			return fmt.Errorf("ensure git_deployments.%s: %w", c.name, err)
		}
	}
	return nil
}
