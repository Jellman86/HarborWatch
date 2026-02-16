package updates

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type fakeExecutor struct{ failStep string }

func (f fakeExecutor) Preflight(ctx context.Context, req Request) error {
	if f.failStep == "preflight" {
		return errors.New("preflight failed")
	}
	return nil
}
func (f fakeExecutor) Backup(ctx context.Context, req Request) error {
	if f.failStep == "backup" {
		return errors.New("backup failed")
	}
	return nil
}
func (f fakeExecutor) Pull(ctx context.Context, req Request) error {
	if f.failStep == "pull" {
		return errors.New("pull failed")
	}
	return nil
}
func (f fakeExecutor) Recreate(ctx context.Context, req Request) error {
	if f.failStep == "recreate" {
		return errors.New("recreate failed")
	}
	return nil
}
func (f fakeExecutor) Validate(ctx context.Context, req Request) error {
	if f.failStep == "validate" {
		return errors.New("validate failed")
	}
	return nil
}
func (f fakeExecutor) Rollback(ctx context.Context, req Request, cause error) error {
	if f.failStep == "rollback" {
		return errors.New("rollback failed")
	}
	return nil
}

func newTestService(t *testing.T, failStep string) *Service {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewService(store, fakeExecutor{failStep: failStep})
}

func TestUpdatePipelineSuccess(t *testing.T) {
	svc := newTestService(t, "")
	res, err := svc.StartUpdate(Request{ContainerID: "test-c", TargetImage: "img", ValidateURL: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := svc.GetJob(context.Background(), res.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == "completed" {
			if len(job.Steps) == 0 {
				t.Fatal("expected steps")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for completion")
}

func TestUpdatePipelineRollback(t *testing.T) {
	svc := newTestService(t, "validate")
	res, err := svc.StartUpdate(Request{ContainerID: "test-c", TargetImage: "img", ValidateURL: "http://x"})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := svc.GetJob(context.Background(), res.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == "rolled_back" {
			foundRollback := false
			for _, s := range job.Steps {
				if s.Step == "rollback" {
					foundRollback = true
					break
				}
			}
			if !foundRollback {
				t.Fatal("expected rollback step")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for rolled_back")
}
