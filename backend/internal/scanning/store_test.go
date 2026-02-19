package scanning

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newStoreTestDB(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	return store
}

func TestMalwareSummariesByPrefix_MatchesShortContainerID(t *testing.T) {
	store := newStoreTestDB(t)
	now := time.Now().UTC().Unix()
	fullID := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	if err := store.SaveMalwareResult(context.Background(), MalwareResult{
		Target:    "container:" + fullID + ":mount:/config",
		Source:    "clamav",
		ScannedAt: now,
	}); err != nil {
		t.Fatalf("save malware result: %v", err)
	}

	rows, err := store.MalwareSummariesByPrefix(context.Background(), "container:abcdef123456")
	if err != nil {
		t.Fatalf("query summaries by prefix: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 summary for short id prefix, got %d", len(rows))
	}
}

func TestMalwareDetails_MatchesShortContainerIDPrefix(t *testing.T) {
	store := newStoreTestDB(t)
	now := time.Now().UTC().Unix()
	fullID := "123456abcdef7890123456abcdef7890123456abcdef7890123456abcdef7890"

	if err := store.SaveMalwareResult(context.Background(), MalwareResult{
		Target:    "container:" + fullID + ":rootfs",
		Source:    "clamav",
		ScannedAt: now,
	}); err != nil {
		t.Fatalf("save malware result: %v", err)
	}

	rows, err := store.MalwareDetails(context.Background(), "", "container:123456abcdef", 25)
	if err != nil {
		t.Fatalf("query details by prefix: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 detail for short id prefix, got %d", len(rows))
	}
}
