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

func NewServiceFromEnv() (*Service, error) {
	dbPath := os.Getenv("HARBORWATCH_DB_PATH")
	if dbPath == "" {
		dbPath = "/tmp/harborwatch.db"
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return NewService(db), nil
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
