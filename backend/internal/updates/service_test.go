package updates

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	_ "modernc.org/sqlite"
)

type fakeExecutor struct{ failStep string }

type fakeAIProvider struct {
	result ai.AnalysisResult
	err    error
}

func (f fakeAIProvider) Name() string { return "fake" }
func (f fakeAIProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (ai.AnalysisResult, error) {
	if f.err != nil {
		return ai.AnalysisResult{}, f.err
	}
	return f.result, nil
}
func (f fakeAIProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	return "ok", nil
}
func (f fakeAIProvider) AnalyzeMetrics(ctx context.Context, containerID string, metrics []any) (string, error) {
	return "ok", nil
}
func (f fakeAIProvider) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (ai.HealthAssessment, error) {
	return ai.HealthAssessment{Healthy: true, Confidence: 80, Summary: "healthy"}, nil
}

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
func (f fakeExecutor) Cleanup(ctx context.Context, req Request) error {
	if f.failStep == "cleanup" {
		return errors.New("cleanup failed")
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
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(store, fakeExecutor{failStep: failStep}, nil, nil, nil, nil, jobs.NewManager(1))
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

func TestStartUpdateRejectsDuplicateRunningJob(t *testing.T) {
	svc := newTestService(t, "")
	now := time.Now().UTC().Unix()
	if err := svc.store.CreateRun(context.Background(), gen.UpdateJobStatus{
		JobID:       "existing-run",
		ContainerID: "test-c",
		TargetImage: "img:v1",
		ValidateURL: "http://x",
		Status:      "running",
		CreatedAt:   now,
		UpdatedAt:   now,
		Error:       "",
		Steps:       []gen.UpdateStepEvent{},
	}); err != nil {
		t.Fatalf("create existing run: %v", err)
	}

	_, err := svc.StartUpdate(Request{ContainerID: "test-c", TargetImage: "img:v2", ValidateURL: "http://x"})
	if err == nil {
		t.Fatalf("expected duplicate running update to be rejected")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "running update job") {
		t.Fatalf("expected running update error, got: %v", err)
	}
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

func TestUpdatePipelineFailsOnHighAIRisk(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	aiSvc := ai.NewService(fakeAIProvider{
		result: ai.AnalysisResult{
			RiskScore: 90,
			RiskLevel: ai.RiskCritical,
			Summary:   "breaking schema migration",
		},
	})
	svc := NewService(store, fakeExecutor{}, aiSvc, nil, nil, nil, jobs.NewManager(1))

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
		if job != nil && job.Status == "failed" {
			seenReleaseFailure := false
			for _, step := range job.Steps {
				if step.Step == "release_analysis" && step.Status == "failed" {
					seenReleaseFailure = true
				}
				if step.Step == "backup" {
					t.Fatal("did not expect backup step when AI blocks the update")
				}
			}
			if !seenReleaseFailure {
				t.Fatal("expected release_analysis failure step")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for failed status")
}

func TestUpdatePipelineFailsOnAIBreakingChanges(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	aiSvc := ai.NewService(fakeAIProvider{
		result: ai.AnalysisResult{
			RiskScore:       20,
			RiskLevel:       ai.RiskLow,
			Summary:         "contains breaking config changes",
			BreakingChanges: []string{"config format changed"},
		},
	})
	svc := NewService(store, fakeExecutor{}, aiSvc, nil, nil, nil, jobs.NewManager(1))

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
		if job != nil && job.Status == "failed" {
			for _, step := range job.Steps {
				if step.Step == "backup" {
					t.Fatal("did not expect backup step when AI blocks due to breaking changes")
				}
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for failed status")
}

func TestUpdatePipelineFailsOnAIActionRequired(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	aiSvc := ai.NewService(fakeAIProvider{
		result: ai.AnalysisResult{
			RiskScore:      10,
			RiskLevel:      ai.RiskLow,
			Summary:        "manual migration required",
			ActionRequired: true,
		},
	})
	svc := NewService(store, fakeExecutor{}, aiSvc, nil, nil, nil, jobs.NewManager(1))

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
		if job != nil && job.Status == "failed" {
			for _, step := range job.Steps {
				if step.Step == "backup" {
					t.Fatal("did not expect backup step when AI marks action_required=true")
				}
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout waiting for failed status")
}
