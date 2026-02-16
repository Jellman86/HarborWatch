package scheduler

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type ScheduleEntry struct {
	ID       string `json:"id"`
	CronSpec string `json:"cronSpec"`
	Enabled  bool   `json:"enabled"`
	LastRun  int64  `json:"lastRun"`
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
	// 1. Initial table creation
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schedules (
  id TEXT PRIMARY KEY,
  cron_spec TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  last_run INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		return fmt.Errorf("init scheduler tables: %w", err)
	}

	return nil
}

func (s *Store) SaveSchedule(ctx context.Context, entry ScheduleEntry) error {
	val := 0
	if entry.Enabled {
		val = 1
	}
	// We use INSERT OR IGNORE to ensure we don't overwrite user-toggled states 
	// during the boot-time task registration.
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO schedules(id, cron_spec, enabled)
VALUES(?, ?, ?)
`, entry.ID, entry.CronSpec, val)
	return err
}

func (s *Store) ToggleSchedule(ctx context.Context, id string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.ExecContext(ctx, "UPDATE schedules SET enabled=? WHERE id=?", val, id)
	return err
}

func (s *Store) ListSchedules(ctx context.Context) ([]ScheduleEntry, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, cron_spec, enabled, last_run FROM schedules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScheduleEntry
	for rows.Next() {
		var e ScheduleEntry
		var enabled int
		if err := rows.Scan(&e.ID, &e.CronSpec, &enabled, &e.LastRun); err != nil {
			return nil, err
		}
		e.Enabled = enabled == 1
		list = append(list, e)
	}
	return list, nil
}

func (s *Store) GetSchedule(ctx context.Context, id string) (ScheduleEntry, error) {
	var e ScheduleEntry
	var enabled int
	err := s.db.QueryRowContext(ctx, "SELECT id, cron_spec, enabled, last_run FROM schedules WHERE id=?", id).Scan(&e.ID, &e.CronSpec, &enabled, &e.LastRun)
	if err != nil {
		return ScheduleEntry{}, err
	}
	e.Enabled = enabled == 1
	return e, nil
}

func (s *Store) UpdateLastRun(ctx context.Context, id string, ts int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE schedules SET last_run=? WHERE id=?", ts, id)
	return err
}
