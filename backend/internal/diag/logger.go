package diag

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

type SystemStatus struct {
	Uptime      int64  `json:"uptime"`
	Version     string `json:"version"`
	MemoryAlloc uint64 `json:"memoryAlloc"`
	NumGoroutine int    `json:"numGoroutine"`
	DBSize      int64  `json:"dbSize"`
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
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS internal_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp INTEGER NOT NULL,
  level TEXT NOT NULL,
  message TEXT NOT NULL,
  source TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_logs_ts ON internal_logs(timestamp);
`)
	return err
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

func (s *Service) ListLogs(ctx context.Context, limit int) ([]LogEntry, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, level, message, source FROM internal_logs ORDER BY timestamp DESC, id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Level, &e.Message, &e.Source); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) PruneLogs(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM internal_logs WHERE timestamp < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
