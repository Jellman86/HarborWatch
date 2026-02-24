package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed sql/*.sql
var sqlFS embed.FS

type Migration struct {
	Version int
	Name    string
	SQL     string
	ApplyTx func(ctx context.Context, tx *sql.Tx) error
}

func mustReadSQL(path string) string {
	b, err := sqlFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("read embedded migration %s: %v", path, err))
	}
	return string(b)
}

func defaultMigrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "legacy_schema_baseline",
			SQL:     mustReadSQL("sql/0001_legacy_schema_baseline.sql"),
		},
		{
			Version: 2,
			Name:    "create_app_settings",
			SQL:     mustReadSQL("sql/0002_create_app_settings.sql"),
		},
		{
			Version: 3,
			Name:    "create_container_rules",
			SQL:     mustReadSQL("sql/0003_create_container_rules.sql"),
		},
		{
			Version: 4,
			Name:    "upgrade_container_rules_columns",
			ApplyTx: migrateContainerRulesColumns,
		},
		{
			Version: 5,
			Name:    "create_update_tables",
			SQL:     mustReadSQL("sql/0005_create_update_tables.sql"),
		},
		{
			Version: 6,
			Name:    "upgrade_update_runs_columns_and_indexes",
			ApplyTx: migrateUpdateRunsColumnsAndIndexes,
		},
		{
			Version: 7,
			Name:    "best_effort_unique_active_update_index",
			ApplyTx: migrateUniqueActiveUpdateIndexBestEffort,
		},
		{
			Version: 8,
			Name:    "create_scanning_tables",
			SQL:     mustReadSQL("sql/0008_create_scanning_tables.sql"),
		},
		{
			Version: 9,
			Name:    "upgrade_scanning_columns_and_indexes",
			ApplyTx: migrateScanningColumnsAndIndexes,
		},
		{
			Version: 10,
			Name:    "create_metrics_table",
			SQL:     mustReadSQL("sql/0010_create_metrics_table.sql"),
		},
		{
			Version: 11,
			Name:    "create_scheduler_table",
			SQL:     mustReadSQL("sql/0011_create_scheduler_table.sql"),
		},
		{
			Version: 12,
			Name:    "create_internal_logs_table",
			SQL:     mustReadSQL("sql/0012_create_internal_logs_table.sql"),
		},
		{
			Version: 13,
			Name:    "create_remediation_runs_table",
			SQL:     mustReadSQL("sql/0013_create_remediation_runs_table.sql"),
		},
		{
			Version: 14,
			Name:    "create_container_intel_overrides_table",
			SQL:     mustReadSQL("sql/0014_create_container_intel_overrides_table.sql"),
		},
		{
			Version: 15,
			Name:    "upgrade_container_intel_overrides_columns",
			ApplyTx: migrateContainerIntelColumns,
		},
		{
			Version: 16,
			Name:    "create_ai_usage_tables",
			SQL:     mustReadSQL("sql/0016_create_ai_usage_tables.sql"),
		},
		{
			Version: 17,
			Name:    "create_compose_audit_history_table",
			SQL:     mustReadSQL("sql/0017_create_compose_audit_history_table.sql"),
		},
	}
}
