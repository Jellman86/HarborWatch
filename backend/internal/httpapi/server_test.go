package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
)

type fakeDockerClient struct {
	containers []gen.ContainerSummary
	images     []gen.ImageSummary
	events     string
}

func (f fakeDockerClient) ListContainers(ctx context.Context) ([]gen.ContainerSummary, error) {
	return f.containers, nil
}
func (f fakeDockerClient) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	return f.images, nil
}
func (f fakeDockerClient) OpenEventStream(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.events)), nil
}

type fakeScanService struct {
	startResp gen.ScanStartResponse
	jobs      map[string]gen.ScanJobStatus
	summary   *gen.ScanSummary
}

func (f fakeScanService) StartScan(target string) (gen.ScanStartResponse, error) {
	return f.startResp, nil
}
func (f fakeScanService) StartMalwareScan(target string) (gen.ScanStartResponse, error) {
	return f.startResp, nil
}
func (f fakeScanService) Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error) {
	j, ok := f.jobs[jobID]
	if !ok {
		return gen.ScanJobStatus{}, errors.New("not found")
	}
	return j, nil
}
func (f fakeScanService) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	return f.summary, nil
}
func (f fakeScanService) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	return nil, nil
}

type fakeReleaseService struct{ summary gen.ReleaseRiskSummary }

func (f fakeReleaseService) Analyze(ctx context.Context, repo string) (gen.ReleaseRiskSummary, error) {
	return f.summary, nil
}

type fakeAuditService struct {
	jobs []gen.AuditJobSummary
}

func (f fakeAuditService) ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error) {
	return f.jobs, nil
}

type fakeAIService struct{ enabled bool }

func (f fakeAIService) HasProvider() bool { return f.enabled }
func (f fakeAIService) AnalyzeReleaseNotes(ctx context.Context, notes string) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (f fakeAIService) AuditCompose(ctx context.Context, yaml string) (string, error) {
	return "ok", nil
}

type fakeSchedulerService struct{}

func (f fakeSchedulerService) AddTask(spec string, task scheduler.Task) error { return nil }
func (f fakeSchedulerService) RemoveTask(name string)                         {}

type fakeUpdateService struct {
	startResp gen.UpdateStartResponse
	job       *gen.UpdateJobStatus
}

func (f fakeUpdateService) StartUpdate(req updates.Request) (gen.UpdateStartResponse, error) {
	return f.startResp, nil
}
func (f fakeUpdateService) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return f.job, nil
}
func (f fakeUpdateService) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent, 1)
	ch <- gen.UpdateStepEvent{JobID: jobID, Step: "preflight", Status: "completed", Message: "ok", Timestamp: 1}
	close(ch)
	return ch, func() {}
}

func TestHealthEndpoint(t *testing.T) {
	ts := httptest.NewServer(NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health failed: %v status=%d", err, resp.StatusCode)
	}
}

func TestDockerContainersEndpoint(t *testing.T) {
	mux := NewMuxWithDeps(fakeDockerClient{containers: []gen.ContainerSummary{{ID: "abc", Image: "nginx:latest", State: "running"}}}, nil, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/containers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var got []gen.ContainerSummary
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil || len(got) != 1 {
		t.Fatalf("decode/len failed: %v %#v", err, got)
	}
}

func TestReleaseSummaryEndpoint(t *testing.T) {
	fake := fakeReleaseService{summary: gen.ReleaseRiskSummary{Repo: "Jellman86/HarborWatch", TotalRisk: 42}}
	mux := NewMuxWithDeps(nil, nil, fake, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/releases/summary?repo=Jellman86/HarborWatch", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpdateEndpoints(t *testing.T) {
	up := fakeUpdateService{startResp: gen.UpdateStartResponse{JobID: "u1", Status: "running"}, job: &gen.UpdateJobStatus{JobID: "u1", Status: "running"}}
	mux := NewMuxWithDeps(nil, nil, nil, up, nil, nil, nil)

	recRun := httptest.NewRecorder()
	mux.ServeHTTP(recRun, httptest.NewRequest(http.MethodPost, "/api/updates/run", strings.NewReader(`{"containerId":"c1","targetImage":"img","validateUrl":"http://x"}`)))
	if recRun.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", recRun.Code)
	}

	recJob := httptest.NewRecorder()
	mux.ServeHTTP(recJob, httptest.NewRequest(http.MethodGet, "/api/updates/jobs/u1", nil))
	if recJob.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recJob.Code)
	}

	recEvents := httptest.NewRecorder()
	mux.ServeHTTP(recEvents, httptest.NewRequest(http.MethodGet, "/api/updates/events/u1", nil))
	if recEvents.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recEvents.Code)
	}
}
