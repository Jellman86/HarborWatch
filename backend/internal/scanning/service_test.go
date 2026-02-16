package scanning

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
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

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestServiceStartScanSuccess(t *testing.T) {
	store := newTestStore(t)
	svc := NewService(fakeScanner{result: Result{Source: "fake", High: 2, Medium: 1}}, nil, store)

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
	svc := NewService(fakeScanner{err: errors.New("boom")}, nil, store)

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
