package scanning

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS scan_results (
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
`)
	if err != nil {
		return fmt.Errorf("create scan_results table: %w", err)
	}
	return nil
}

func (s *Store) SaveResult(ctx context.Context, r Result) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scan_results(target, source, scanned_at, critical, high, medium, low, unknown, raw_json)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.Target, r.Source, r.Scanned, r.Critical, r.High, r.Medium, r.Low, r.Unknown, r.RawJSON)
	if err != nil {
		return fmt.Errorf("insert scan result: %w", err)
	}
	return nil
}

func (s *Store) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT target, source, scanned_at, critical, high, medium, low, unknown
FROM scan_results
ORDER BY scanned_at DESC, id DESC
LIMIT 1
`)

	var summary gen.ScanSummary
	if err := row.Scan(
		&summary.Target,
		&summary.Source,
		&summary.ScannedAt,
		&summary.Critical,
		&summary.High,
		&summary.Medium,
		&summary.Low,
		&summary.Unknown,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load latest summary: %w", err)
	}

	summary.Total = summary.Critical + summary.High + summary.Medium + summary.Low + summary.Unknown
	summary.RiskScore = riskScore(summary)
	return &summary, nil
}

func riskScore(s gen.ScanSummary) int {
	score := (s.Critical * 10) + (s.High * 7) + (s.Medium * 4) + (s.Low * 2) + s.Unknown
	if score > 100 {
		return 100
	}
	return score
}
