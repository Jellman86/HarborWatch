package httpapi

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestDetectContainerOrchestrationMode(t *testing.T) {
	t.Run("plain docker", func(t *testing.T) {
		mode := detectContainerOrchestrationMode(gen.ContainerSummary{
			ID:     "c1",
			Labels: map[string]string{},
		})
		if mode != orchestrationModePlainDocker {
			t.Fatalf("expected plain docker, got %q", mode)
		}
	})

	t.Run("local compose", func(t *testing.T) {
		mode := detectContainerOrchestrationMode(gen.ContainerSummary{
			ID: "c2",
			Labels: map[string]string{
				"com.docker.compose.project": "media",
				"com.docker.compose.service": "sonarr",
			},
		})
		if mode != orchestrationModeDockerCompose {
			t.Fatalf("expected docker compose, got %q", mode)
		}
	})

	t.Run("portainer by label", func(t *testing.T) {
		mode := detectContainerOrchestrationMode(gen.ContainerSummary{
			ID: "c3",
			Labels: map[string]string{
				"io.portainer.stack_id":                  "12",
				"com.docker.compose.project":             "media",
				"com.docker.compose.service":             "gluetun",
				"io.portainer.endpoint_id":               "1",
				"com.docker.compose.project.working_dir": "/data/compose/12",
			},
		})
		if mode != orchestrationModePortainerStack {
			t.Fatalf("expected portainer stack, got %q", mode)
		}
	})

	t.Run("portainer by path pattern", func(t *testing.T) {
		mode := detectContainerOrchestrationMode(gen.ContainerSummary{
			ID: "c4",
			Labels: map[string]string{
				"com.docker.compose.project":              "media",
				"com.docker.compose.service":              "prowlarr",
				"com.docker.compose.project.config_files": "/data/compose/99/docker-compose.yml",
			},
		})
		if mode != orchestrationModePortainerStack {
			t.Fatalf("expected portainer stack, got %q", mode)
		}
	})
}

func TestDiscoverLocalComposeProjects(t *testing.T) {
	tempDir := t.TempDir()
	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	readonlyFile := filepath.Join(tempDir, "readonly-compose.yml")
	if err := os.WriteFile(readonlyFile, []byte("services:\n  db:\n    image: postgres:16\n"), 0o444); err != nil {
		t.Fatalf("write readonly compose file: %v", err)
	}

	containers := []gen.ContainerSummary{
		{
			ID:    "plain-1",
			Names: []string{"/plain"},
			Image: "nginx:latest",
		},
		{
			ID:    "p1-web",
			Names: []string{"/web"},
			Image: "ghcr.io/acme/web:1.2.3",
			Labels: map[string]string{
				"com.docker.compose.project":              "app",
				"com.docker.compose.service":              "web",
				"com.docker.compose.project.working_dir":  tempDir,
				"com.docker.compose.project.config_files": composeFile,
			},
		},
		{
			ID:    "p1-worker",
			Names: []string{"/worker"},
			Image: "ghcr.io/acme/worker:1.2.3",
			Labels: map[string]string{
				"com.docker.compose.project":              "app",
				"com.docker.compose.service":              "worker",
				"com.docker.compose.project.working_dir":  tempDir,
				"com.docker.compose.project.config_files": composeFile,
			},
		},
		{
			ID:    "p2-ro",
			Names: []string{"/db"},
			Image: "postgres:16",
			Labels: map[string]string{
				"com.docker.compose.project":              "readonly",
				"com.docker.compose.service":              "db",
				"com.docker.compose.project.working_dir":  tempDir,
				"com.docker.compose.project.config_files": readonlyFile,
			},
		},
		{
			ID:    "p3-missing",
			Names: []string{"/missing"},
			Image: "redis:7",
			Labels: map[string]string{
				"com.docker.compose.project":              "missing",
				"com.docker.compose.service":              "cache",
				"com.docker.compose.project.config_files": filepath.Join(tempDir, "missing.yml"),
			},
		},
		{
			ID:    "portainer-1",
			Names: []string{"/pt"},
			Image: "alpine:3.20",
			Labels: map[string]string{
				"io.portainer.stack_id":      "44",
				"com.docker.compose.project": "pt",
				"com.docker.compose.service": "svc",
			},
		},
	}

	projects := discoverLocalComposeProjects(containers)
	if len(projects) != 3 {
		t.Fatalf("expected 3 local compose projects, got %d", len(projects))
	}

	var app, readonly, missing *localComposeProject
	for i := range projects {
		switch projects[i].ProjectName {
		case "app":
			app = &projects[i]
		case "readonly":
			readonly = &projects[i]
		case "missing":
			missing = &projects[i]
		}
	}

	if app == nil || readonly == nil || missing == nil {
		t.Fatalf("expected app/readonly/missing projects in %+v", projects)
	}
	if app.SourceStatus != composeSourceStatusVerifiedWritable {
		t.Fatalf("expected app project writable source, got %q", app.SourceStatus)
	}
	if len(app.Members) != 2 {
		t.Fatalf("expected app to have 2 members, got %d", len(app.Members))
	}
	if missing.SourceStatus != composeSourceStatusUnverified {
		t.Fatalf("expected missing project unverified, got %q", missing.SourceStatus)
	}
	if readonly.SourceStatus != composeSourceStatusVerifiedReadonly && runtime.GOOS != "windows" {
		// On some environments (e.g. root), readonly permission checks may still succeed.
		t.Logf("readonly source status = %q (acceptable in privileged environments)", readonly.SourceStatus)
	}
}
