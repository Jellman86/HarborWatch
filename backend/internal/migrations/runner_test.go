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

func TestRunAppliesBaselineAndReportsCurrentVersion(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	result, err := Run(ctx, db)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.CurrentVersion != 1 {
		t.Fatalf("expected current version 1, got %d", result.CurrentVersion)
	}
	if result.AppliedCount != 1 {
		t.Fatalf("expected 1 applied migration, got %d", result.AppliedCount)
	}

	version, err := CurrentVersion(ctx, db)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected CurrentVersion=1, got %d", version)
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
	if result.CurrentVersion != 1 {
		t.Fatalf("expected current version 1 after rerun, got %d", result.CurrentVersion)
	}

	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&rows); err != nil {
		t.Fatalf("count schema_migrations rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 schema migration row, got %d", rows)
	}
}
