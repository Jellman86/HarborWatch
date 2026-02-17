package scheduler

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

type testTask struct{ name string }

func (t testTask) Name() string                  { return t.name }
func (t testTask) Run(ctx context.Context) error { return nil }

func TestNormalizeCronSpec(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{in: "* * * * *", out: "0 * * * * *"},
		{in: "0 * * * * *", out: "0 * * * * *"},
		{in: "0 0 * * * *", out: "0 0 * * * *"},
	}
	for _, tc := range cases {
		got := normalizeCronSpec(tc.in)
		if got != tc.out {
			t.Fatalf("normalizeCronSpec(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestAddTaskUpgradesLegacyCronSpec(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := store.SaveSchedule(context.Background(), ScheduleEntry{
		ID:       "metrics_collector",
		CronSpec: "* * * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed schedule: %v", err)
	}

	svc := NewService(store)
	if err := svc.AddTask("0 * * * * *", testTask{name: "metrics_collector"}, true); err != nil {
		t.Fatalf("add task: %v", err)
	}

	entry, err := store.GetSchedule(context.Background(), "metrics_collector")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if entry.CronSpec != "0 * * * * *" {
		t.Fatalf("expected upgraded cron spec, got %q", entry.CronSpec)
	}
}

func TestUpdateTaskSchedulePersistsAndNormalizes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := store.SaveSchedule(context.Background(), ScheduleEntry{
		ID:       "test_task",
		CronSpec: "0 0 0 * * *",
		Enabled:  false,
	}); err != nil {
		t.Fatalf("seed schedule: %v", err)
	}

	svc := NewService(store)
	svc.RegisterTask("test_task", func() Task { return testTask{name: "test_task"} })

	if err := svc.UpdateTaskSchedule(context.Background(), "test_task", "15 2 * * *"); err != nil {
		t.Fatalf("update schedule: %v", err)
	}

	entry, err := store.GetSchedule(context.Background(), "test_task")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if entry.CronSpec != "0 15 2 * * *" {
		t.Fatalf("expected normalized cron spec, got %q", entry.CronSpec)
	}
}

func TestUpdateTaskScheduleRebindsEnabledTask(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := store.SaveSchedule(context.Background(), ScheduleEntry{
		ID:       "test_task",
		CronSpec: "0 0 0 * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed schedule: %v", err)
	}

	svc := NewService(store)
	svc.RegisterTask("test_task", func() Task { return testTask{name: "test_task"} })
	if err := svc.LoadSchedules(context.Background()); err != nil {
		t.Fatalf("load schedules: %v", err)
	}

	oldID, ok := svc.tasks["test_task"]
	if !ok {
		t.Fatalf("expected runtime task to be registered")
	}

	if err := svc.UpdateTaskSchedule(context.Background(), "test_task", "0 45 1 * * *"); err != nil {
		t.Fatalf("update schedule: %v", err)
	}

	newID, ok := svc.tasks["test_task"]
	if !ok {
		t.Fatalf("expected runtime task to remain registered")
	}
	if newID == oldID {
		t.Fatalf("expected runtime task id to change after rebind")
	}

	entry, err := store.GetSchedule(context.Background(), "test_task")
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if entry.CronSpec != "0 45 1 * * *" {
		t.Fatalf("expected updated cron spec, got %q", entry.CronSpec)
	}
}
