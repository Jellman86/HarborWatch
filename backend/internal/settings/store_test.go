package settings

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	return store
}

func TestGetDefaultsPrepopulateHarborWatchIgnore(t *testing.T) {
	store := newTestStore(t)
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !strings.Contains(strings.ToLower(got.AutomationIgnoredContainers), "harborwatch") {
		t.Fatalf("expected harborwatch in automation ignores, got %q", got.AutomationIgnoredContainers)
	}
}

func TestSaveNormalizesIgnoreLists(t *testing.T) {
	store := newTestStore(t)
	err := store.Save(context.Background(), Settings{
		AutomationIgnoredContainers: "plex, harborwatch, PLEX",
		MalwareIgnoredMounts:        "/mnt/media,\n/mnt/media ; /srv/plex",
	})
	if err != nil {
		t.Fatalf("save settings: %v", err)
	}

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}

	lowered := strings.ToLower(got.AutomationIgnoredContainers)
	if !strings.Contains(lowered, "harborwatch") {
		t.Fatalf("expected harborwatch in automation ignores, got %q", got.AutomationIgnoredContainers)
	}
	if strings.Count(lowered, "plex") != 1 {
		t.Fatalf("expected deduplicated plex token, got %q", got.AutomationIgnoredContainers)
	}
	if strings.Count(strings.ToLower(got.MalwareIgnoredMounts), "/mnt/media") != 1 {
		t.Fatalf("expected deduplicated mount patterns, got %q", got.MalwareIgnoredMounts)
	}
}
