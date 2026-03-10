package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/gitops"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type fakeGitOpsLookup struct {
	sources     []gitops.GitSource
	deployments map[string][]gitops.GitDeployment
}

func (f fakeGitOpsLookup) ListSources(ctx context.Context) ([]gitops.GitSource, error) {
	return f.sources, nil
}

func (f fakeGitOpsLookup) ListDeploymentsForSource(ctx context.Context, sourceID string) ([]gitops.GitDeployment, error) {
	return append([]gitops.GitDeployment(nil), f.deployments[sourceID]...), nil
}

func TestBuildUpdateRequestForContainer_IncludesProjectAndGitOpsOverrideEnvFiles(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "docker-configs", "security_inference_stack")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	composeFile := filepath.Join(repoPath, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: ghcr.io/acme/app:1.2.3\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	envPath := filepath.Join(repoPath, ".env")
	if err := os.WriteFile(envPath, []byte("APP_TAG=1.2.3\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	summary := gen.ContainerSummary{
		ID:    "c1",
		Image: "ghcr.io/acme/app:1.2.3",
		Names: []string{"/web"},
		Labels: map[string]string{
			"com.docker.compose.project":              "security",
			"com.docker.compose.service":              "web",
			"com.docker.compose.project.working_dir":  repoPath,
			"com.docker.compose.project.config_files": composeFile,
		},
	}
	dockerClient := autoTaskDockerClient{
		containers: []gen.ContainerSummary{summary},
		byID:       map[string]gen.ContainerSummary{summary.ID: summary},
	}
	lookup := fakeGitOpsLookup{
		sources: []gitops.GitSource{{
			ID:        "src-1",
			TargetDir: "docker-configs",
		}},
		deployments: map[string][]gitops.GitDeployment{
			"src-1": {{
				ID:               "dep-1",
				GitSourceID:      "src-1",
				ComposePath:      "security_inference_stack/docker-compose.yml",
				Enabled:          true,
				EnvInlineEnabled: true,
				EnvInlineContent: "APP_TAG=1.2.3\n",
			}},
		},
	}

	res, err := buildUpdateRequestForContainer(
		context.Background(),
		summary.ID,
		summary.Image,
		"http://localhost:8080/health",
		"",
		0,
		0,
		false,
		false,
		dockerClient,
		nil,
		nil,
		stubSettingsService{st: settings.Settings{GitOpsMasterDirectory: root}},
		nil,
		nil,
		nil,
		lookup,
		updateRequestBuildOptions{},
	)
	if err != nil {
		t.Fatalf("build update request: %v", err)
	}
	if len(res.Request.ComposeEnvFiles) != 1 {
		t.Fatalf("expected only project env file in request, got %#v", res.Request.ComposeEnvFiles)
	}
	if res.Request.ComposeEnvFiles[0] != envPath {
		t.Fatalf("expected project env first, got %#v", res.Request.ComposeEnvFiles)
	}
	if res.Request.ComposeManagedEnvContent != "APP_TAG=1.2.3\n" {
		t.Fatalf("expected managed override env content, got %q", res.Request.ComposeManagedEnvContent)
	}
	if _, err := os.Stat(filepath.Join(root, ".harborwatch", "gitops-env", "src-1", "dep-1.env")); !os.IsNotExist(err) {
		t.Fatalf("expected request building to avoid writing managed env file, got err=%v", err)
	}
}

func TestBuildUpdateRequestForContainer_IgnoresDisabledMatchingGitOpsDeployment(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "docker-configs", "security_inference_stack")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	composeFile := filepath.Join(repoPath, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: ghcr.io/acme/app:1.2.3\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	summary := gen.ContainerSummary{
		ID:    "c1",
		Image: "ghcr.io/acme/app:1.2.3",
		Names: []string{"/web"},
		Labels: map[string]string{
			"com.docker.compose.project":              "security",
			"com.docker.compose.service":              "web",
			"com.docker.compose.project.working_dir":  repoPath,
			"com.docker.compose.project.config_files": composeFile,
		},
	}
	dockerClient := autoTaskDockerClient{
		containers: []gen.ContainerSummary{summary},
		byID:       map[string]gen.ContainerSummary{summary.ID: summary},
	}
	lookup := fakeGitOpsLookup{
		sources: []gitops.GitSource{{ID: "src-1", TargetDir: "docker-configs"}},
		deployments: map[string][]gitops.GitDeployment{
			"src-1": {{
				ID:               "dep-disabled",
				GitSourceID:      "src-1",
				ComposePath:      "security_inference_stack/docker-compose.yml",
				Enabled:          false,
				EnvInlineEnabled: true,
				EnvInlineContent: "APP_TAG=bad\n",
			}},
		},
	}

	res, err := buildUpdateRequestForContainer(
		context.Background(),
		summary.ID,
		summary.Image,
		"http://localhost:8080/health",
		"",
		0,
		0,
		false,
		false,
		dockerClient,
		nil,
		nil,
		stubSettingsService{st: settings.Settings{GitOpsMasterDirectory: root}},
		nil,
		nil,
		nil,
		lookup,
		updateRequestBuildOptions{},
	)
	if err != nil {
		t.Fatalf("build update request: %v", err)
	}
	if res.Request.ComposeManagedEnvContent != "" {
		t.Fatalf("expected disabled deployment to be ignored, got %q", res.Request.ComposeManagedEnvContent)
	}
}
