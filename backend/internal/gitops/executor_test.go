package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/docker/docker/api/types/container"
)

func TestBuildEnvFileSortsKeys(t *testing.T) {
	content, err := buildEnvFile(map[string]string{
		"Z_VAR": "z",
		"A_VAR": "a",
	})
	if err != nil {
		t.Fatalf("buildEnvFile returned error: %v", err)
	}
	if content != "A_VAR=a\nZ_VAR=z\n" {
		t.Fatalf("unexpected env content: %q", content)
	}
}

func TestBuildEnvFileRejectsInvalidEntries(t *testing.T) {
	if _, err := buildEnvFile(map[string]string{"BAD=KEY": "x"}); err == nil {
		t.Fatalf("expected invalid key error")
	}

	if _, err := buildEnvFile(map[string]string{"OK": "multi\nline"}); err == nil {
		t.Fatalf("expected multiline value error")
	}
}

func TestBuildEnvFileRejectsWindowsLineEndingsInValues(t *testing.T) {
	_, err := buildEnvFile(map[string]string{"A": "value\r\n"})
	if err == nil {
		t.Fatalf("expected multiline value validation error")
	}
	if strings.Contains(err.Error(), "\r") {
		t.Fatalf("expected normalized error text, got %q", err.Error())
	}
}

func TestResolveEnvFilePathSupportsAbsoluteAndRelative(t *testing.T) {
	abs, err := resolveEnvFilePath("/data/gitops/repo", "/mnt/Storage-SSD/dockercompose/env/.env")
	if err != nil {
		t.Fatalf("resolveEnvFilePath absolute returned error: %v", err)
	}
	if abs != "/mnt/Storage-SSD/dockercompose/env/.env" {
		t.Fatalf("unexpected absolute env file path: %s", abs)
	}

	rel, err := resolveEnvFilePath("/data/gitops/repo", "env/.env")
	if err != nil {
		t.Fatalf("resolveEnvFilePath relative returned error: %v", err)
	}
	if rel != "/data/gitops/repo/env/.env" {
		t.Fatalf("unexpected relative env file path: %s", rel)
	}
}

func TestManagedInlineEnvFilePathLivesOutsideRepoCheckout(t *testing.T) {
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "repo", "apps", "demo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo path: %v", err)
	}

	path, err := managedInlineEnvFilePath(masterDir, "src-1", "dep-1")
	if err != nil {
		t.Fatalf("managedInlineEnvFilePath returned error: %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(masterDir, ".harborwatch", "gitops-env")) {
		t.Fatalf("expected managed env path under HarborWatch gitops env root, got %q", path)
	}
	if strings.HasPrefix(path, repoPath) {
		t.Fatalf("managed env path must not live inside repo checkout: %q", path)
	}
}

func TestRunTrackedDeployUpdatesDeploymentRuntimeStateOnSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "source-success")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:\n  demo:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	src := GitSource{
		ID:               "src-success",
		Name:             "Success Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-success",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.UpdateSourceSyncStatus(ctx, src.ID, "abc123", ""); err != nil {
		t.Fatalf("update source sync status: %v", err)
	}
	if err := store.CreateDeployment(ctx, GitDeployment{
		ID:          "dep-success",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	toolDir := t.TempDir()
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  exit 0
fi
if [ "$1" = "compose" ]; then
  echo "container demo is up-to-date"
  exit 0
fi
exit 1
`)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)

	svc := NewService(store, staticSettingsProvider{masterDir: masterDir})
	events := []string{}
	err := svc.RunTrackedDeploy(ctx, src.ID, "dep-success", "job-success", func(progress int, status, message string) {
		events = append(events, status+":"+message)
	})
	if err != nil {
		t.Fatalf("RunTrackedDeploy returned error: %v", err)
	}

	got, err := store.GetDeployment(ctx, "dep-success")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.DeployStatus != "completed" {
		t.Fatalf("expected completed deploy status, got %q", got.DeployStatus)
	}
	if got.LastJobID != "job-success" {
		t.Fatalf("expected last job id persisted, got %q", got.LastJobID)
	}
	if got.LastDeployedHash != "abc123" {
		t.Fatalf("expected last deployed hash abc123, got %q", got.LastDeployedHash)
	}
	if got.LastDeployedAt == 0 {
		t.Fatalf("expected last deployed at to be set")
	}
	if got.DeployStartedAt == 0 || got.DeployFinishedAt == 0 {
		t.Fatalf("expected deploy runtime timestamps to be set, got start=%d finish=%d", got.DeployStartedAt, got.DeployFinishedAt)
	}
	if got.DeployFinishedAt < got.DeployStartedAt {
		t.Fatalf("expected deploy_finished_at >= deploy_started_at, got start=%d finish=%d", got.DeployStartedAt, got.DeployFinishedAt)
	}
	if got.LastError != "" {
		t.Fatalf("expected last error cleared on success, got %q", got.LastError)
	}
	if len(events) == 0 {
		t.Fatalf("expected progress callback events")
	}
}

func TestRunTrackedDeployStoresBoundedFailureSummary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "source-failure")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:\n  demo:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	src := GitSource{
		ID:               "src-failure",
		Name:             "Failure Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-failure",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(ctx, GitDeployment{
		ID:          "dep-failure",
		GitSourceID: src.ID,
		ComposePath: "docker-compose.yml",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	toolDir := t.TempDir()
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  echo "compose plugin missing" >&2
  exit 1
fi
exit 1
`)
	writeTestScript(t, filepath.Join(toolDir, "docker-compose"), `#!/bin/sh
if [ "$1" = "version" ]; then
  exit 0
fi
count=0
while [ "$count" -lt 5000 ]; do
  printf 'X'
  count=$((count+1))
done
printf '\n'
exit 1
`)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)

	svc := NewService(store, staticSettingsProvider{masterDir: masterDir})
	err := svc.RunTrackedDeploy(ctx, src.ID, "dep-failure", "job-failure", nil)
	if err == nil {
		t.Fatalf("expected RunTrackedDeploy to fail")
	}

	got, err := store.GetDeployment(ctx, "dep-failure")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if got.DeployStatus != "failed" {
		t.Fatalf("expected failed deploy status, got %q", got.DeployStatus)
	}
	if got.LastDeployedAt != 0 {
		t.Fatalf("expected last deployed at to remain unset on failure, got %d", got.LastDeployedAt)
	}
	if got.LastError == "" {
		t.Fatalf("expected last error to be persisted")
	}
	if got.DeployOutputSummary == "" {
		t.Fatalf("expected deploy output summary to be persisted")
	}
	if len(got.DeployOutputSummary) > 1200 {
		t.Fatalf("expected bounded deploy output summary, got length %d", len(got.DeployOutputSummary))
	}
}

func TestRunTrackedDeployPullOnDeployRunsPullBeforeUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "source-pull-order")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:\n  demo:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	src := GitSource{
		ID:               "src-pull-order",
		Name:             "Pull Order Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-pull-order",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(ctx, GitDeployment{
		ID:           "dep-pull-order",
		GitSourceID:  src.ID,
		ComposePath:  "docker-compose.yml",
		Enabled:      true,
		PullOnDeploy: true,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	toolDir := t.TempDir()
	logPath := filepath.Join(toolDir, "compose.log")
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  exit 0
fi
if [ "$1" = "compose" ]; then
  echo "$4" >> "$TEST_LOG"
  exit 0
fi
exit 1
`)
	t.Setenv("TEST_LOG", logPath)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)

	svc := NewService(store, staticSettingsProvider{masterDir: masterDir})
	if err := svc.RunTrackedDeploy(ctx, src.ID, "dep-pull-order", "job-pull-order", nil); err != nil {
		t.Fatalf("RunTrackedDeploy returned error: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read compose log: %v", err)
	}
	if string(logBytes) != "pull\nup\n" {
		t.Fatalf("expected pull before up, got %q", string(logBytes))
	}
}

func TestRunTrackedDeployPullFailureSkipsUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "source-pull-fail")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:\n  demo:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	src := GitSource{
		ID:               "src-pull-fail",
		Name:             "Pull Fail Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-pull-fail",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(ctx, GitDeployment{
		ID:           "dep-pull-fail",
		GitSourceID:  src.ID,
		ComposePath:  "docker-compose.yml",
		Enabled:      true,
		PullOnDeploy: true,
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	toolDir := t.TempDir()
	logPath := filepath.Join(toolDir, "compose.log")
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  exit 0
fi
if [ "$1" = "compose" ]; then
  echo "$4" >> "$TEST_LOG"
  if [ "$4" = "pull" ]; then
    echo "pull failed" >&2
    exit 1
  fi
  exit 0
fi
exit 1
`)
	t.Setenv("TEST_LOG", logPath)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)

	svc := NewService(store, staticSettingsProvider{masterDir: masterDir})
	if err := svc.RunTrackedDeploy(ctx, src.ID, "dep-pull-fail", "job-pull-fail", nil); err == nil {
		t.Fatalf("expected RunTrackedDeploy to fail when pull fails")
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read compose log: %v", err)
	}
	if string(logBytes) != "pull\n" {
		t.Fatalf("expected pull failure to skip up, got %q", string(logBytes))
	}
}

func TestRunTrackedDeployPullOnDeployUsesManagedEnvFileForPullAndUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	ctx := context.Background()
	db := openStoreTestDB(t)
	store := NewStore(db)
	masterDir := t.TempDir()
	repoPath := filepath.Join(masterDir, "source-pull-env")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "docker-compose.yml"), []byte("services:\n  demo:\n    image: ${IMAGE_NAME}\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("BASE_IMAGE=busybox:latest\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	src := GitSource{
		ID:               "src-pull-env",
		Name:             "Pull Env Source",
		URL:              "https://example.com/repo.git",
		Branch:           "main",
		TargetDir:        "source-pull-env",
		AuthMethod:       AuthMethodNone,
		SyncIntervalMins: 5,
	}
	if err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.CreateDeployment(ctx, GitDeployment{
		ID:               "dep-pull-env",
		GitSourceID:      src.ID,
		ComposePath:      "docker-compose.yml",
		Enabled:          true,
		PullOnDeploy:     true,
		EnvInlineEnabled: true,
		EnvInlineContent: "IMAGE_NAME=nginx:latest\n",
	}); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	toolDir := t.TempDir()
	logPath := filepath.Join(toolDir, "compose.log")
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  exit 0
fi
if [ "$1" = "compose" ]; then
  printf '%s|' "$0" >> "$TEST_LOG"
  idx=1
  while [ "$idx" -le "$#" ]; do
    eval "arg=\${$idx}"
    printf '%s ' "$arg" >> "$TEST_LOG"
    idx=$((idx+1))
  done
  printf '\n' >> "$TEST_LOG"
  exit 0
fi
exit 1
`)
	t.Setenv("TEST_LOG", logPath)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)

	svc := NewService(store, staticSettingsProvider{masterDir: masterDir})
	if err := svc.RunTrackedDeploy(ctx, src.ID, "dep-pull-env", "job-pull-env", nil); err != nil {
		t.Fatalf("RunTrackedDeploy returned error: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read compose log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logBytes)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two compose invocations, got %d (%q)", len(lines), string(logBytes))
	}
	for _, line := range lines {
		if !strings.Contains(line, "--env-file") {
			t.Fatalf("expected compose command to include --env-file, got %q", line)
		}
		if !strings.Contains(line, filepath.Join(repoPath, ".env")) {
			t.Fatalf("expected compose command to include project .env, got %q", line)
		}
		if !strings.Contains(line, filepath.Join(masterDir, ".harborwatch", "gitops-env", src.ID, "dep-pull-env.env")) {
			t.Fatalf("expected compose command to include managed override env file, got %q", line)
		}
	}
	if !strings.Contains(lines[0], " pull ") {
		t.Fatalf("expected first command to be pull, got %q", lines[0])
	}
	if !strings.Contains(lines[1], " up ") {
		t.Fatalf("expected second command to be up, got %q", lines[1])
	}
}

func TestRemoveStaleComposeReplacementContainersRemovesOnlyMatchingNonRunningReplacements(t *testing.T) {
	ctx := context.Background()
	client := &fakeComposeCleanupClient{
		containers: []container.Summary{
			{
				ID:    "remove-me",
				Names: []string{"/abc_yawamf-backend"},
				State: "exited",
				Labels: map[string]string{
					"com.docker.compose.replace":              "yawamf-backend",
					"com.docker.compose.project.config_files": "/repo/docker-compose.yml",
					"com.docker.compose.project.working_dir":  "/repo",
				},
			},
			{
				ID:    "keep-running",
				Names: []string{"/def_yawamf-backend"},
				State: "running",
				Labels: map[string]string{
					"com.docker.compose.replace":              "yawamf-backend",
					"com.docker.compose.project.config_files": "/repo/docker-compose.yml",
					"com.docker.compose.project.working_dir":  "/repo",
				},
			},
			{
				ID:    "keep-nonreplacement",
				Names: []string{"/plain-exited"},
				State: "exited",
				Labels: map[string]string{
					"com.docker.compose.project.config_files": "/repo/docker-compose.yml",
					"com.docker.compose.project.working_dir":  "/repo",
				},
			},
			{
				ID:    "keep-other-project",
				Names: []string{"/other"},
				State: "exited",
				Labels: map[string]string{
					"com.docker.compose.replace":              "yawamf-backend",
					"com.docker.compose.project.config_files": "/other/docker-compose.yml",
					"com.docker.compose.project.working_dir":  "/other",
				},
			},
		},
	}

	removed, err := removeStaleComposeReplacementContainers(ctx, client, "/repo/docker-compose.yml", "/repo")
	if err != nil {
		t.Fatalf("removeStaleComposeReplacementContainers returned error: %v", err)
	}
	if len(removed) != 1 || removed[0] != "abc_yawamf-backend" {
		t.Fatalf("expected one removed replacement container, got %#v", removed)
	}
	if len(client.removedIDs) != 1 || client.removedIDs[0] != "remove-me" {
		t.Fatalf("expected only remove-me to be deleted, got %#v", client.removedIDs)
	}
}

func TestRemoveStaleComposeReplacementContainersPropagatesRemoveError(t *testing.T) {
	ctx := context.Background()
	client := &fakeComposeCleanupClient{
		containers: []container.Summary{
			{
				ID:    "remove-me",
				Names: []string{"/abc_yawamf-backend"},
				State: "created",
				Labels: map[string]string{
					"com.docker.compose.replace":              "yawamf-backend",
					"com.docker.compose.project.config_files": "/repo/docker-compose.yml",
					"com.docker.compose.project.working_dir":  "/repo",
				},
			},
		},
		removeErr: errors.New("boom"),
	}

	_, err := removeStaleComposeReplacementContainers(ctx, client, "/repo/docker-compose.yml", "/repo")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected remove error to propagate, got %v", err)
	}
}

type staticSettingsProvider struct {
	masterDir string
}

func (s staticSettingsProvider) Get(ctx context.Context) (settings.Settings, error) {
	return settings.Settings{GitOpsMasterDirectory: s.masterDir}, nil
}

func writeTestScript(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write script %s: %v", path, err)
	}
}

type fakeComposeCleanupClient struct {
	containers []container.Summary
	removedIDs []string
	removeErr  error
}

func (f *fakeComposeCleanupClient) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return f.containers, nil
}

func (f *fakeComposeCleanupClient) ContainerRemove(_ context.Context, id string, _ container.RemoveOptions) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removedIDs = append(f.removedIDs, id)
	return nil
}
