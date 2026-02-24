package scanning

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func newStoreTestDB(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	return store
}

func TestInitRequiresMigratedSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when scanning tables are missing")
	}
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

func TestMalwareSummariesByContainer_ExposesContainerName(t *testing.T) {
	store := newStoreTestDB(t)
	now := time.Now().UTC().Unix()

	if err := store.SaveMalwareResult(context.Background(), MalwareResult{
		Target:        "container:abc123:rootfs",
		Source:        "clamav",
		ScannedAt:     now,
		ContainerName: "gluetun",
	}); err != nil {
		t.Fatalf("save malware result: %v", err)
	}

	rows, err := store.MalwareSummariesByContainer(context.Background(), "abc123", "gluetun")
	if err != nil {
		t.Fatalf("query summaries by container: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(rows))
	}
	if rows[0].ContainerName != "gluetun" {
		t.Fatalf("expected container name gluetun, got %q", rows[0].ContainerName)
	}
}

func TestMalwareDetailsForContainer_ExposesContainerName(t *testing.T) {
	store := newStoreTestDB(t)
	now := time.Now().UTC().Unix()

	if err := store.SaveMalwareResult(context.Background(), MalwareResult{
		Target:        "container:def456:mount:/config",
		Source:        "clamav",
		ScannedAt:     now,
		ContainerName: "qbittorrent",
	}); err != nil {
		t.Fatalf("save malware result: %v", err)
	}

	rows, err := store.MalwareDetailsForContainer(context.Background(), "def456", "qbittorrent", 25)
	if err != nil {
		t.Fatalf("query details by container: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(rows))
	}
	if rows[0].ContainerName != "qbittorrent" {
		t.Fatalf("expected container name qbittorrent, got %q", rows[0].ContainerName)
	}
}
