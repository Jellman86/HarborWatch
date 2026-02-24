package metrics

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func TestStoreInitRequiresMigratedSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when metrics schema is missing")
	}
}

func TestStoreInitSucceedsAfterMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store after migrations: %v", err)
	}
}
