package settings

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
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
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when app_settings table is missing")
	}
}

func TestGetDefaultsPrepopulateHarborWatchIgnore(t *testing.T) {
	store := newTestStore(t)
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.TrivySweepMode != "running-only" {
		t.Fatalf("expected default trivy sweep mode running-only, got %q", got.TrivySweepMode)
	}
	if !strings.Contains(strings.ToLower(got.AutomationIgnoredContainers), "harborwatch") {
		t.Fatalf("expected harborwatch in automation ignores, got %q", got.AutomationIgnoredContainers)
	}
	if got.DefaultValidateMode != "docker" {
		t.Fatalf("expected default validate mode docker, got %q", got.DefaultValidateMode)
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

func TestGetAppliesTrivySweepModeEnvOverride(t *testing.T) {
	t.Setenv("HW_TRIVY_SWEEP_MODE", "all-images")

	store := newTestStore(t)
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.TrivySweepMode != "all-images" {
		t.Fatalf("expected TrivySweepMode=all-images, got %q", got.TrivySweepMode)
	}
	if !got.EnvironmentOverrides["trivySweepMode"] {
		t.Fatalf("expected trivySweepMode override to be locked")
	}
}

func TestGetAppliesRetentionEnvOverrides(t *testing.T) {
	t.Setenv("HW_RETENTION_LOG_DAYS", "45")
	t.Setenv("HW_RETENTION_METRICS_DAYS", "20")
	t.Setenv("HW_RETENTION_SCAN_RESULTS_DAYS", "50")
	t.Setenv("HW_RETENTION_SCAN_JOBS_DAYS", "55")
	t.Setenv("HW_RETENTION_UPDATE_RUNS_DAYS", "120")
	t.Setenv("HW_RETENTION_COMPOSE_AUDIT_DAYS", "150")
	t.Setenv("HW_RETENTION_AI_USAGE_DAYS", "365")

	store := newTestStore(t)
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.RetentionLogsDays != 45 {
		t.Fatalf("expected RetentionLogsDays=45, got %d", got.RetentionLogsDays)
	}
	if got.RetentionMetricsDays != 20 {
		t.Fatalf("expected RetentionMetricsDays=20, got %d", got.RetentionMetricsDays)
	}
	if got.RetentionScanResultsDays != 50 {
		t.Fatalf("expected RetentionScanResultsDays=50, got %d", got.RetentionScanResultsDays)
	}
	if got.RetentionScanJobsDays != 55 {
		t.Fatalf("expected RetentionScanJobsDays=55, got %d", got.RetentionScanJobsDays)
	}
	if got.RetentionUpdateRunsDays != 120 {
		t.Fatalf("expected RetentionUpdateRunsDays=120, got %d", got.RetentionUpdateRunsDays)
	}
	if got.RetentionComposeAuditDays != 150 {
		t.Fatalf("expected RetentionComposeAuditDays=150, got %d", got.RetentionComposeAuditDays)
	}
	if got.RetentionAIUsageDays != 365 {
		t.Fatalf("expected RetentionAIUsageDays=365, got %d", got.RetentionAIUsageDays)
	}
}

func TestSaveAndGetLifecycleDefaultPolicySettings(t *testing.T) {
	store := newTestStore(t)
	in := Settings{
		DefaultValidateMode:                "docker",
		DefaultValidateTimeoutSec:          120,
		DefaultValidateIntervalSec:         5,
		DefaultAIValidateLogs:              true,
		DefaultAutoRollback:                true,
		DefaultRestartOnUnhealthy:          true,
		UnhealthyRestartCooldownSecDefault: 900,
	}
	if err := store.Save(context.Background(), in); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.DefaultValidateMode != "docker" {
		t.Fatalf("expected DefaultValidateMode=docker, got %q", got.DefaultValidateMode)
	}
	if got.DefaultValidateTimeoutSec != 120 {
		t.Fatalf("expected DefaultValidateTimeoutSec=120, got %d", got.DefaultValidateTimeoutSec)
	}
	if got.DefaultValidateIntervalSec != 5 {
		t.Fatalf("expected DefaultValidateIntervalSec=5, got %d", got.DefaultValidateIntervalSec)
	}
	if !got.DefaultAIValidateLogs {
		t.Fatalf("expected DefaultAIValidateLogs=true")
	}
	if !got.DefaultAutoRollback {
		t.Fatalf("expected DefaultAutoRollback=true")
	}
	if !got.DefaultRestartOnUnhealthy {
		t.Fatalf("expected DefaultRestartOnUnhealthy=true")
	}
}

func TestSetDashboardUpdateCheckSnapshotPersistsReadOnlyFields(t *testing.T) {
	store := newTestStore(t)
	if err := store.SetDashboardUpdateCheckSnapshot(context.Background(), 14, 1700000123); err != nil {
		t.Fatalf("set dashboard update snapshot: %v", err)
	}

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.DashboardLastUpdateDetectedCount != 14 {
		t.Fatalf("expected DashboardLastUpdateDetectedCount=14, got %d", got.DashboardLastUpdateDetectedCount)
	}
	if got.DashboardLastUpdateCheckAt != 1700000123 {
		t.Fatalf("expected DashboardLastUpdateCheckAt=1700000123, got %d", got.DashboardLastUpdateCheckAt)
	}
}

func TestSavePersistsGitOpsMasterDirectory(t *testing.T) {
	store := newTestStore(t)

	const expected = "/mnt/Storage-SSD/dockercompose"
	if err := store.Save(context.Background(), Settings{
		GitOpsMasterDirectory: expected,
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if got.GitOpsMasterDirectory != expected {
		t.Fatalf("expected GitOpsMasterDirectory=%q, got %q", expected, got.GitOpsMasterDirectory)
	}
}
