package scanning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	// 1. Scan Results table
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

	// 2. Malware results table (v0.5.0)
	_, err = s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS malware_scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  infected INTEGER NOT NULL,
  threats_found TEXT NOT NULL,
  raw_output TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("create malware_scan_results table: %w", err)
	}

	// 3. Scan jobs table (v0.5.0)
	_, err = s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS scan_jobs (
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
		return fmt.Errorf("create scan_jobs table: %w", err)
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

func (s *Store) SaveMalwareResult(ctx context.Context, r MalwareResult) error {
	threats, _ := json.Marshal(r.FoundThreats)
	infected := 0
	if r.Infected {
		infected = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO malware_scan_results(target, source, scanned_at, infected, threats_found, raw_output)
VALUES(?, ?, ?, ?, ?, ?)
`, r.Target, r.Source, r.ScannedAt, infected, string(threats), r.RawOutput)
	if err != nil {
		return fmt.Errorf("insert malware scan result: %w", err)
	}
	return nil
}

func (s *Store) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	query := `SELECT target, source, scanned_at, infected, threats_found FROM malware_scan_results`
	var args []any
	if target != "" {
		query += ` WHERE target = ?`
		args = append(args, target)
	}
	query += ` ORDER BY scanned_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query malware summaries: %w", err)
	}
	defer rows.Close()

	var summaries []gen.MalwareScanSummary
	for rows.Next() {
		var sm gen.MalwareScanSummary
		var infected int
		var threatsRaw string
		if err := rows.Scan(&sm.Target, &sm.Source, &sm.ScannedAt, &infected, &threatsRaw); err != nil {
			return nil, fmt.Errorf("scan malware summary: %w", err)
		}
		sm.Infected = infected == 1
		_ = json.Unmarshal([]byte(threatsRaw), &sm.ThreatsFound)
		summaries = append(summaries, sm)
	}
	return summaries, nil
}

func (s *Store) CreateJob(ctx context.Context, job gen.ScanJobStatus, scanType string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scan_jobs(job_id, target, type, status, source, started_at)
VALUES(?, ?, ?, ?, ?, ?)
`, job.JobID, job.Target, scanType, job.Status, job.Source, job.StartedAt)
	if err != nil {
		return fmt.Errorf("create scan job: %w", err)
	}
	return nil
}

func (s *Store) UpdateJob(ctx context.Context, jobID, status, errMsg string, completedAt int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE scan_jobs SET status=?, error=?, completed_at=? WHERE job_id=?
`, status, errMsg, completedAt, jobID)
	if err != nil {
		return fmt.Errorf("update scan job: %w", err)
	}
	return nil
}

func (s *Store) GetJob(ctx context.Context, jobID string) (*gen.ScanJobStatus, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT job_id, target, status, source, error, started_at, completed_at
FROM scan_jobs WHERE job_id=?
`, jobID)

	var job gen.ScanJobStatus
	if err := row.Scan(&job.JobID, &job.Target, &job.Status, &job.Source, &job.Error, &job.StartedAt, &job.CompletedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("read scan job: %w", err)
	}
	return &job, nil
}

func (s *Store) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	return s.LatestSummaryForTarget(ctx, "")
}

func (s *Store) LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error) {
	query := `
SELECT target, source, scanned_at, critical, high, medium, low, unknown
FROM scan_results
`
	var args []any
	if target != "" {
		query += " WHERE target = ?"
		args = append(args, target)
	}
	query += " ORDER BY scanned_at DESC, id DESC LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)

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
