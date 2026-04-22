package gitops

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func openStoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gitops-store.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

func TestUpdateDeploymentRuntimeStatusPersistsFields(t *testing.T) {
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)

	src := GitSource{
		ID:               "src-runtime",
		Name:             "Runtime Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "runtime-source",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	dep := GitDeployment{
		ID:          "dep-runtime",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     true,
	}
	if err := store.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	update := GitDeployment{
		ID:                  dep.ID,
		LastJobID:           "job-123",
		DeployStatus:        "running",
		DeployStatusMessage: "Running compose apply",
		DeployStartedAt:     100,
		DeployFinishedAt:    0,
		DeployOutputSummary: "compose output",
	}
	if err := store.UpdateDeploymentRuntimeStatus(ctx, update); err != nil {
		t.Fatalf("update runtime status: %v", err)
	}

	got, err := store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.LastJobID != "job-123" {
		t.Fatalf("expected last job id to persist, got %q", got.LastJobID)
	}
	if got.DeployStatus != "running" {
		t.Fatalf("expected deploy status running, got %q", got.DeployStatus)
	}
	if got.DeployStatusMessage != "Running compose apply" {
		t.Fatalf("expected deploy status message to persist, got %q", got.DeployStatusMessage)
	}
	if got.DeployStartedAt != 100 {
		t.Fatalf("expected deploy_started_at 100, got %d", got.DeployStartedAt)
	}
	if got.DeployFinishedAt != 0 {
		t.Fatalf("expected deploy_finished_at 0, got %d", got.DeployFinishedAt)
	}
	if got.DeployOutputSummary != "compose output" {
		t.Fatalf("expected deploy output summary to persist, got %q", got.DeployOutputSummary)
	}
}

func TestMarkInFlightDeploymentsFailedMarksQueuedAndRunningInterrupted(t *testing.T) {
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)

	src := GitSource{
		ID:               "src-recover",
		Name:             "Recover Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "recover-source",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	for _, dep := range []GitDeployment{
		{ID: "dep-queued", GitSourceID: src.ID, ComposePath: "queued.yml", Enabled: true},
		{ID: "dep-running", GitSourceID: src.ID, ComposePath: "running.yml", Enabled: true},
		{ID: "dep-completed", GitSourceID: src.ID, ComposePath: "completed.yml", Enabled: true},
	} {
		if err := store.CreateDeployment(ctx, dep); err != nil {
			t.Fatalf("create deployment %s: %v", dep.ID, err)
		}
	}

	for _, dep := range []GitDeployment{
		{ID: "dep-queued", DeployStatus: "queued", DeployStatusMessage: "Waiting for slot", DeployStartedAt: 10, LastJobID: "job-queued"},
		{ID: "dep-running", DeployStatus: "running", DeployStatusMessage: "Running compose", DeployStartedAt: 11, LastJobID: "job-running"},
		{ID: "dep-completed", DeployStatus: "completed", DeployStatusMessage: "Done", DeployStartedAt: 12, DeployFinishedAt: 13, LastJobID: "job-completed"},
	} {
		if err := store.UpdateDeploymentRuntimeStatus(ctx, dep); err != nil {
			t.Fatalf("update runtime status for %s: %v", dep.ID, err)
		}
	}

	recovered, err := store.MarkInFlightDeploymentsFailed(ctx, "deploy interrupted by HarborWatch restart")
	if err != nil {
		t.Fatalf("MarkInFlightDeploymentsFailed returned error: %v", err)
	}
	if recovered != 2 {
		t.Fatalf("expected 2 recovered deployments, got %d", recovered)
	}

	queued, err := store.GetDeployment(ctx, "dep-queued")
	if err != nil {
		t.Fatalf("get queued deployment: %v", err)
	}
	if queued.DeployStatus != "failed" {
		t.Fatalf("expected queued deployment to be failed, got %q", queued.DeployStatus)
	}
	if queued.DeployStatusMessage != "deploy interrupted by HarborWatch restart" {
		t.Fatalf("unexpected queued deployment status message: %q", queued.DeployStatusMessage)
	}
	if queued.DeployFinishedAt == 0 {
		t.Fatalf("expected queued deployment finished timestamp to be set")
	}

	running, err := store.GetDeployment(ctx, "dep-running")
	if err != nil {
		t.Fatalf("get running deployment: %v", err)
	}
	if running.DeployStatus != "failed" {
		t.Fatalf("expected running deployment to be failed, got %q", running.DeployStatus)
	}
	if running.DeployStatusMessage != "deploy interrupted by HarborWatch restart" {
		t.Fatalf("unexpected running deployment status message: %q", running.DeployStatusMessage)
	}
	if running.DeployFinishedAt == 0 {
		t.Fatalf("expected running deployment finished timestamp to be set")
	}

	completed, err := store.GetDeployment(ctx, "dep-completed")
	if err != nil {
		t.Fatalf("get completed deployment: %v", err)
	}
	if completed.DeployStatus != "completed" {
		t.Fatalf("expected completed deployment to stay completed, got %q", completed.DeployStatus)
	}
}

func TestDeploymentPullOnDeployPersistsThroughStore(t *testing.T) {
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)

	src := GitSource{
		ID:               "src-pull",
		Name:             "Pull Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "pull-source",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	dep := GitDeployment{
		ID:           "dep-pull",
		GitSourceID:  src.ID,
		ComposePath:  "docker-compose.yml",
		Enabled:      true,
		PullOnDeploy: true,
	}
	if err := store.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	got, err := store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("get deployment after create: %v", err)
	}
	if !got.PullOnDeploy {
		t.Fatalf("expected pull_on_deploy to persist on create")
	}

	got.PullOnDeploy = false
	if err := store.UpdateDeployment(ctx, got); err != nil {
		t.Fatalf("update deployment: %v", err)
	}

	updated, err := store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("get deployment after update: %v", err)
	}
	if updated.PullOnDeploy {
		t.Fatalf("expected pull_on_deploy to persist on update")
	}
}

func TestMarkInFlightDeploymentsFailedClearsOrphanedRunningMetadata(t *testing.T) {
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)

	src := GitSource{
		ID:               "src-orphaned",
		Name:             "Orphaned Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "orphaned-source",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	dep := GitDeployment{
		ID:          "dep-orphaned",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     true,
	}
	if err := store.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	if err := store.UpdateDeploymentRuntimeStatus(ctx, GitDeployment{
		ID:                  dep.ID,
		LastJobID:           "job-orphaned",
		DeployStatus:        "running",
		DeployStatusMessage: "Validating deployment",
		DeployStartedAt:     123,
		DeployFinishedAt:    0,
		DeployOutputSummary: "stale output",
	}); err != nil {
		t.Fatalf("seed runtime status: %v", err)
	}

	recovered, err := store.MarkInFlightDeploymentsFailed(ctx, "deploy interrupted by HarborWatch restart")
	if err != nil {
		t.Fatalf("MarkInFlightDeploymentsFailed returned error: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered deployment, got %d", recovered)
	}

	got, err := store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.DeployStatus != "failed" {
		t.Fatalf("expected failed deploy status, got %q", got.DeployStatus)
	}
	if got.DeployStatusMessage != "deploy interrupted by HarborWatch restart" {
		t.Fatalf("expected restart interruption message, got %q", got.DeployStatusMessage)
	}
	if got.LastJobID != "job-orphaned" {
		t.Fatalf("expected last job id preserved for auditability, got %q", got.LastJobID)
	}
	if got.DeployStartedAt != 123 {
		t.Fatalf("expected deploy_started_at preserved, got %d", got.DeployStartedAt)
	}
	if got.DeployFinishedAt == 0 {
		t.Fatalf("expected deploy_finished_at to be set")
	}
}
