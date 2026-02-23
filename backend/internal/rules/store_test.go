package rules

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newRulesTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	return store
}

func TestGetDefaultsSkipHealthCheckFalse(t *testing.T) {
	store := newRulesTestStore(t)
	got, err := store.Get(context.Background(), "c1", "container1")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if got.SkipHealthCheck {
		t.Fatalf("expected default SkipHealthCheck=false")
	}
}

func TestSavePersistsSkipHealthCheck(t *testing.T) {
	store := newRulesTestStore(t)
	if err := store.Save(context.Background(), ContainerRules{
		ContainerID:     "c1",
		ContainerName:   "container1",
		UpdatePolicy:    "auto",
		ValidateURL:     "http://localhost:8080/health",
		SkipHealthCheck: true,
	}); err != nil {
		t.Fatalf("save rules: %v", err)
	}

	got, err := store.Get(context.Background(), "c1", "container1")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if !got.SkipHealthCheck {
		t.Fatalf("expected SkipHealthCheck=true after save")
	}
}
