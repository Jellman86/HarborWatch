package healthremediation

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	_ "modernc.org/sqlite"
)

func TestServiceInitRequiresMigratedSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	svc := NewService(db, nil, nil, nil, nil, nil)
	if err := svc.Init(context.Background()); err == nil {
		t.Fatalf("expected init to fail when remediation schema is missing")
	}
}

func TestServiceInitSucceedsAfterMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	svc := NewService(db, nil, nil, nil, nil, nil)
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("init after migrations: %v", err)
	}
}
