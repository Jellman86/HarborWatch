package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestExtractComposeServiceImageRefFromFiles_IgnoresFilesWithoutServicesBlock(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "compose.base.yml")
	override := filepath.Join(dir, "compose.override.yml")

	if err := os.WriteFile(base, []byte("volumes:\n  data: {}\n"), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if err := os.WriteFile(override, []byte("services:\n  web:\n    image: ghcr.io/acme/app:1.2.3\n"), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	got, err := extractComposeServiceImageRefFromFiles([]string{base, override}, "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ghcr.io/acme/app:1.2.3" {
		t.Fatalf("unexpected image ref: %q", got)
	}
}

func TestResolveDeclaredLocalComposeImageRef_UsesResolvedComposeConfigForInterpolatedImage(t *testing.T) {
	dir := t.TempDir()
	composeFile := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: ghcr.io/acme/app:${APP_TAG}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	originalRunner := dockerComposeConfigRunner
	t.Cleanup(func() { dockerComposeConfigRunner = originalRunner })
	called := false
	dockerComposeConfigRunner = func(ctx context.Context, workingDir string, configFiles []string) ([]byte, error) {
		called = true
		if workingDir != dir {
			t.Fatalf("unexpected workingDir: %q", workingDir)
		}
		if len(configFiles) != 1 || configFiles[0] != composeFile {
			t.Fatalf("unexpected config files: %#v", configFiles)
		}
		return []byte("services:\n  web:\n    image: ghcr.io/acme/app:1.2.3\n"), nil
	}

	summary := gen.ContainerSummary{
		ID:    "c1",
		Image: "ghcr.io/acme/app:1.2.3",
		Labels: map[string]string{
			"com.docker.compose.project":              "app",
			"com.docker.compose.service":              "web",
			"com.docker.compose.project.working_dir":  dir,
			"com.docker.compose.project.config_files": composeFile,
		},
	}

	got, err := resolveDeclaredLocalComposeImageRef(context.Background(), summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected docker compose config runner to be used for interpolated image")
	}
	if got != "ghcr.io/acme/app:1.2.3" {
		t.Fatalf("unexpected resolved image ref: %q", got)
	}
}
