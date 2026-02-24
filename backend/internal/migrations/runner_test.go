package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func latestVersion(t *testing.T) int {
	t.Helper()
	migs := defaultMigrations()
	if len(migs) == 0 {
		return 0
	}
	return migs[len(migs)-1].Version
}

func hasColumn(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma row for %s: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma rows for %s: %v", table, err)
	}
	return false
}

func TestRunAppliesBaselineAndReportsCurrentVersion(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	result, err := Run(ctx, db)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.CurrentVersion != latestVersion(t) {
		t.Fatalf("expected current version %d, got %d", latestVersion(t), result.CurrentVersion)
	}
	if result.AppliedCount != len(defaultMigrations()) {
		t.Fatalf("expected %d applied migrations, got %d", len(defaultMigrations()), result.AppliedCount)
	}

	version, err := CurrentVersion(ctx, db)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != latestVersion(t) {
		t.Fatalf("expected CurrentVersion=%d, got %d", latestVersion(t), version)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	result, err := Run(ctx, db)
	if err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	if result.AppliedCount != 0 {
		t.Fatalf("expected second Run to apply 0 migrations, got %d", result.AppliedCount)
	}
	if result.CurrentVersion != latestVersion(t) {
		t.Fatalf("expected current version %d after rerun, got %d", latestVersion(t), result.CurrentVersion)
	}

	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rows); err != nil {
		t.Fatalf("count schema_migrations rows: %v", err)
	}
	if rows != len(defaultMigrations()) {
		t.Fatalf("expected %d schema migration rows, got %d", len(defaultMigrations()), rows)
	}
}

func TestRunCreatesSettingsAndRulesTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !hasColumn(t, db, "app_settings", "key") || !hasColumn(t, db, "app_settings", "value") {
		t.Fatalf("expected app_settings table with key/value columns after migrations")
	}

	requiredRulesColumns := []string{
		"container_id",
		"container_name",
		"update_policy",
		"validate_url",
		"validate_mode",
		"validate_timeout_sec",
		"validate_interval_sec",
		"bypass_ai",
		"skip_health_check",
		"ai_validate_logs",
		"auto_rollback",
		"inherit_automation",
		"upgrades_automation",
		"maintenance_automation",
		"security_automation",
		"restart_on_unhealthy",
		"unhealthy_restart_cooldown_sec",
		"restart_dependents_after_upgrade",
		"dependent_restart_delay_sec",
	}
	for _, col := range requiredRulesColumns {
		if !hasColumn(t, db, "container_rules", col) {
			t.Fatalf("expected container_rules.%s after migrations", col)
		}
	}
}

func TestRunCreatesUpdatesTablesAndIndexes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, table := range []string{"update_runs", "update_steps"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("expected table %s after migrations: %v", table, err)
		}
	}

	for _, col := range []string{"progress", "container_id", "container_name", "ai_analysis"} {
		if !hasColumn(t, db, "update_runs", col) {
			t.Fatalf("expected update_runs.%s after migrations", col)
		}
	}

	// Check representative indexes exist.
	for _, idx := range []string{
		"idx_update_runs_container_updated",
		"idx_update_runs_status_created",
		"idx_update_steps_run_id_id",
	} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='index' AND name=? LIMIT 1`, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("expected index %s after migrations: %v", idx, err)
		}
	}
}

func TestRunUpgradesLegacyUpdateRunsColumns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
CREATE TABLE update_runs (
  id TEXT PRIMARY KEY,
  target_image TEXT NOT NULL,
  validate_url TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE update_steps (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  step TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL,
  ts INTEGER NOT NULL
);
`)
	if err != nil {
		t.Fatalf("create legacy update tables: %v", err)
	}

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed on legacy updates schema: %v", err)
	}

	for _, col := range []string{"progress", "container_id", "container_name", "ai_analysis"} {
		if !hasColumn(t, db, "update_runs", col) {
			t.Fatalf("expected legacy upgrade to add update_runs.%s", col)
		}
	}
}

func TestRunCreatesScanningTablesAndIndexes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, table := range []string{"scan_results", "malware_scan_results", "scan_jobs"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("expected table %s after migrations: %v", table, err)
		}
	}

	for _, check := range []struct {
		table string
		col   string
	}{
		{"malware_scan_results", "container_name"},
		{"scan_jobs", "container_name"},
		{"scan_jobs", "progress"},
	} {
		if !hasColumn(t, db, check.table, check.col) {
			t.Fatalf("expected %s.%s after migrations", check.table, check.col)
		}
	}

	for _, idx := range []string{
		"idx_scan_results_target_scanned",
		"idx_malware_results_target_scanned",
		"idx_scan_jobs_type_started",
		"idx_scan_jobs_status_started",
	} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='index' AND name=? LIMIT 1`, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("expected index %s after migrations: %v", idx, err)
		}
	}
}

func TestRunCreatesMetricsAndSchedulerTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, table := range []string{"container_metrics", "schedules"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("expected table %s after migrations: %v", table, err)
		}
	}

	if !hasColumn(t, db, "container_metrics", "container_id") || !hasColumn(t, db, "container_metrics", "timestamp") {
		t.Fatalf("expected container_metrics core columns after migrations")
	}
	if !hasColumn(t, db, "schedules", "id") || !hasColumn(t, db, "schedules", "cron_spec") {
		t.Fatalf("expected schedules core columns after migrations")
	}

	var exists int
	if err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='index' AND name='idx_metrics_container_ts' LIMIT 1`).Scan(&exists); err != nil {
		t.Fatalf("expected idx_metrics_container_ts after migrations: %v", err)
	}
}

func TestRunCreatesDiagRemediationAndContainerIntelTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, table := range []string{"internal_logs", "remediation_runs", "container_intel_overrides"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("expected table %s after migrations: %v", table, err)
		}
	}

	if !hasColumn(t, db, "internal_logs", "timestamp") || !hasColumn(t, db, "internal_logs", "source") {
		t.Fatalf("expected internal_logs core columns after migrations")
	}
	if !hasColumn(t, db, "remediation_runs", "container_id") || !hasColumn(t, db, "remediation_runs", "status") {
		t.Fatalf("expected remediation_runs core columns after migrations")
	}
	if !hasColumn(t, db, "container_intel_overrides", "container_name") {
		t.Fatalf("expected container_intel_overrides.container_name after migrations")
	}

	for _, idx := range []string{"idx_logs_ts", "idx_remediation_runs_container"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='index' AND name=? LIMIT 1`, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("expected index %s after migrations: %v", idx, err)
		}
	}
}

func TestRunCreatesAITablesAndIndexes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, table := range []string{"ai_usage_events", "ai_conversations", "ai_fleet_advice", "compose_audit_history"} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("expected table %s after migrations: %v", table, err)
		}
	}

	for _, idx := range []string{
		"idx_ai_usage_ts",
		"idx_ai_usage_provider_model",
		"idx_ai_conv_ts",
		"idx_compose_audit_container_created",
		"idx_compose_audit_created",
	} {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='index' AND name=? LIMIT 1`, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("expected index %s after migrations: %v", idx, err)
		}
	}
}

func TestRunUpgradesLegacyContainerIntelColumns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
CREATE TABLE container_intel_overrides (
	container_id TEXT PRIMARY KEY,
	repository_url TEXT NOT NULL DEFAULT '',
	changelog_url TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		t.Fatalf("create legacy container_intel_overrides table: %v", err)
	}

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed on legacy container intel schema: %v", err)
	}

	if !hasColumn(t, db, "container_intel_overrides", "container_name") {
		t.Fatalf("expected legacy upgrade to add container_intel_overrides.container_name")
	}
}

func TestRunUpgradesLegacyScanningColumns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
CREATE TABLE scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  critical INTEGER NOT NULL,
  high INTEGER NOT NULL,
  medium INTEGER NOT NULL,
  low INTEGER NOT NULL,
  unknown INTEGER NOT NULL,
  raw_json TEXT NOT NULL
);
CREATE TABLE malware_scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  infected INTEGER NOT NULL,
  threats_found TEXT NOT NULL,
  raw_output TEXT NOT NULL
);
CREATE TABLE scan_jobs (
  job_id TEXT PRIMARY KEY,
  target TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  source TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  completed_at INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		t.Fatalf("create legacy scan tables: %v", err)
	}

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed on legacy scanning schema: %v", err)
	}

	for _, check := range []struct {
		table string
		col   string
	}{
		{"malware_scan_results", "container_name"},
		{"scan_jobs", "container_name"},
		{"scan_jobs", "progress"},
	} {
		if !hasColumn(t, db, check.table, check.col) {
			t.Fatalf("expected legacy upgrade to add %s.%s", check.table, check.col)
		}
	}
}

func TestRunUpgradesLegacyContainerRulesColumns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
CREATE TABLE container_rules (
    container_id TEXT PRIMARY KEY,
    update_policy TEXT DEFAULT 'manual',
    validate_url TEXT DEFAULT '',
    auto_rollback INTEGER DEFAULT 1
);
`)
	if err != nil {
		t.Fatalf("create legacy container_rules table: %v", err)
	}

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run failed on legacy rules schema: %v", err)
	}

	for _, col := range []string{
		"container_name",
		"validate_mode",
		"validate_timeout_sec",
		"validate_interval_sec",
		"bypass_ai",
		"skip_health_check",
		"ai_validate_logs",
		"inherit_automation",
		"upgrades_automation",
		"maintenance_automation",
		"security_automation",
		"restart_on_unhealthy",
		"unhealthy_restart_cooldown_sec",
	} {
		if !hasColumn(t, db, "container_rules", col) {
			t.Fatalf("expected legacy upgrade to add container_rules.%s", col)
		}
	}
}
