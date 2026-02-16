package updates

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	// 1. Initial table creation
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS update_runs (
  id TEXT PRIMARY KEY,
  target_image TEXT NOT NULL,
  validate_url TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS update_steps (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  step TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL,
  ts INTEGER NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("init update tables: %w", err)
	}

	// 2. Migration: Add container_id if it doesn't exist (v0.5.0)
	var hasContainerID bool
	err = s.db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('update_runs') WHERE name='container_id'").Scan(&hasContainerID)
	if err == nil && !hasContainerID {
		_, _ = s.db.ExecContext(ctx, "ALTER TABLE update_runs ADD COLUMN container_id TEXT NOT NULL DEFAULT ''")
	}

	// 3. Migration: Add ai_analysis if it doesn't exist (v0.6.0)
	var hasAIAnalysis bool
	err = s.db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('update_runs') WHERE name='ai_analysis'").Scan(&hasAIAnalysis)
	if err == nil && !hasAIAnalysis {
		_, _ = s.db.ExecContext(ctx, "ALTER TABLE update_runs ADD COLUMN ai_analysis TEXT NOT NULL DEFAULT ''")
	}

	return nil
}

func (s *Store) CreateRun(ctx context.Context, run gen.UpdateJobStatus) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO update_runs(id, container_id, target_image, validate_url, status, created_at, updated_at, error)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
`, run.JobID, run.ContainerID, run.TargetImage, run.ValidateURL, run.Status, run.CreatedAt, run.UpdatedAt, run.Error)
	if err != nil {
		return fmt.Errorf("create update run: %w", err)
	}
	return nil
}

func (s *Store) UpdateRunStatus(ctx context.Context, jobID, status, errMsg string, updatedAt int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE update_runs SET status=?, updated_at=?, error=? WHERE id=?
`, status, updatedAt, errMsg, jobID)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return nil
}

func (s *Store) SaveAIAnalysis(ctx context.Context, jobID string, summary *gen.AIAnalysisSummary) error {
	data, _ := json.Marshal(summary)
	_, err := s.db.ExecContext(ctx, `UPDATE update_runs SET ai_analysis=? WHERE id=?`, string(data), jobID)
	return err
}

func (s *Store) AddStep(ctx context.Context, e gen.UpdateStepEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO update_steps(run_id, step, status, message, ts)
VALUES(?, ?, ?, ?, ?)
`, e.JobID, e.Step, e.Status, e.Message, e.Timestamp)
	if err != nil {
		return fmt.Errorf("insert update step: %w", err)
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, container_id, target_image, validate_url, status, created_at, updated_at, error, ai_analysis
FROM update_runs WHERE id=?
`, jobID)

	var run gen.UpdateJobStatus
	var aiRaw string
	if err := row.Scan(&run.JobID, &run.ContainerID, &run.TargetImage, &run.ValidateURL, &run.Status, &run.CreatedAt, &run.UpdatedAt, &run.Error, &aiRaw); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("read update run: %w", err)
	}

	if aiRaw != "" {
		var summary gen.AIAnalysisSummary
		if err := json.Unmarshal([]byte(aiRaw), &summary); err == nil {
			run.AIAnalysis = &summary
		}
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT step, status, message, ts FROM update_steps WHERE run_id=? ORDER BY id ASC
`, jobID)
	if err != nil {
		return nil, fmt.Errorf("read update steps: %w", err)
	}
	defer rows.Close()

	steps := []gen.UpdateStepEvent{}
	for rows.Next() {
		var e gen.UpdateStepEvent
		e.JobID = jobID
		if err := rows.Scan(&e.Step, &e.Status, &e.Message, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan update step: %w", err)
		}
		steps = append(steps, e)
	}
	run.Steps = steps
	return &run, nil
}
