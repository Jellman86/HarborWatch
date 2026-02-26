package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestRunComposeSnapshotOnChangeSweep_CreatesOnlyWhenChanged(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	snapshotRoot := filepath.Join(t.TempDir(), "snapshots")
	composeFile := filepath.Join(workdir, "compose.yml")
	envFile := filepath.Join(workdir, ".env")

	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	containers := []gen.ContainerSummary{
		{
			ID:    "c-compose-1",
			Image: "nginx:1.25",
			State: "running",
			Labels: map[string]string{
				"com.docker.compose.project":              "demo",
				"com.docker.compose.service":              "app",
				"com.docker.compose.project.working_dir":  workdir,
				"com.docker.compose.project.config_files": composeFile,
			},
		},
	}

	ctx := context.Background()

	first, err := runComposeSnapshotOnChangeSweep(ctx, containers, snapshotRoot, nil)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if first.SnapshotsCreated != 1 {
		t.Fatalf("expected first sweep to create 1 snapshot, got %+v", first)
	}

	second, err := runComposeSnapshotOnChangeSweep(ctx, containers, snapshotRoot, nil)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second.SnapshotsCreated != 0 || second.SkippedUnchanged != 1 {
		t.Fatalf("expected unchanged project to be skipped on second sweep, got %+v", second)
	}

	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.26\n"), 0o644); err != nil {
		t.Fatalf("rewrite compose file: %v", err)
	}

	third, err := runComposeSnapshotOnChangeSweep(ctx, containers, snapshotRoot, nil)
	if err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	if third.SnapshotsCreated != 1 {
		t.Fatalf("expected third sweep to create snapshot after compose change, got %+v", third)
	}
}
