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
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
)

type fakeDockerClient struct {
	containers []gen.ContainerSummary
	images     []gen.ImageSummary
	events     string
	logs       dockerengine.ContainerLogs
	logErr     error
}

func (f fakeDockerClient) ListContainers(ctx context.Context) ([]gen.ContainerSummary, error) {
	return f.containers, nil
}
func (f fakeDockerClient) GetContainer(ctx context.Context, id string) (gen.ContainerSummary, error) {
	return gen.ContainerSummary{ID: id}, nil
}
func (f fakeDockerClient) GetContainerLogs(ctx context.Context, id string, tail int, since time.Time, timestamps bool) (dockerengine.ContainerLogs, error) {
	if f.logErr != nil {
		return dockerengine.ContainerLogs{}, f.logErr
	}
	out := f.logs
	out.ContainerID = id
	if out.Combined == "" {
		out.Combined = "line1\nline2"
		out.LineCount = 2
	}
	out.Tail = tail
	out.Since = since.Unix()
	out.Timestamps = timestamps
	return out, nil
}
func (f fakeDockerClient) GetContainerComposeConfig(ctx context.Context, id string, ps *portainer.Client) (string, error) {
	return "version: '3'", nil
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
func (f fakeScanService) StartMalwareScanPath(targetLabel, scanPath string, cleanup bool) (gen.ScanStartResponse, error) {
	return f.startResp, nil
}
func (f fakeScanService) CancelJob(ctx context.Context, jobID string) (gen.ScanJobStatus, error) {
	j, ok := f.jobs[jobID]
	if !ok {
		return gen.ScanJobStatus{}, errors.New("not found")
	}
	j.Status = "cancelled"
	j.Error = "cancelled by user"
	return j, nil
}
func (f fakeScanService) ClamAVSignatureStatus(ctx context.Context) (scanning.ClamAVSignatureStatus, error) {
	return scanning.ClamAVSignatureStatus{EngineVersion: "ClamAV 1.4.0"}, nil
}
func (f fakeScanService) UpdateClamAVSignatures(ctx context.Context) (string, error) {
	return "up to date", nil
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
func (f fakeScanService) LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error) {
	return f.summary, nil
}
func (f fakeScanService) LatestDetailsForTarget(ctx context.Context, target string) (*gen.TrivyScanDetails, error) {
	return nil, nil
}
func (f fakeScanService) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	return nil, nil
}
func (f fakeScanService) MalwareSummariesForContainer(ctx context.Context, containerID string) ([]gen.MalwareScanSummary, error) {
	return nil, nil
}
func (f fakeScanService) MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error) {
	return nil, nil
}
func (f fakeScanService) MalwareDetailsForContainer(ctx context.Context, containerID string, limit int) ([]gen.MalwareScanDetail, error) {
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
func (f fakeAuditService) ListAuditJobsForContainer(ctx context.Context, containerID string) ([]gen.AuditJobSummary, error) {
	return f.jobs, nil
}
func (f fakeAuditService) GetAuditJobSteps(ctx context.Context, id string) ([]gen.UpdateStepEvent, error) {
	return nil, nil
}

type fakeAIService struct{ enabled bool }

func (f fakeAIService) HasProvider() bool { return f.enabled }
func (f fakeAIService) AnalyzeReleaseNotes(ctx context.Context, notes string) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (f fakeAIService) AuditCompose(ctx context.Context, yaml string) (string, error) {
	return "ok", nil
}
func (f fakeAIService) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	return "ok", nil
}
func (f fakeAIService) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (ai.HealthAssessment, error) {
	return ai.HealthAssessment{Healthy: true, Confidence: 80, Summary: "healthy"}, nil
}

type fakeSchedulerService struct{}

func (f fakeSchedulerService) AddTask(spec string, task scheduler.Task, enabled bool) error {
	return nil
}
func (f fakeSchedulerService) RemoveTask(name string) {}
func (f fakeSchedulerService) ToggleTask(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (f fakeSchedulerService) UpdateTaskSchedule(ctx context.Context, name string, spec string) error {
	return nil
}
func (f fakeSchedulerService) RunTask(ctx context.Context, name string) error { return nil }
func (f fakeSchedulerService) ListSchedules(ctx context.Context) ([]scheduler.ScheduleEntry, error) {
	return []scheduler.ScheduleEntry{}, nil
}

type fakeSchedulerServiceWithRun struct {
	fakeSchedulerService
	lastRunName string
	runErr      error
}

func (f *fakeSchedulerServiceWithRun) RunTask(ctx context.Context, name string) error {
	f.lastRunName = name
	return f.runErr
}

type fakeMetricsService struct{}

func (f fakeMetricsService) GetMetrics(ctx context.Context, id, dur string) ([]metrics.Metric, error) {
	return nil, nil
}
func (f fakeMetricsService) GetCollectorTask() *metrics.Collector { return nil }
func (f fakeMetricsService) GetPruneTask() *metrics.PruneTask     { return nil }

type fakeMetricsServiceWithData struct {
	fakeMetricsService
	data map[string][]metrics.Metric
}

func (f fakeMetricsServiceWithData) GetMetrics(ctx context.Context, id, dur string) ([]metrics.Metric, error) {
	if f.data == nil {
		return []metrics.Metric{}, nil
	}
	return f.data[id], nil
}

type fakeDiagService struct {
	logs []diag.LogEntry
}

func (f fakeDiagService) Log(level, source, message string) {}
func (f fakeDiagService) ListLogs(ctx context.Context, limit int) ([]diag.LogEntry, error) {
	if len(f.logs) == 0 {
		return nil, nil
	}
	if len(f.logs) > limit {
		return f.logs[:limit], nil
	}
	return f.logs, nil
}
func (f fakeDiagService) GetSystemStatus() diag.SystemStatus {
	return diag.SystemStatus{Uptime: 42, NumGoroutine: 9}
}
func (f fakeDiagService) PruneLogs(ctx context.Context, olderThan int64) (int64, error) {
	return 0, nil
}

type fakeNotificationService struct{}

func (f fakeNotificationService) Dispatch(ctx context.Context, msg notifications.Message) {}
func (f fakeNotificationService) AddDispatcher(d notifications.Dispatcher)                {}
func (f fakeNotificationService) RemoveDispatcher(name string)                            {}

type fakeSettingsService struct{}

func (f fakeSettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return settings.Settings{}, nil
}
func (f fakeSettingsService) Save(ctx context.Context, s settings.Settings) error {
	return nil
}

type fakeRulesService struct{}

func (f fakeRulesService) Get(ctx context.Context, id string) (rules.ContainerRules, error) {
	return rules.ContainerRules{ContainerID: id, UpdatePolicy: "manual"}, nil
}
func (f fakeRulesService) Save(ctx context.Context, r rules.ContainerRules) error {
	return nil
}

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
func (f fakeUpdateService) ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	return []gen.UpdateJobStatus{}, nil
}
func (f fakeUpdateService) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent, 1)
	ch <- gen.UpdateStepEvent{JobID: jobID, Step: "preflight", Status: "completed", Message: "ok", Timestamp: 1}
	close(ch)
	return ch, func() {}
}

func TestHealthEndpoint(t *testing.T) {
	ts := httptest.NewServer(NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health failed: %v status=%d", err, resp.StatusCode)
	}
}

func TestDockerContainersEndpoint(t *testing.T) {
	mux := NewMuxWithDeps(fakeDockerClient{containers: []gen.ContainerSummary{{ID: "abc", Image: "nginx:latest", State: "running"}}}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
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
	mux := NewMuxWithDeps(nil, nil, fake, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/releases/summary?repo=Jellman86/HarborWatch", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpdateEndpoints(t *testing.T) {
	up := fakeUpdateService{startResp: gen.UpdateStartResponse{JobID: "u1", Status: "running"}, job: &gen.UpdateJobStatus{JobID: "u1", Status: "running"}}
	mux := NewMuxWithDeps(nil, nil, nil, up, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)

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

func TestScanCancelEndpoint(t *testing.T) {
	scans := fakeScanService{
		jobs: map[string]gen.ScanJobStatus{
			"s1": {
				JobID:     "s1",
				Target:    "nginx:latest",
				Status:    "running",
				Source:    "trivy",
				StartedAt: time.Now().UTC().Unix(),
			},
		},
	}
	mux := NewMuxWithDeps(nil, scans, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/scans/jobs/s1/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body gen.ScanJobStatus
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "cancelled" {
		t.Fatalf("expected cancelled job status, got %q", body.Status)
	}
}

func TestAuditComposeByID_DockerUnavailable(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, fakeAIService{enabled: true}, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/audit-compose/c1", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestRulesRoute_BackwardCompatibleContainersPath(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/containers/c1/rules", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestFleetAdviceEndpoint(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	body := strings.NewReader(`[{"id":"c1","names":["/web"],"image":"nginx:latest","state":"running","status":"Up","labels":{},"updateAvailable":true}]`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/ai/fleet-advice", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]string
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if strings.TrimSpace(payload["advice"]) == "" {
		t.Fatalf("expected non-empty advice payload")
	}
}

func TestAIUsageEndpointGracefulWithoutUsageStore(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, fakeAIService{enabled: false}, nil, nil, nil, fakeNotificationService{}, fakeSettingsService{}, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/usage?span=7d", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body["span"] != "7d" {
		t.Fatalf("expected span 7d, got %#v", body["span"])
	}
}

func TestDockerPruneEndpoint_TriggersSchedulerTask(t *testing.T) {
	sched := &fakeSchedulerServiceWithRun{}
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, sched, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/docker/prune", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if sched.lastRunName != "docker_system_prune" {
		t.Fatalf("expected docker_system_prune task trigger, got %q", sched.lastRunName)
	}
}

func TestMetricsBatchEndpoint(t *testing.T) {
	m := fakeMetricsServiceWithData{
		data: map[string][]metrics.Metric{
			"c1": {{ContainerID: "c1", Timestamp: 1, CPUPercent: 10}},
			"c2": {{ContainerID: "c2", Timestamp: 2, CPUPercent: 20}},
		},
	}
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, m, nil, nil, nil, nil, fakeRulesService{}, nil)
	body := strings.NewReader(`{"ids":["c1","c2"],"duration":"1h"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/metrics/batch", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string][]metrics.Metric
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload["c1"]) != 1 || len(payload["c2"]) != 1 {
		t.Fatalf("expected metrics for c1 and c2, got %#v", payload)
	}
}

func TestDockerContainerLogsEndpoint(t *testing.T) {
	docker := fakeDockerClient{
		logs: dockerengine.ContainerLogs{
			Combined: "hello\nworld",
		},
	}
	mux := NewMuxWithDeps(docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/c1/logs?tail=10&since=1h&timestamps=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload dockerengine.ContainerLogs
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload.ContainerID != "c1" {
		t.Fatalf("expected container id c1, got %q", payload.ContainerID)
	}
	if payload.Tail != 10 {
		t.Fatalf("expected tail 10, got %d", payload.Tail)
	}
}

func TestSystemLogsFilters(t *testing.T) {
	diagSvc := fakeDiagService{
		logs: []diag.LogEntry{
			{Timestamp: 200, Level: "INFO", Source: "A", Message: "ok"},
			{Timestamp: 300, Level: "ERROR", Source: "Docker", Message: "boom"},
		},
	}
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, diagSvc, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/logs?limit=10&level=error&source=docker&since=250", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload []diag.LogEntry
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 filtered log, got %d", len(payload))
	}
	if payload[0].Level != "ERROR" {
		t.Fatalf("expected ERROR log, got %q", payload[0].Level)
	}
}

func TestDiagnosticsSnapshotEndpoint(t *testing.T) {
	diagSvc := fakeDiagService{
		logs: []diag.LogEntry{
			{Timestamp: 300, Level: "ERROR", Source: "Scanner", Message: "failed"},
		},
	}
	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{{ID: "c1", State: "running", Image: "nginx:latest"}},
		images:     []gen.ImageSummary{{ID: "img1", RepoTags: []string{"nginx:latest"}}},
		logs:       dockerengine.ContainerLogs{Combined: "x"},
	}
	mux := NewMuxWithDeps(docker, fakeScanService{}, nil, nil, fakeAuditService{}, nil, fakeSchedulerService{}, nil, diagSvc, nil, nil, nil, fakeRulesService{}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/diagnostics/snapshot?containerId=c1&includeFleet=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["generatedAt"] == nil {
		t.Fatalf("expected generatedAt in diagnostics snapshot")
	}
	if payload["components"] == nil {
		t.Fatalf("expected components in diagnostics snapshot")
	}
}
