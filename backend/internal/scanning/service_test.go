package scanning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type fakeScanner struct {
	result Result
	err    error
}

func (f fakeScanner) Name() string { return "fake" }

func (f fakeScanner) Scan(ctx context.Context, target string) (Result, error) {
	if f.err != nil {
		return Result{}, f.err
	}
	out := f.result
	out.Target = target
	if out.Scanned == 0 {
		out.Scanned = time.Now().UTC().Unix()
	}
	return out, nil
}

type fakeMalwareScanner struct{}

func (fakeMalwareScanner) Name() string { return "fake-malware" }

func (fakeMalwareScanner) ScanPath(ctx context.Context, path string) (MalwareResult, error) {
	return MalwareResult{Target: path, Source: "fake-malware", ScannedAt: time.Now().UTC().Unix()}, nil
}

type blockingScanner struct {
	started chan struct{}
}

func (b blockingScanner) Name() string { return "blocking" }

func (b blockingScanner) Scan(ctx context.Context, target string) (Result, error) {
	select {
	case <-b.started:
	default:
		close(b.started)
	}
	<-ctx.Done()
	return Result{}, fmt.Errorf("scan cancelled: %w", ctx.Err())
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

	store := NewStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store
}

func TestServiceStartScanSuccess(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(fakeScanner{result: Result{Source: "fake", High: 2, Medium: 1}}, nil, store, nil)

	started, err := svc.StartScan("nginx:latest")
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}
	if started.JobID == "" {
		t.Fatal("expected non-empty job ID")
	}

	var jobStatus string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := svc.Job(context.Background(), started.JobID)
		if err != nil {
			t.Fatalf("expected job %s: %v", started.JobID, err)
		}
		jobStatus = job.Status
		if jobStatus == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobStatus != "completed" {
		t.Fatalf("expected completed status, got %s", jobStatus)
	}

	summary, err := svc.LatestSummary(context.Background())
	if err != nil {
		t.Fatalf("load summary: %v", err)
	}
	if summary == nil || summary.Target != "nginx:latest" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.Total != 3 {
		t.Fatalf("expected total 3, got %d", summary.Total)
	}
}

func TestServiceStartScanFailure(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(fakeScanner{err: errors.New("boom")}, nil, store, nil)

	started, err := svc.StartScan("nginx:latest")
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}

	var jobStatus string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, _ := svc.Job(context.Background(), started.JobID)
		jobStatus = job.Status
		if jobStatus == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if jobStatus != "failed" {
		t.Fatalf("expected failed status, got %s", jobStatus)
	}
}

func TestServiceCancelRunningJob(t *testing.T) {
	store := newTestStore(t)
	started := make(chan struct{})
	svc := NewService(blockingScanner{started: started}, fakeMalwareScanner{}, store, nil)

	run, err := svc.StartScan("nginx:latest")
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not start")
	}

	job, err := svc.CancelJob(context.Background(), run.JobID)
	if err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if job.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", job.Status)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		current, err := svc.Job(context.Background(), run.JobID)
		if err != nil {
			t.Fatalf("job lookup failed: %v", err)
		}
		if current.Status == "cancelled" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	final, err := svc.Job(context.Background(), run.JobID)
	if err != nil {
		t.Fatalf("job lookup failed: %v", err)
	}
	t.Fatalf("expected cancelled status, got %s", final.Status)
}
