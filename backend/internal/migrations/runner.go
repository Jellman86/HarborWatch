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
