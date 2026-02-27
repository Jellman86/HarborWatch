package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

func TestComposeProjectEditorDetail_ReturnsComposeAndEnv(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(workdir, ".env")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeSettingsService{}, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/compose/projects/demo", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["projectName"] != "demo" {
		t.Fatalf("expected projectName=demo, got %#v", payload["projectName"])
	}
	if payload["composeFiles"] == nil {
		t.Fatalf("expected composeFiles in payload")
	}
	if payload["envFile"] == nil {
		t.Fatalf("expected envFile in payload")
	}
}

func TestComposeProjectValidate_RejectsInvalidYAML(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	body := stringsNewReader(t, `{
		"composeFiles":[{"path":"`+composeFile+`","content":"services:\n  app:\n    image: [bad"}]
	}`)

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeSettingsService{}, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/compose/projects/demo/validate", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestComposeProjectSave_RejectsHashConflict(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	body := stringsNewReader(t, `{
		"composeFiles":[{"path":"`+composeFile+`","content":"services:\n  app:\n    image: nginx:1.26\n","expectedSha256":"deadbeef"}]
	}`)

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeSettingsService{}, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/compose/projects/demo", body))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestComposeProjectSave_SucceedsAndPersistsComposeAndEnv(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(workdir, ".env")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, staticComposeSettingsService{settings.Settings{
		ComposeSnapshotRootPath: filepath.Join(t.TempDir(), "snapshots"),
	}}, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	// Load current hashes from detail endpoint.
	recDetail := httptest.NewRecorder()
	mux.ServeHTTP(recDetail, httptest.NewRequest(http.MethodGet, "/api/compose/projects/demo", nil))
	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d body=%s", recDetail.Code, recDetail.Body.String())
	}
	var detail struct {
		ComposeFiles []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"composeFiles"`
		EnvFile struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"envFile"`
	}
	if err := json.NewDecoder(bytes.NewReader(recDetail.Body.Bytes())).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.ComposeFiles) != 1 {
		t.Fatalf("expected 1 compose file, got %d", len(detail.ComposeFiles))
	}

	body := stringsNewReader(t, `{
		"composeFiles":[{"path":"`+composeFile+`","content":"services:\n  app:\n    image: nginx:1.26\n","expectedSha256":"`+detail.ComposeFiles[0].SHA256+`"}],
		"envFile":{"path":"`+envFile+`","content":"APP_ENV=staging\n","exists":true,"expectedSha256":"`+detail.EnvFile.SHA256+`"}
	}`)

	recSave := httptest.NewRecorder()
	mux.ServeHTTP(recSave, httptest.NewRequest(http.MethodPut, "/api/compose/projects/demo", body))
	if recSave.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recSave.Code, recSave.Body.String())
	}

	gotCompose, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if string(gotCompose) != "services:\n  app:\n    image: nginx:1.26\n" {
		t.Fatalf("unexpected compose content: %s", string(gotCompose))
	}
	gotEnv, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if string(gotEnv) != "APP_ENV=staging\n" {
		t.Fatalf("unexpected env content: %s", string(gotEnv))
	}
}

func TestComposeProjectSave_RejectsNonWritableEnvFile(t *testing.T) {
	skipIfRootUser(t)

	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	lockedDir := filepath.Join(workdir, "locked")
	if err := os.MkdirAll(lockedDir, 0o755); err != nil {
		t.Fatalf("mkdir locked dir: %v", err)
	}
	envFile := filepath.Join(lockedDir, ".env")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("chmod dir readonly: %v", err)
	}
	defer func() { _ = os.Chmod(lockedDir, 0o755) }()

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  lockedDir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, staticComposeSettingsService{settings.Settings{
		ComposeSnapshotRootPath: filepath.Join(t.TempDir(), "snapshots"),
	}}, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	recDetail := httptest.NewRecorder()
	mux.ServeHTTP(recDetail, httptest.NewRequest(http.MethodGet, "/api/compose/projects/demo", nil))
	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d body=%s", recDetail.Code, recDetail.Body.String())
	}
	var detail struct {
		ComposeFiles []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"composeFiles"`
		EnvFile struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"envFile"`
	}
	if err := json.NewDecoder(bytes.NewReader(recDetail.Body.Bytes())).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.ComposeFiles) != 1 {
		t.Fatalf("expected 1 compose file, got %d", len(detail.ComposeFiles))
	}

	body := stringsNewReader(t, `{
		"composeFiles":[],
		"envFile":{"path":"`+envFile+`","content":"APP_ENV=staging\n","exists":true,"expectedSha256":"`+detail.EnvFile.SHA256+`"}
	}`)

	recSave := httptest.NewRecorder()
	mux.ServeHTTP(recSave, httptest.NewRequest(http.MethodPut, "/api/compose/projects/demo", body))
	if recSave.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recSave.Code, recSave.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(recSave.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode save error: %v", err)
	}
	if payload["error"] != "compose_file_not_writable" {
		t.Fatalf("expected compose_file_not_writable, got %#v", payload["error"])
	}
}

func TestComposeProjectSave_PreflightReadErrorIsNotHashConflict(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(workdir, ".env")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, staticComposeSettingsService{settings.Settings{
		ComposeSnapshotRootPath: filepath.Join(t.TempDir(), "snapshots"),
	}}, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	if err := os.Remove(envFile); err != nil {
		t.Fatalf("remove env: %v", err)
	}

	body := stringsNewReader(t, `{
		"composeFiles":[{"path":"`+composeFile+`","content":"services:\n  app:\n    image: nginx:1.26\n"}],
		"envFile":{"path":"`+envFile+`","content":"APP_ENV=staging\n","exists":true,"expectedSha256":"deadbeef"}
	}`)
	recSave := httptest.NewRecorder()
	mux.ServeHTTP(recSave, httptest.NewRequest(http.MethodPut, "/api/compose/projects/demo", body))
	if recSave.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", recSave.Code, recSave.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(recSave.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode save error: %v", err)
	}
	if payload["error"] != "compose_preflight_failed" {
		t.Fatalf("expected compose_preflight_failed, got %#v", payload["error"])
	}
}

func TestComposeProjectPrettify_ReturnsFormattedCompose(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{
				ID:    "c1",
				Names: []string{"/demo-app"},
				Labels: map[string]string{
					"com.docker.compose.project":              "demo",
					"com.docker.compose.service":              "app",
					"com.docker.compose.project.working_dir":  workdir,
					"com.docker.compose.project.config_files": composeFile,
				},
			},
		},
	}

	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeSettingsService{}, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	body := stringsNewReader(t, `{
		"composeFiles":[{"path":"`+composeFile+`","content":"services: {app: {image: nginx:1.26}}"}]
	}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/compose/projects/demo/prettify", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		ComposeFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"composeFiles"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode prettify response: %v", err)
	}
	if len(payload.ComposeFiles) != 1 {
		t.Fatalf("expected 1 compose file, got %d", len(payload.ComposeFiles))
	}
	if payload.ComposeFiles[0].Content == "services: {app: {image: nginx:1.26}}" {
		t.Fatalf("expected formatted yaml output, got %q", payload.ComposeFiles[0].Content)
	}
}

type staticComposeSettingsService struct {
	st settings.Settings
}

func (s staticComposeSettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return s.st, nil
}

func (s staticComposeSettingsService) Save(ctx context.Context, st settings.Settings) error {
	s.st = st
	return nil
}

func stringsNewReader(t *testing.T, s string) *bytes.Reader {
	t.Helper()
	return bytes.NewReader([]byte(s))
}

func skipIfRootUser(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission test is not reliable as root")
	}
}
