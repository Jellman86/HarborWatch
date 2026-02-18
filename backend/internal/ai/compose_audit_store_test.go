package ai

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestComposeAuditSQLiteStore_SaveListGet(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "compose_audit.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewComposeAuditSQLiteStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}

	saved, err := store.SaveComposeAudit(context.Background(), ComposeAuditRecord{
		ContainerID:      "c1",
		ContainerName:    "frigate",
		Provider:         "openai",
		Model:            "gpt-5.2",
		ComposeConfig:    "services:\n  app:\n    image: test",
		AnalysisMarkdown: "# Findings\n\n- Tighten privileges",
		AnalysisHTML:     "<h1>Findings</h1><ul><li>Tighten privileges</li></ul>",
	})
	if err != nil {
		t.Fatalf("save compose audit: %v", err)
	}
	if saved.ID == "" {
		t.Fatalf("expected generated id")
	}
	if saved.Headline == "" {
		t.Fatalf("expected headline to be derived")
	}

	list, err := store.ListComposeAudits(context.Background(), "c1", 10, 0)
	if err != nil {
		t.Fatalf("list compose audits: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(list))
	}
	if list[0].ID != saved.ID {
		t.Fatalf("expected list id %q, got %q", saved.ID, list[0].ID)
	}

	got, err := store.GetComposeAudit(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("get compose audit: %v", err)
	}
	if got.ComposeConfig == "" || got.AnalysisHTML == "" || got.AnalysisMarkdown == "" {
		t.Fatalf("expected stored config/analysis payload")
	}
}
