package containerintel

import (
	"context"
	"database/sql"
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
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS container_intel_overrides (
	container_id TEXT PRIMARY KEY,
	container_name TEXT NOT NULL DEFAULT '',
	repository_url TEXT NOT NULL DEFAULT '',
	changelog_url TEXT NOT NULL DEFAULT '',
	updated_at INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		return err
	}
	return s.ensureColumn(ctx, "container_name", "TEXT NOT NULL DEFAULT ''")
}

func (s *Store) ensureColumn(ctx context.Context, columnName, columnDDL string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(container_intel_overrides)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, columnName) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, "ALTER TABLE container_intel_overrides ADD COLUMN "+columnName+" "+columnDDL)
	return err
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
