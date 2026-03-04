package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gitops"
	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	_ "modernc.org/sqlite"
)

func TestGitOpsSyncAutoCreatesDisabledDeploymentRules(t *testing.T) {
	db := openGitOpsTestDB(t)
	store := gitops.NewStore(db)

	repoURL := createLocalGitRepo(t, map[string]string{
		"docker-compose.yml":     "services:\n  web:\n    image: nginx:1.27\n",
		"stacks/api/compose.yml": "services:\n  api:\n    image: caddy:2\n",
		"README.md":              "# test\n",
	})

	src := gitops.GitSource{
		ID:               "src-sync",
		Name:             "Sync Source",
		URL:              repoURL,
		Branch:           "master",
		TargetDir:        "source-sync",
		AuthMethod:       gitops.AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	masterDir := t.TempDir()
	mux := newGitOpsTestMux(db, masterDir)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/sources/src-sync/sync", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	deployments, err := store.ListDeploymentsForSource(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deployments) != 2 {
		t.Fatalf("expected 2 auto-created deployments, got %d", len(deployments))
	}

	paths := make([]string, 0, len(deployments))
	for _, dep := range deployments {
		if dep.Enabled {
			t.Fatalf("expected auto-created deployment %q to be disabled", dep.ComposePath)
		}
		if !dep.AutoCreated {
			t.Fatalf("expected auto-created deployment %q to be flagged autoCreated", dep.ComposePath)
		}
		paths = append(paths, dep.ComposePath)
	}
	sort.Strings(paths)
	expected := []string{"docker-compose.yml", "stacks/api/compose.yml"}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatalf("unexpected compose paths: got=%v want=%v", paths, expected)
	}
}

func TestGitOpsUpdateDeploymentRouteUpdatesFields(t *testing.T) {
	db := openGitOpsTestDB(t)
	store := gitops.NewStore(db)
	mux := newGitOpsTestMux(db, t.TempDir())

	src := gitops.GitSource{
		ID:               "src-update",
		Name:             "Update Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-update",
		AuthMethod:       gitops.AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	initial := gitops.GitDeployment{
		ID:          "dep-update",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     false,
	}
	if err := store.CreateDeployment(context.Background(), initial); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	body := bytes.NewBufferString(`{
		"composePath":"stacks/web/compose.yaml",
		"envFilePath":"stacks/web/.env",
		"envVarsJson":"{\"APP_ENV\":\"prod\"}",
		"enabled":true
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/gitops/deployments/dep-update", body)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	deployments, err := store.ListDeploymentsForSource(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments))
	}
	got := deployments[0]
	if got.ComposePath != "stacks/web/compose.yaml" {
		t.Fatalf("composePath not updated: %q", got.ComposePath)
	}
	if got.EnvFilePath != "stacks/web/.env" {
		t.Fatalf("envFilePath not updated: %q", got.EnvFilePath)
	}
	if got.EnvVarsJSON != `{"APP_ENV":"prod"}` {
		t.Fatalf("envVarsJson not updated: %q", got.EnvVarsJSON)
	}
	if !got.Enabled {
		t.Fatalf("enabled flag not updated")
	}
	if got.AutoCreated {
		t.Fatalf("autoCreated should be false after edit")
	}
}

func TestGitOpsSourceFilesRouteListsComposeAndEnvFiles(t *testing.T) {
	db := openGitOpsTestDB(t)
	store := gitops.NewStore(db)

	masterDir := t.TempDir()
	targetDir := "source-files"
	repoPath := filepath.Join(masterDir, targetDir)
	if err := os.MkdirAll(filepath.Join(repoPath, "apps", "api"), 0o755); err != nil {
		t.Fatalf("mkdir repo path: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:{}"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "apps", "api", "compose.yaml"), []byte("services:{}"), 0o644); err != nil {
		t.Fatalf("write compose nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "apps", "api", ".env.prod"), []byte("B=2\n"), 0o600); err != nil {
		t.Fatalf("write nested env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".git", "config"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write git metadata: %v", err)
	}

	src := gitops.GitSource{
		ID:               "src-files",
		Name:             "Files Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        targetDir,
		AuthMethod:       gitops.AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	mux := newGitOpsTestMux(db, masterDir)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/gitops/sources/src-files/files", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		ComposeFiles []string `json:"composeFiles"`
		EnvFiles     []string `json:"envFiles"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sort.Strings(payload.ComposeFiles)
	sort.Strings(payload.EnvFiles)
	if !reflect.DeepEqual(payload.ComposeFiles, []string{"apps/api/compose.yaml", "docker-compose.yml"}) {
		t.Fatalf("unexpected compose files: %v", payload.ComposeFiles)
	}
	if !reflect.DeepEqual(payload.EnvFiles, []string{".env", "apps/api/.env.prod"}) {
		t.Fatalf("unexpected env files: %v", payload.EnvFiles)
	}
}

func TestGitOpsCreateDeploymentRouteReturnsConflictOnDuplicateComposePath(t *testing.T) {
	db := openGitOpsTestDB(t)
	store := gitops.NewStore(db)
	mux := newGitOpsTestMux(db, t.TempDir())

	src := gitops.GitSource{
		ID:               "src-conflict-create",
		Name:             "Conflict Create",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-conflict-create",
		AuthMethod:       gitops.AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(context.Background(), gitops.GitDeployment{
		ID:          "dep-existing",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     false,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	body := bytes.NewBufferString(`{"gitSourceId":"src-conflict-create","composePath":"docker-compose.yml","enabled":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/deployments", body)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGitOpsUpdateDeploymentEnabledReturnsNotFound(t *testing.T) {
	db := openGitOpsTestDB(t)
	mux := newGitOpsTestMux(db, t.TempDir())

	body := bytes.NewBufferString(`{"enabled":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/gitops/deployments/missing/enabled", body)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGitOpsToggleDeploymentClearsAutoCreatedFlag(t *testing.T) {
	db := openGitOpsTestDB(t)
	store := gitops.NewStore(db)
	mux := newGitOpsTestMux(db, t.TempDir())

	src := gitops.GitSource{
		ID:               "src-toggle-auto",
		Name:             "Toggle Auto",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-toggle-auto",
		AuthMethod:       gitops.AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(context.Background(), gitops.GitDeployment{
		ID:          "dep-toggle-auto",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		AutoCreated: true,
		Enabled:     false,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	body := bytes.NewBufferString(`{"enabled":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/gitops/deployments/dep-toggle-auto/enabled", body)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	got, err := store.GetDeployment(context.Background(), "dep-toggle-auto")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.AutoCreated {
		t.Fatalf("expected autoCreated to be false after explicit toggle")
	}
	if !got.Enabled {
		t.Fatalf("expected enabled true after toggle")
	}
}

func openGitOpsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gitops.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrations.Run failed: %v", err)
	}
	return db
}

func newGitOpsTestMux(db *sql.DB, masterDir string) http.Handler {
	return NewMuxWithDeps(
		db,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		staticGitOpsSettingsService{st: settings.Settings{GitOpsMasterDirectory: masterDir}},
		fakePortainerClient{},
		fakeRulesService{},
		nil,
		nil,
	)
}

func createLocalGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	repoDir := t.TempDir()
	repo, err := gogit.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	for rel, content := range files {
		abs := filepath.Join(repoDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "HarborWatch Test",
			Email: "test@example.com",
			When:  time.Now().UTC(),
		},
	}); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return repoDir
}

type staticGitOpsSettingsService struct {
	st settings.Settings
}

func (s staticGitOpsSettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return s.st, nil
}

func (s staticGitOpsSettingsService) Save(ctx context.Context, st settings.Settings) error {
	s.st = st
	return nil
}
