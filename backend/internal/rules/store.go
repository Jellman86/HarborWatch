package rules

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type ContainerRules struct {
	ContainerID  string `json:"containerId"`
	UpdatePolicy string `json:"updatePolicy"` // auto, manual, locked
	ValidateURL  string `json:"validateUrl"`
	AutoRollback bool   `json:"autoRollback"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS container_rules (
    container_id TEXT PRIMARY KEY,
    update_policy TEXT DEFAULT 'manual',
    validate_url TEXT DEFAULT '',
    auto_rollback INTEGER DEFAULT 1
);
`)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (ContainerRules, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT container_id, update_policy, validate_url, auto_rollback 
FROM container_rules WHERE container_id = ?
`, id)

	var r ContainerRules
	var rollback int
	if err := row.Scan(&r.ContainerID, &r.UpdatePolicy, &r.ValidateURL, &rollback); err != nil {
		if err == sql.ErrNoRows {
			return ContainerRules{ContainerID: id, UpdatePolicy: "manual", AutoRollback: true}, nil
		}
		return r, err
	}
	r.AutoRollback = rollback == 1
	return r, nil
}

func (s *Store) Save(ctx context.Context, r ContainerRules) error {
	rollback := 0
	if r.AutoRollback {
		rollback = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO container_rules (container_id, update_policy, validate_url, auto_rollback)
VALUES (?, ?, ?, ?)
ON CONFLICT(container_id) DO UPDATE SET
    update_policy = excluded.update_policy,
    validate_url = excluded.validate_url,
    auto_rollback = excluded.auto_rollback
`, r.ContainerID, r.UpdatePolicy, r.ValidateURL, rollback)
	return err
}
