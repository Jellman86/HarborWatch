package ai

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func TestUsageSQLiteStoreInitRequiresMigratedSchema(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "usage-missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	store := NewUsageSQLiteStore(db)
	if err := store.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when ai usage schema is missing")
	}
}

func TestUsageSQLiteStoreSummary(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	store := NewUsageSQLiteStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}

	now := time.Now().UTC().Unix()
	events := []UsageRecord{
		{
			Timestamp:    now - 120,
			Provider:     "openai",
			Model:        "gpt-5.2",
			Feature:      "release_analysis",
			InputTokens:  1000,
			OutputTokens: 200,
			TotalTokens:  1200,
		},
		{
			Timestamp:    now - 60,
			Provider:     "openai",
			Model:        "gpt-5.2",
			Feature:      "health_logs",
			InputTokens:  400,
			OutputTokens: 100,
			TotalTokens:  500,
		},
	}
	for _, item := range events {
		if err := store.RecordUsage(context.Background(), item); err != nil {
			t.Fatalf("record usage: %v", err)
		}
	}

	summary, err := store.SummaryUsage(context.Background(), now-3600, now+10)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Calls != 2 {
		t.Fatalf("expected 2 calls, got %d", summary.Calls)
	}
	if summary.TotalTokens != 1700 {
		t.Fatalf("expected total tokens 1700, got %d", summary.TotalTokens)
	}
	if len(summary.Breakdown) != 2 {
		t.Fatalf("expected 2 breakdown rows, got %d", len(summary.Breakdown))
	}
	if len(summary.Daily) != 1 {
		t.Fatalf("expected 1 daily row, got %d", len(summary.Daily))
	}
	if len(summary.DailyBreakdown) != 1 {
		t.Fatalf("expected 1 daily breakdown row, got %d", len(summary.DailyBreakdown))
	}
}
