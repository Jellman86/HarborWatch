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

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='schedules' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("schedules table missing; run schema migrations before scheduler store init")
	}
	if err != nil {
		return fmt.Errorf("verify schedules table: %w", err)
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

func (s *Store) UpdateScheduleSpec(ctx context.Context, id, cronSpec string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE schedules SET cron_spec=? WHERE id=?", cronSpec, id)
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

func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM schedules WHERE id=?", id)
	return err
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
