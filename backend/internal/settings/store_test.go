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

func TestGetAppliesRuntimeEnvOverridesForNumericSettings(t *testing.T) {
	t.Setenv("HW_AI_BLOCK_RISK_THRESHOLD", "65")
	t.Setenv("HW_AUTO_UPGRADE_MAX_CONCURRENCY", "3")
	t.Setenv("HW_AUTO_UPGRADE_MIN_RETRY_MINUTES", "90")
	t.Setenv("HW_CLAMAV_SNAPSHOT_MAX_BYTES", "3221225472")

	store := newTestStore(t)
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.AIBlockRiskThreshold != 65 {
		t.Fatalf("expected AIBlockRiskThreshold=65, got %d", got.AIBlockRiskThreshold)
	}
	if got.AutoUpgradeMaxConcurrency != 3 {
		t.Fatalf("expected AutoUpgradeMaxConcurrency=3, got %d", got.AutoUpgradeMaxConcurrency)
	}
	if got.AutoUpgradeMinRetryMinutes != 90 {
		t.Fatalf("expected AutoUpgradeMinRetryMinutes=90, got %d", got.AutoUpgradeMinRetryMinutes)
	}
	if got.ClamAVSnapshotMaxBytes != 3221225472 {
		t.Fatalf("expected ClamAVSnapshotMaxBytes=3221225472, got %d", got.ClamAVSnapshotMaxBytes)
	}
	if !got.EnvironmentOverrides["aiBlockRiskThreshold"] {
		t.Fatalf("expected aiBlockRiskThreshold override to be locked")
	}
	if !got.EnvironmentOverrides["autoUpgradeMaxConcurrency"] {
		t.Fatalf("expected autoUpgradeMaxConcurrency override to be locked")
	}
	if !got.EnvironmentOverrides["autoUpgradeMinRetryMinutes"] {
		t.Fatalf("expected autoUpgradeMinRetryMinutes override to be locked")
	}
	if !got.EnvironmentOverrides["clamavSnapshotMaxBytes"] {
		t.Fatalf("expected clamavSnapshotMaxBytes override to be locked")
	}
}
