package containerintel

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type Override struct {
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	RepositoryURL string `json:"repositoryUrl"`
	ChangelogURL  string `json:"changelogUrl"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='container_intel_overrides' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("container_intel_overrides table missing; run schema migrations before container intel store init")
	}
	if err != nil {
		return fmt.Errorf("verify container_intel_overrides table: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id, name string) (Override, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	row := s.db.QueryRowContext(ctx, `
SELECT container_id, container_name, repository_url, changelog_url, updated_at
FROM container_intel_overrides
WHERE container_id = ? OR (container_name = ? AND container_name != '')
ORDER BY updated_at DESC
LIMIT 1
`, id, name)

	out := Override{ContainerID: id, ContainerName: name}
	if err := row.Scan(&out.ContainerID, &out.ContainerName, &out.RepositoryURL, &out.ChangelogURL, &out.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return out, nil
		}
		return Override{}, err
	}
	return out, nil
}

func (s *Store) List(ctx context.Context) ([]Override, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT container_id, container_name, repository_url, changelog_url, updated_at
FROM container_intel_overrides
ORDER BY container_id ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Override{}
	for rows.Next() {
		var item Override
		if err := rows.Scan(&item.ContainerID, &item.ContainerName, &item.RepositoryURL, &item.ChangelogURL, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Save(ctx context.Context, item Override) error {
	item.ContainerID = strings.TrimSpace(item.ContainerID)
	item.ContainerName = strings.TrimSpace(item.ContainerName)
	item.RepositoryURL = strings.TrimSpace(item.RepositoryURL)
	item.ChangelogURL = strings.TrimSpace(item.ChangelogURL)

	if item.ContainerID == "" {
		return sql.ErrNoRows
	}

	if item.RepositoryURL == "" && item.ChangelogURL == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM container_intel_overrides WHERE container_id = ?`, item.ContainerID)
		return err
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO container_intel_overrides(container_id, container_name, repository_url, changelog_url, updated_at)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(container_id) DO UPDATE SET
	container_name = excluded.container_name,
	repository_url = excluded.repository_url,
	changelog_url = excluded.changelog_url,
	updated_at = excluded.updated_at
`, item.ContainerID, item.ContainerName, item.RepositoryURL, item.ChangelogURL, item.UpdatedAt)
	return err
}
