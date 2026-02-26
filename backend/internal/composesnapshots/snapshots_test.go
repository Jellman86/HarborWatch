package composesnapshots

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateProjectSnapshotAndDetectChanges(t *testing.T) {
	workdir := t.TempDir()
	snapshotRoot := filepath.Join(t.TempDir(), "snapshots")
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(workdir, ".env")

	if err := os.WriteFile(composeFile, []byte("services:\n  web:\n    image: nginx:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	rec, err := CreateProjectSnapshot(CreateSnapshotInput{
		RootDir:      snapshotRoot,
		ProjectName:  "app",
		ServiceName:  "web",
		WorkingDir:   workdir,
		ConfigFiles:  []string{composeFile},
		CreatedAtUTC: time.Unix(1700000000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if rec.ArchivePath == "" {
		t.Fatalf("expected archive path")
	}
	if _, err := os.Stat(rec.ArchivePath); err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if _, err := os.Stat(rec.ManifestPath); err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	if len(rec.Manifest.Files) < 2 {
		t.Fatalf("expected compose + env files in manifest, got %d", len(rec.Manifest.Files))
	}

	status, err := DetectProjectChangeStatus(snapshotRoot, "app", workdir, []string{composeFile})
	if err != nil {
		t.Fatalf("detect unchanged status: %v", err)
	}
	if status.Status != SnapshotChangeStatusUnchanged {
		t.Fatalf("expected unchanged, got %q", status.Status)
	}

	if err := os.WriteFile(envFile, []byte("FOO=baz\n"), 0o600); err != nil {
		t.Fatalf("rewrite env file: %v", err)
	}

	status, err = DetectProjectChangeStatus(snapshotRoot, "app", workdir, []string{composeFile})
	if err != nil {
		t.Fatalf("detect changed status: %v", err)
	}
	if status.Status != SnapshotChangeStatusChanged {
		t.Fatalf("expected changed, got %q", status.Status)
	}
	if status.ChangedFileCount < 1 {
		t.Fatalf("expected changed file count > 0, got %d", status.ChangedFileCount)
	}
	if status.Latest == nil || status.Latest.ArchivePath == "" {
		t.Fatalf("expected latest snapshot metadata")
	}
}
