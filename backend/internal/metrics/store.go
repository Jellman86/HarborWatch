package metrics

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Metric struct {
	ContainerID string  `json:"containerId"`
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsage int64   `json:"memoryUsage"`
	MemoryLimit int64   `json:"memoryLimit"`
	Pids        int     `json:"pids"`
}

type Store struct {
	db *sql.DB
}

func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS container_metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  container_id TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  cpu_percent REAL NOT NULL,
  memory_usage INTEGER NOT NULL,
  memory_limit INTEGER NOT NULL,
  pids INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metrics_container_ts ON container_metrics(container_id, timestamp);
`)
	if err != nil {
		return fmt.Errorf("init metrics tables: %w", err)
	}
	return nil
}

func (s *Store) SaveMetric(ctx context.Context, m Metric) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO container_metrics(container_id, timestamp, cpu_percent, memory_usage, memory_limit, pids)
VALUES(?, ?, ?, ?, ?, ?)
`, m.ContainerID, m.Timestamp, m.CPUPercent, m.MemoryUsage, m.MemoryLimit, m.Pids)
	return err
}

func (s *Store) GetMetrics(ctx context.Context, containerID string, since int64) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT container_id, timestamp, cpu_percent, memory_usage, memory_limit, pids
FROM container_metrics
WHERE container_id = ? AND timestamp > ?
ORDER BY timestamp ASC
`, containerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ContainerID, &m.Timestamp, &m.CPUPercent, &m.MemoryUsage, &m.MemoryLimit, &m.Pids); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

func (s *Store) PruneMetrics(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM container_metrics WHERE timestamp < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
