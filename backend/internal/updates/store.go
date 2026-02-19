package updates

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
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

func (s *Store) ListRunsForContainer(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	if strings.TrimSpace(containerID) == "" {
		return []gen.UpdateJobStatus{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, container_id, target_image, validate_url, status, created_at, updated_at, error, ai_analysis
FROM update_runs
WHERE container_id = ?
ORDER BY updated_at DESC
LIMIT ?
`, strings.TrimSpace(containerID), limit)
	if err != nil {
		return nil, fmt.Errorf("list update runs: %w", err)
	}
	defer rows.Close()

	out := []gen.UpdateJobStatus{}
	for rows.Next() {
		var item gen.UpdateJobStatus
		var aiRaw string
		if err := rows.Scan(&item.JobID, &item.ContainerID, &item.TargetImage, &item.ValidateURL, &item.Status, &item.CreatedAt, &item.UpdatedAt, &item.Error, &aiRaw); err != nil {
			return nil, fmt.Errorf("scan update run: %w", err)
		}
		if aiRaw != "" {
			var summary gen.AIAnalysisSummary
			if err := json.Unmarshal([]byte(aiRaw), &summary); err == nil {
				item.AIAnalysis = &summary
			}
		}
		item.Steps = []gen.UpdateStepEvent{}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) PruneRuns(ctx context.Context, olderThan int64) (int64, int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin prune update runs tx: %w", err)
	}
	defer tx.Rollback()

	stepRes, err := tx.ExecContext(ctx, `
DELETE FROM update_steps
WHERE run_id IN (SELECT id FROM update_runs WHERE updated_at < ?)
`, olderThan)
	if err != nil {
		return 0, 0, fmt.Errorf("delete update steps: %w", err)
	}

	runRes, err := tx.ExecContext(ctx, "DELETE FROM update_runs WHERE updated_at < ?", olderThan)
	if err != nil {
		return 0, 0, fmt.Errorf("delete update runs: %w", err)
	}

	stepRows, err := stepRes.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("update step rows affected: %w", err)
	}
	runRows, err := runRes.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("update run rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit prune update runs tx: %w", err)
	}
	return runRows, stepRows, nil
}
