package updates

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func newUpdateStoreTest(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates.db"))
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
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates-missing-schema.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when update tables are missing")
	}
}

func TestListRunsForContainerMatchesStoredContainerName(t *testing.T) {
	store := newUpdateStoreTest(t)
	now := time.Now().UTC().Unix()
	if err := store.CreateRunWithContainerName(context.Background(), gen.UpdateJobStatus{
		JobID:       "u1",
		ContainerID: "old-container-id",
		TargetImage: "ghcr.io/example/app:v2",
		ValidateURL: "http://localhost/health",
		Status:      "completed",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, "gluetun"); err != nil {
		t.Fatalf("create run: %v", err)
	}

	rows, err := store.ListRunsForContainer(context.Background(), "gluetun", 10)
	if err != nil {
		t.Fatalf("list runs by name: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 run by container name, got %d", len(rows))
	}
	if rows[0].JobID != "u1" {
		t.Fatalf("unexpected run returned: %#v", rows[0])
	}
}
