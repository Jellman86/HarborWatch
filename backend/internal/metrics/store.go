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

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='container_metrics' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("container_metrics table missing; run schema migrations before metrics store init")
	}
	if err != nil {
		return fmt.Errorf("verify container_metrics table: %w", err)
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
