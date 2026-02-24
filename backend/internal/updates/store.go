package updates

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	for _, table := range []string{"update_runs", "update_steps"} {
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("%s table missing; run schema migrations before updates store init", table)
		}
		if err != nil {
			return fmt.Errorf("verify %s table: %w", table, err)
		}
	}
	return nil
}

func (s *Store) CreateRun(ctx context.Context, run gen.UpdateJobStatus) error {
	return s.CreateRunWithContainerName(ctx, run, "")
}

func (s *Store) CreateRunWithContainerName(ctx context.Context, run gen.UpdateJobStatus, containerName string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO update_runs(id, container_id, container_name, target_image, validate_url, status, progress, created_at, updated_at, error)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, run.JobID, strings.TrimSpace(run.ContainerID), strings.TrimSpace(containerName), run.TargetImage, run.ValidateURL, run.Status, run.Progress, run.CreatedAt, run.UpdatedAt, run.Error)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "idx_update_runs_one_running_per_container") || strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
			return fmt.Errorf("create update run: container %s already has a running update job", strings.TrimSpace(run.ContainerID))
		}
		return fmt.Errorf("create update run: %w", err)
	}
	return nil
}

func (s *Store) UpdateRunStatus(ctx context.Context, jobID, status, errMsg string, updatedAt int64) error {
	progress := 0
	if status == "completed" {
		progress = 100
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE update_runs SET status=?, updated_at=?, error=?, progress=MAX(progress, ?) WHERE id=?
`, status, updatedAt, errMsg, progress, jobID)
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	return nil
}

func (s *Store) UpdateRunProgress(ctx context.Context, jobID, status string, progress int) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE update_runs SET status=?, progress=? WHERE id=?
`, status, progress, jobID)
	if err != nil {
		return fmt.Errorf("update run progress: %w", err)
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
SELECT id, container_id, target_image, validate_url, status, progress, created_at, updated_at, error, ai_analysis
FROM update_runs WHERE id=?
`, jobID)

	var run gen.UpdateJobStatus
	var aiRaw string
	if err := row.Scan(&run.JobID, &run.ContainerID, &run.TargetImage, &run.ValidateURL, &run.Status, &run.Progress, &run.CreatedAt, &run.UpdatedAt, &run.Error, &aiRaw); err != nil {
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
SELECT id, container_id, target_image, validate_url, status, progress, created_at, updated_at, error, ai_analysis
FROM update_runs
WHERE container_id = ? OR container_name = ?
ORDER BY updated_at DESC
LIMIT ?
`, strings.TrimSpace(containerID), strings.TrimSpace(containerID), limit)
	if err != nil {
		return nil, fmt.Errorf("list update runs: %w", err)
	}
	defer rows.Close()

	out := []gen.UpdateJobStatus{}
	for rows.Next() {
		var item gen.UpdateJobStatus
		var aiRaw string
		if err := rows.Scan(&item.JobID, &item.ContainerID, &item.TargetImage, &item.ValidateURL, &item.Status, &item.Progress, &item.CreatedAt, &item.UpdatedAt, &item.Error, &aiRaw); err != nil {
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

func (s *Store) ListActiveRuns(ctx context.Context) ([]gen.UpdateJobStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.container_id, r.target_image, r.validate_url, r.status, r.progress, r.created_at, r.updated_at, r.error, r.ai_analysis,
       (SELECT message FROM update_steps WHERE run_id = r.id ORDER BY id DESC LIMIT 1) as last_message
FROM update_runs r
WHERE r.status = 'running' OR r.status = 'queued'
ORDER BY r.created_at DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list active update runs: %w", err)
	}
	defer rows.Close()

	out := []gen.UpdateJobStatus{}
	for rows.Next() {
		var item gen.UpdateJobStatus
		var aiRaw string
		var lastMsg sql.NullString
		if err := rows.Scan(&item.JobID, &item.ContainerID, &item.TargetImage, &item.ValidateURL, &item.Status, &item.Progress, &item.CreatedAt, &item.UpdatedAt, &item.Error, &aiRaw, &lastMsg); err != nil {
			return nil, fmt.Errorf("scan update run row: %w", err)
		}
		if aiRaw != "" {
			var summary gen.AIAnalysisSummary
			if err := json.Unmarshal([]byte(aiRaw), &summary); err == nil {
				item.AIAnalysis = &summary
			}
		}
		item.Message = lastMsg.String
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

func (s *Store) MarkRunningRunsFailed(ctx context.Context, reason string) (int64, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "process interrupted by system restart"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE update_runs
SET status = 'failed', error = ?, updated_at = ?
WHERE status = 'running'
`, reason, time.Now().UTC().Unix())
	if err != nil {
		return 0, fmt.Errorf("mark running update runs failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected running update runs: %w", err)
	}
	return n, nil
}
