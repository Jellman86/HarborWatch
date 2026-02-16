package audit

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error) {
	query := `
SELECT id, 'Update' as type, target_image as target, status, error, created_at as started_at, updated_at as completed_at
FROM update_runs
UNION ALL
SELECT job_id as id, type, target, status, error, started_at, completed_at
FROM scan_jobs
ORDER BY started_at DESC
LIMIT 100
`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query audit jobs: %w", err)
	}
	defer rows.Close()

	var jobs []gen.AuditJobSummary
	for rows.Next() {
		var j gen.AuditJobSummary
		var compAt sql.NullInt64
		if err := rows.Scan(&j.ID, &j.Type, &j.Target, &j.Status, &j.Error, &j.StartedAt, &compAt); err != nil {
			return nil, fmt.Errorf("scan audit job: %w", err)
		}
		if compAt.Valid {
			j.CompletedAt = compAt.Int64
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Service) ListAuditJobsForContainer(ctx context.Context, containerID string) ([]gen.AuditJobSummary, error) {
	query := `
SELECT id, 'Update' as type, target_image as target, container_id, status, error, created_at as started_at, updated_at as completed_at
FROM update_runs
WHERE container_id = ?
UNION ALL
SELECT job_id as id, type, target, '' as container_id, status, error, started_at, completed_at
FROM scan_jobs
WHERE target = (SELECT image FROM containers WHERE id = ? LIMIT 1) OR target = ?
ORDER BY started_at DESC
LIMIT 50
`
	rows, err := s.db.QueryContext(ctx, query, containerID, containerID, containerID)
	if err != nil {
		return nil, fmt.Errorf("query container audit jobs: %w", err)
	}
	defer rows.Close()

	var jobs []gen.AuditJobSummary
	for rows.Next() {
		var j gen.AuditJobSummary
		var compAt sql.NullInt64
		if err := rows.Scan(&j.ID, &j.Type, &j.Target, &j.ContainerID, &j.Status, &j.Error, &j.StartedAt, &compAt); err != nil {
			return nil, fmt.Errorf("scan audit job: %w", err)
		}
		if compAt.Valid {
			j.CompletedAt = compAt.Int64
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Service) GetAuditJobSteps(ctx context.Context, id string) ([]gen.UpdateStepEvent, error) {
	// For now, we only have detailed step logs for Updates. 
	// Scans are atomic jobs without sub-steps in the DB currently.
	rows, err := s.db.QueryContext(ctx, `
SELECT step, status, message, ts FROM update_steps WHERE run_id=? ORDER BY id ASC
`, id)
	if err != nil {
		return nil, fmt.Errorf("query job steps: %w", err)
	}
	defer rows.Close()

	var steps []gen.UpdateStepEvent
	for rows.Next() {
		var e gen.UpdateStepEvent
		e.JobID = id
		if err := rows.Scan(&e.Step, &e.Status, &e.Message, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan job step: %w", err)
		}
		steps = append(steps, e)
	}
	return steps, nil
}
