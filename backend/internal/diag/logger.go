package diag

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type SystemStatus struct {
	Uptime       int64             `json:"uptime"`
	Version      string            `json:"version"`
	MemoryAlloc  uint64            `json:"memoryAlloc"`
	NumGoroutine int               `json:"numGoroutine"`
	DBSize       int64             `json:"dbSize"`
	ActiveJobs   []gen.JobProgress `json:"activeJobs"`
}

type LogEntry struct {
	ID        int64  `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

type Service struct {
	db        *sql.DB
	startTime time.Time
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, startTime: time.Now()}
}

func (s *Service) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='internal_logs' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("internal_logs table missing; run schema migrations before diag service init")
	}
	if err != nil {
		return fmt.Errorf("verify internal_logs table: %w", err)
	}
	return nil
}

func (s *Service) GetSystemStatus() SystemStatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbPath := os.Getenv("HARBORWATCH_DB_PATH")
	var dbSize int64
	if info, err := os.Stat(dbPath); err == nil {
		dbSize = info.Size()
	}

	return SystemStatus{
		Uptime:       int64(time.Since(s.startTime).Seconds()),
		Version:      os.Getenv("HARBORWATCH_VERSION"),
		MemoryAlloc:  m.Alloc,
		NumGoroutine: runtime.NumGoroutine(),
		DBSize:       dbSize,
	}
}

func (s *Service) Log(level, source, message string) {
	// Always log to stdout as well
	log.Printf("[%s] %s: %s", level, source, message)

	_, err := s.db.Exec("INSERT INTO internal_logs (timestamp, level, message, source) VALUES (?, ?, ?, ?)",
		time.Now().Unix(), level, message, source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write to internal_logs: %v\n", err)
	}
}

func (s *Service) ListLogs(ctx context.Context, limit, offset int, level, source, search string, since int64) ([]LogEntry, int, error) {
	where := " WHERE 1=1"
	var args []any

	if level != "" {
		where += " AND level = ?"
		args = append(args, level)
	}
	if source != "" {
		where += " AND source LIKE ?"
		args = append(args, "%"+source+"%")
	}
	if since > 0 {
		where += " AND timestamp >= ?"
		args = append(args, since)
	}
	if search != "" {
		where += " AND (message LIKE ? OR source LIKE ? OR level LIKE ?)"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern)
	}

	// 1. Get total count for these filters
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM internal_logs"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get paged entries
	query := "SELECT id, timestamp, level, message, source FROM internal_logs" + where
	query += " ORDER BY timestamp DESC, id DESC LIMIT ? OFFSET ?"
	pagedArgs := append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Level, &e.Message, &e.Source); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, nil
}

func (s *Service) PruneLogs(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM internal_logs WHERE timestamp < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
