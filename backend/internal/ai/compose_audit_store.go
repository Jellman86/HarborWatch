package ai

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrComposeAuditNotFound = errors.New("compose audit record not found")

type ComposeAuditRecord struct {
	ID               string `json:"id"`
	ContainerID      string `json:"containerId"`
	ContainerName    string `json:"containerName,omitempty"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Headline         string `json:"headline,omitempty"`
	ComposeConfig    string `json:"config"`
	AnalysisMarkdown string `json:"analysisMarkdown"`
	AnalysisHTML     string `json:"analysisHtml"`
	CreatedAt        int64  `json:"createdAt"`
}

type ComposeAuditRecordSummary struct {
	ID            string `json:"id"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName,omitempty"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Headline      string `json:"headline,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
}

type ComposeAuditStore interface {
	SaveComposeAudit(ctx context.Context, rec ComposeAuditRecord) (ComposeAuditRecord, error)
	ListComposeAudits(ctx context.Context, containerID string, limit, offset int) ([]ComposeAuditRecordSummary, error)
	GetComposeAudit(ctx context.Context, id string) (ComposeAuditRecord, error)
}

type ComposeAuditSQLiteStore struct {
	db *sql.DB
}

func NewComposeAuditSQLiteStore(db *sql.DB) *ComposeAuditSQLiteStore {
	return &ComposeAuditSQLiteStore{db: db}
}

func (s *ComposeAuditSQLiteStore) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS compose_audit_history (
	id TEXT PRIMARY KEY,
	container_id TEXT NOT NULL,
	container_name TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	headline TEXT NOT NULL DEFAULT '',
	compose_config TEXT NOT NULL,
	analysis_markdown TEXT NOT NULL,
	analysis_html TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_compose_audit_container_created ON compose_audit_history(container_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_compose_audit_created ON compose_audit_history(created_at DESC);
`)
	return err
}

func (s *ComposeAuditSQLiteStore) SaveComposeAudit(ctx context.Context, rec ComposeAuditRecord) (ComposeAuditRecord, error) {
	rec.ID = strings.TrimSpace(rec.ID)
	if rec.ID == "" {
		rec.ID = newComposeAuditID()
	}
	rec.ContainerID = strings.TrimSpace(rec.ContainerID)
	rec.ContainerName = strings.TrimSpace(rec.ContainerName)
	rec.Provider = strings.ToLower(strings.TrimSpace(rec.Provider))
	rec.Model = strings.TrimSpace(rec.Model)
	rec.ComposeConfig = strings.TrimSpace(rec.ComposeConfig)
	rec.AnalysisMarkdown = strings.TrimSpace(rec.AnalysisMarkdown)
	rec.AnalysisHTML = strings.TrimSpace(rec.AnalysisHTML)
	rec.Headline = strings.TrimSpace(rec.Headline)
	if rec.Headline == "" {
		rec.Headline = deriveComposeAuditHeadline(rec.AnalysisMarkdown)
	}
	if rec.CreatedAt <= 0 {
		rec.CreatedAt = time.Now().UTC().Unix()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO compose_audit_history(
	id, container_id, container_name, provider, model, headline, compose_config, analysis_markdown, analysis_html, created_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, rec.ID, rec.ContainerID, rec.ContainerName, rec.Provider, rec.Model, rec.Headline, rec.ComposeConfig, rec.AnalysisMarkdown, rec.AnalysisHTML, rec.CreatedAt)
	if err != nil {
		return ComposeAuditRecord{}, fmt.Errorf("insert compose audit history: %w", err)
	}
	return rec, nil
}

func (s *ComposeAuditSQLiteStore) ListComposeAudits(ctx context.Context, containerID string, limit, offset int) ([]ComposeAuditRecordSummary, error) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return []ComposeAuditRecordSummary{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, container_id, container_name, provider, model, headline, created_at
FROM compose_audit_history
WHERE container_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?
`, containerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query compose audit history: %w", err)
	}
	defer rows.Close()

	records := make([]ComposeAuditRecordSummary, 0, limit)
	for rows.Next() {
		var rec ComposeAuditRecordSummary
		if err := rows.Scan(&rec.ID, &rec.ContainerID, &rec.ContainerName, &rec.Provider, &rec.Model, &rec.Headline, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan compose audit history: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate compose audit history: %w", err)
	}
	return records, nil
}

func (s *ComposeAuditSQLiteStore) GetComposeAudit(ctx context.Context, id string) (ComposeAuditRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ComposeAuditRecord{}, ErrComposeAuditNotFound
	}

	var rec ComposeAuditRecord
	err := s.db.QueryRowContext(ctx, `
SELECT id, container_id, container_name, provider, model, headline, compose_config, analysis_markdown, analysis_html, created_at
FROM compose_audit_history
WHERE id = ?
`, id).Scan(
		&rec.ID,
		&rec.ContainerID,
		&rec.ContainerName,
		&rec.Provider,
		&rec.Model,
		&rec.Headline,
		&rec.ComposeConfig,
		&rec.AnalysisMarkdown,
		&rec.AnalysisHTML,
		&rec.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ComposeAuditRecord{}, ErrComposeAuditNotFound
		}
		return ComposeAuditRecord{}, fmt.Errorf("query compose audit record: %w", err)
	}
	return rec, nil
}

func newComposeAuditID() string {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("ca_%d", time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("ca_%d_%s", time.Now().UTC().UnixNano(), hex.EncodeToString(suffix[:]))
}

func deriveComposeAuditHeadline(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#-*>\t ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 140 {
			return trimmed[:140]
		}
		return trimmed
	}
	return "Compose Audit"
}
