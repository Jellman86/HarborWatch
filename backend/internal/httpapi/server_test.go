package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
	_ "modernc.org/sqlite"
)

type fakeDockerClient struct {
	containers  []gen.ContainerSummary
	images      []gen.ImageSummary
	events      string
	logs        dockerengine.ContainerLogs
	logErr      error
	topology    dockerengine.NetworkTopologySnapshot
	topologyErr error
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
func (f fakeDockerClient) RestartContainer(ctx context.Context, id string) error {
	return nil
}
func (f fakeDockerClient) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	return f.images, nil
}
func (f fakeDockerClient) OpenEventStream(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.events)), nil
}
func (f fakeDockerClient) GetNetworkTopology(ctx context.Context) (dockerengine.NetworkTopologySnapshot, error) {
	if f.topologyErr != nil {
		return dockerengine.NetworkTopologySnapshot{}, f.topologyErr
	}
	return f.topology, nil
}

type fakeScanService struct {
	startResp  gen.ScanStartResponse
	jobs       map[string]gen.ScanJobStatus
	summary    *gen.ScanSummary
	activeJobs []gen.JobProgress
}

func (f fakeScanService) StartScan(target string) (gen.ScanStartResponse, error) {
	return f.startResp, nil
}
func (f fakeScanService) StartMalwareScan(target string) (gen.ScanStartResponse, error) {
	return f.startResp, nil
}
func (f fakeScanService) StartMalwareScanPath(targetLabel, containerName, scanPath string, cleanup bool) (gen.ScanStartResponse, error) {
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
func (f fakeScanService) ListJobs(ctx context.Context, scanType, targetPrefix string, limit int) ([]gen.ScanJobStatus, error) {
	out := make([]gen.ScanJobStatus, 0, len(f.jobs))
	for _, job := range f.jobs {
		if strings.TrimSpace(scanType) != "" && !strings.EqualFold(scanType, "malware") && !strings.EqualFold(scanType, "vulnerability") {
			continue
		}
		if strings.TrimSpace(targetPrefix) != "" && !strings.HasPrefix(job.Target, targetPrefix) {
			continue
		}
		out = append(out, job)
	}
	return out, nil
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
func (f fakeScanService) MalwareSummariesForContainer(ctx context.Context, containerID, containerName string) ([]gen.MalwareScanSummary, error) {
	return nil, nil
}
func (f fakeScanService) MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error) {
	return nil, nil
}
func (f fakeScanService) MalwareDetailsForContainer(ctx context.Context, containerID, containerName string, limit int) ([]gen.MalwareScanDetail, error) {
	return nil, nil
}
func (f fakeScanService) ActiveJobs() []gen.JobProgress {
	return f.activeJobs
}
func (f fakeScanService) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	return []gen.ImageSummary{}, nil
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
func (f fakeAuditService) ListAuditJobsForContainer(ctx context.Context, containerID, containerName string) ([]gen.AuditJobSummary, error) {
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
func (f fakeAIService) AnalyzeFleet(ctx context.Context, inventory string) (string, error) {
	return "ok", nil
}
func (f fakeAIService) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (ai.HealthAssessment, error) {
	return ai.HealthAssessment{Healthy: true, Confidence: 80, Summary: "healthy"}, nil
}
func (f fakeAIService) ListConversations(ctx context.Context, limit, offset int) ([]ai.ConversationRecord, error) {
	return []ai.ConversationRecord{}, nil
}
func (f fakeAIService) GetLatestFleetAdvice(ctx context.Context) (ai.FleetAdviceRecord, error) {
	return ai.FleetAdviceRecord{}, nil
}
func (f fakeAIService) SaveFleetAdvice(ctx context.Context, rec ai.FleetAdviceRecord) error {
	return nil
}

type fakePortainerClient struct {
	yaml string
	err  error
}

func (f fakePortainerClient) GetStackFile(ctx context.Context, id int) (string, error) {
	return f.yaml, f.err
}
func (f fakePortainerClient) GetStack(ctx context.Context, id int) (*portainer.Stack, error) {
	return &portainer.Stack{}, f.err
}
func (f fakePortainerClient) UpdateStack(ctx context.Context, id, eid int, yaml string, env []map[string]string, prune, pull bool) error {
	return f.err
}
func (f fakePortainerClient) ListStacks(ctx context.Context) ([]portainer.Stack, error) {
	return nil, f.err
}

type fakeComposeAuditHistoryStore struct {
	items []ai.ComposeAuditRecord
	byID  map[string]ai.ComposeAuditRecord
}

func (f *fakeComposeAuditHistoryStore) SaveComposeAudit(ctx context.Context, rec ai.ComposeAuditRecord) (ai.ComposeAuditRecord, error) {
	if f.byID == nil {
		f.byID = map[string]ai.ComposeAuditRecord{}
	}
	if rec.ID == "" {
		rec.ID = "rec-" + rec.ContainerID
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UTC().Unix()
	}
	f.byID[rec.ID] = rec
	f.items = append([]ai.ComposeAuditRecord{rec}, f.items...)
	return rec, nil
}

func (f *fakeComposeAuditHistoryStore) ListComposeAudits(ctx context.Context, containerID string, limit, offset int) ([]ai.ComposeAuditRecordSummary, error) {
	out := make([]ai.ComposeAuditRecordSummary, 0, len(f.items))
	for _, rec := range f.items {
		if rec.ContainerID != containerID {
			continue
		}
		out = append(out, ai.ComposeAuditRecordSummary{
			ID:            rec.ID,
			ContainerID:   rec.ContainerID,
			ContainerName: rec.ContainerName,
			Provider:      rec.Provider,
			Model:         rec.Model,
			Headline:      rec.Headline,
			CreatedAt:     rec.CreatedAt,
		})
	}
	if offset > len(out) {
		return []ai.ComposeAuditRecordSummary{}, nil
	}
	end := len(out)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return out[offset:end], nil
}

func (f *fakeComposeAuditHistoryStore) GetComposeAudit(ctx context.Context, id string) (ai.ComposeAuditRecord, error) {
	if rec, ok := f.byID[id]; ok {
		return rec, nil
	}
	return ai.ComposeAuditRecord{}, ai.ErrComposeAuditNotFound
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
func (f fakeDiagService) ListLogs(ctx context.Context, limit, offset int, level, source, search string, since int64) ([]diag.LogEntry, int, error) {
	var filtered []diag.LogEntry
	for _, entry := range f.logs {
		if level != "" && !strings.EqualFold(entry.Level, level) {
			continue
		}
		if source != "" && !strings.Contains(strings.ToLower(entry.Source), strings.ToLower(source)) {
			continue
		}
		if since > 0 && entry.Timestamp < since {
			continue
		}
		if search != "" {
			haystack := strings.ToLower(entry.Level + " " + entry.Source + " " + entry.Message)
			if !strings.Contains(haystack, strings.ToLower(search)) {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	total := len(filtered)
	if total == 0 {
		return nil, 0, nil
	}
	start := offset
	if start >= total {
		return nil, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
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

func (f fakeRulesService) Get(ctx context.Context, id, name string) (rules.ContainerRules, error) {
	return rules.ContainerRules{Exists: true, ContainerID: id, ContainerName: name, UpdatePolicy: "manual"}, nil
}
func (f fakeRulesService) Save(ctx context.Context, r rules.ContainerRules) error {
	return nil
}

type staticRulesService struct {
	rule rules.ContainerRules
}

func (f staticRulesService) Get(ctx context.Context, id, name string) (rules.ContainerRules, error) {
	out := f.rule
	if out.ContainerID == "" {
		out.ContainerID = id
	}
	if out.ContainerName == "" {
		out.ContainerName = name
	}
	out.Exists = true
	return out, nil
}

func (f staticRulesService) Save(ctx context.Context, r rules.ContainerRules) error {
	return nil
}

type recordingRulesService struct {
	getRule rules.ContainerRules
	saved   rules.ContainerRules
}

func (f *recordingRulesService) Get(ctx context.Context, id, name string) (rules.ContainerRules, error) {
	out := f.getRule
	if out.ContainerID == "" {
		out.ContainerID = id
	}
	if out.ContainerName == "" {
		out.ContainerName = name
	}
	out.Exists = true
	return out, nil
}

func (f *recordingRulesService) Save(ctx context.Context, r rules.ContainerRules) error {
	f.saved = r
	return nil
}

type fakeUpdateService struct {
	startResp  gen.UpdateStartResponse
	job        *gen.UpdateJobStatus
	runsByID   map[string][]gen.UpdateJobStatus
	activeJobs []gen.JobProgress
}

func (f fakeUpdateService) StartUpdate(req updates.Request) (gen.UpdateStartResponse, error) {
	return f.startResp, nil
}
func (f fakeUpdateService) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return f.job, nil
}
func (f fakeUpdateService) ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	if f.runsByID != nil {
		return f.runsByID[containerID], nil
	}
	return []gen.UpdateJobStatus{}, nil
}
func (f fakeUpdateService) ActiveJobs() []gen.JobProgress {
	return f.activeJobs
}
func (f fakeUpdateService) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent, 1)
	ch <- gen.UpdateStepEvent{JobID: jobID, Step: "preflight", Status: "completed", Message: "ok", Timestamp: 1}
	close(ch)
	return ch, func() {}
}

type panicUpdateService struct{}

func (panicUpdateService) StartUpdate(req updates.Request) (gen.UpdateStartResponse, error) {
	panic("StartUpdate should not be called")
}
func (panicUpdateService) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return nil, nil
}
func (panicUpdateService) ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	return []gen.UpdateJobStatus{}, nil
}
func (panicUpdateService) ActiveJobs() []gen.JobProgress {
	return nil
}
func (panicUpdateService) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent)
	close(ch)
	return ch, func() {}
}

func TestHealthEndpoint(t *testing.T) {
	ts := httptest.NewServer(NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health failed: %v status=%d", err, resp.StatusCode)
	}
}

func TestDockerContainersEndpoint(t *testing.T) {
	mux := NewMuxWithDeps(nil, fakeDockerClient{containers: []gen.ContainerSummary{{ID: "abc", Image: "nginx:latest", State: "running"}}}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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

func TestDockerContainerRulesSummaryEndpoint(t *testing.T) {
	rulesSvc := staticRulesService{
		rule: rules.ContainerRules{
			UpdatePolicy:       "auto",
			InheritAutomation:  true,
			UpgradesAutomation: true,
		},
	}
	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{ID: "c1", Names: []string{"/web"}, Image: "nginx:latest", State: "running", Labels: map[string]string{}},
			{ID: "c2", Names: []string{"/db"}, Image: "postgres:16", State: "running", Labels: map[string]string{}},
		},
	}
	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, rulesSvc, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/containers/rules-summary", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0]["containerId"] == nil {
		t.Fatalf("expected containerId field in row")
	}
	if got[0]["updatePolicy"] != "auto" {
		t.Fatalf("expected updatePolicy=auto, got %#v", got[0]["updatePolicy"])
	}
}

func TestContainerDetailIncludesLifecycleFlags(t *testing.T) {
	rulesSvc := staticRulesService{
		rule: rules.ContainerRules{
			UpdatePolicy:    "auto",
			BypassAI:        true,
			SkipHealthCheck: true,
		},
	}
	mux := NewMuxWithDeps(nil, fakeDockerClient{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, rulesSvc, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/c1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got gen.ContainerDetail
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.Rules == nil {
		t.Fatalf("expected rules in container detail")
	}
	if !got.Rules.BypassAI {
		t.Fatalf("expected bypassAi=true in container detail rules")
	}
	if !got.Rules.SkipHealthCheck {
		t.Fatalf("expected skipHealthCheck=true in container detail rules")
	}
}

func TestReleaseSummaryEndpoint(t *testing.T) {
	fake := fakeReleaseService{summary: gen.ReleaseRiskSummary{Repo: "Jellman86/HarborWatch", TotalRisk: 42}}
	mux := NewMuxWithDeps(nil, nil, nil, fake, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/releases/summary?repo=Jellman86/HarborWatch", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestUpdateEndpoints(t *testing.T) {
	up := fakeUpdateService{startResp: gen.UpdateStartResponse{JobID: "u1", Status: "running"}, job: &gen.UpdateJobStatus{JobID: "u1", Status: "running"}}
	mux := NewMuxWithDeps(nil, nil, nil, nil, up, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)

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

func TestUpdateAIBlockedEndpoint(t *testing.T) {
	up := fakeUpdateService{
		runsByID: map[string][]gen.UpdateJobStatus{
			"c1": {
				{
					JobID:       "u-ai-block",
					ContainerID: "c1",
					Status:      "failed",
					Error:       "AI blocked update: risk_score=95 (threshold=80)",
					UpdatedAt:   1700000001,
					AIAnalysis: &gen.AIAnalysisSummary{
						RiskScore: 95,
						RiskLevel: "Critical",
					},
					Steps: []gen.UpdateStepEvent{
						{Step: "release_analysis", Status: "failed", Message: "AI blocked update: risk_score=95 (threshold=80)"},
					},
				},
			},
			"c2": {
				{
					JobID:       "u-generic-fail",
					ContainerID: "c2",
					Status:      "failed",
					Error:       "validate failed",
					UpdatedAt:   1700000002,
				},
			},
		},
	}
	docker := fakeDockerClient{
		containers: []gen.ContainerSummary{
			{ID: "c1", Names: []string{"/one"}, Image: "nginx:latest", State: "running"},
			{ID: "c2", Names: []string{"/two"}, Image: "redis:latest", State: "running"},
		},
	}
	mux := NewMuxWithDeps(nil, docker, nil, nil, up, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/updates/ai-blocked", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	entry, ok := payload["c1"]
	if !ok {
		t.Fatalf("expected c1 to have ai blocked signal, got %#v", payload)
	}
	if asBool(entry["blocked"]) != true {
		t.Fatalf("expected blocked=true for c1, got %#v", entry["blocked"])
	}
	if asInt(entry["riskScore"]) != 95 {
		t.Fatalf("expected riskScore=95, got %#v", entry["riskScore"])
	}
	if _, exists := payload["c2"]; exists {
		t.Fatalf("expected c2 to be absent (generic failure), got %#v", payload["c2"])
	}
}

func TestUpdateRunEndpoint_BlockedWhenPolicyLocked(t *testing.T) {
	mux := NewMuxWithDeps(nil,
		nil,
		nil,
		nil,
		panicUpdateService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		fakePortainerClient{},
		staticRulesService{rule: rules.ContainerRules{
			UpdatePolicy: "locked",
			ValidateURL:  "http://x",
		}},
		nil,
		nil,
	)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"containerId":"c1","targetImage":"img","validateUrl":"http://x"}`)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/updates/run", body))
	if rec.Code != http.StatusLocked {
		t.Fatalf("expected 423 when policy is locked, got %d", rec.Code)
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
	mux := NewMuxWithDeps(nil, nil, scans, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)

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

func TestScanJobsListEndpoint(t *testing.T) {
	scans := fakeScanService{
		jobs: map[string]gen.ScanJobStatus{
			"s1": {
				JobID:     "s1",
				Target:    "container:c1:rootfs",
				Status:    "running",
				Source:    "clamav",
				StartedAt: time.Now().UTC().Unix(),
			},
		},
	}
	mux := NewMuxWithDeps(nil, nil, scans, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/scans/jobs?type=malware&prefix=container:c1&limit=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var jobs []gen.ScanJobStatus
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&jobs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "s1" {
		t.Fatalf("unexpected jobs payload: %#v", jobs)
	}
}

func TestAuditComposeByID_DockerUnavailable(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, fakeAIService{enabled: true}, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/audit-compose/c1", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAuditComposeByID_PersistsAndReturnsHTML(t *testing.T) {
	history := &fakeComposeAuditHistoryStore{}
	mux := newMuxWithDepsAndComposeAuditStore(nil,
		fakeDockerClient{},
		nil,
		nil,
		nil,
		nil,
		fakeAIService{enabled: true},
		nil,
		nil,
		nil,
		nil,
		nil,
		fakePortainerClient{},
		fakeRulesService{},
		nil,
		history,
		nil,
	)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/audit-compose/c1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body["persisted"] != true {
		t.Fatalf("expected persisted=true, got %#v", body["persisted"])
	}
	if strings.TrimSpace(asString(body["analysisHtml"])) == "" {
		t.Fatalf("expected analysisHtml in response")
	}
	if len(history.items) != 1 {
		t.Fatalf("expected 1 persisted history item, got %d", len(history.items))
	}
}

func TestAuditComposeHistoryEndpoints(t *testing.T) {
	history := &fakeComposeAuditHistoryStore{
		byID: map[string]ai.ComposeAuditRecord{
			"rec1": {
				ID:               "rec1",
				ContainerID:      "c1",
				ContainerName:    "frigate",
				Provider:         "openai",
				Model:            "gpt-5.2",
				ComposeConfig:    "version: '3'",
				AnalysisMarkdown: "# Findings",
				AnalysisHTML:     "<h1>Findings</h1>",
				CreatedAt:        1700000000,
			},
		},
		items: []ai.ComposeAuditRecord{{
			ID:               "rec1",
			ContainerID:      "c1",
			ContainerName:    "frigate",
			Provider:         "openai",
			Model:            "gpt-5.2",
			Headline:         "Findings",
			ComposeConfig:    "version: '3'",
			AnalysisMarkdown: "# Findings",
			AnalysisHTML:     "<h1>Findings</h1>",
			CreatedAt:        1700000000,
		}},
	}
	mux := newMuxWithDepsAndComposeAuditStore(nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		fakeAIService{enabled: true},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		fakeRulesService{},
		nil,
		history,
		nil,
	)

	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, httptest.NewRequest(http.MethodGet, "/api/ai/audit-compose/c1/history?limit=5", nil))
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 list status, got %d", recList.Code)
	}
	var list []map[string]any
	if err := json.NewDecoder(bytes.NewReader(recList.Body.Bytes())).Decode(&list); err != nil {
		t.Fatalf("decode list failed: %v", err)
	}
	if len(list) != 1 || asString(list[0]["id"]) != "rec1" {
		t.Fatalf("expected rec1 in list, got %#v", list)
	}

	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, httptest.NewRequest(http.MethodGet, "/api/ai/audit-compose/history/rec1", nil))
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 record status, got %d", recGet.Code)
	}
	var record map[string]any
	if err := json.NewDecoder(bytes.NewReader(recGet.Body.Bytes())).Decode(&record); err != nil {
		t.Fatalf("decode record failed: %v", err)
	}
	if asString(record["id"]) != "rec1" || strings.TrimSpace(asString(record["analysisHtml"])) == "" {
		t.Fatalf("unexpected record payload: %#v", record)
	}
}

func TestRulesRoute_BackwardCompatibleContainersPath(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docker/containers/c1/rules", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRulesRoute_SaveMergesPartialPayloadWithoutClearingUnhealthyRestart(t *testing.T) {
	rulesSvc := &recordingRulesService{
		getRule: rules.ContainerRules{
			ContainerID:                 "c1",
			ContainerName:               "web",
			UpdatePolicy:                "manual",
			AutoRollback:                true,
			InheritAutomation:           false,
			UpgradesAutomation:          false,
			MaintenanceAutomation:       false,
			SecurityAutomation:          false,
			RestartOnUnhealthy:          true,
			UnhealthyRestartCooldownSec: 600,
		},
	}
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, rulesSvc, nil, nil)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"updatePolicy":"auto","inheritAutomation":true,"upgradesAutomation":true}`)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/docker/c1/rules", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	if !rulesSvc.saved.RestartOnUnhealthy {
		t.Fatalf("expected restartOnUnhealthy to be preserved on partial save")
	}
	if rulesSvc.saved.UnhealthyRestartCooldownSec != 600 {
		t.Fatalf("expected cooldown to be preserved, got %d", rulesSvc.saved.UnhealthyRestartCooldownSec)
	}
	if rulesSvc.saved.UpdatePolicy != "auto" {
		t.Fatalf("expected updatePolicy=auto, got %q", rulesSvc.saved.UpdatePolicy)
	}
}

func TestFleetAdviceEndpoint(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	if strings.TrimSpace(payload["adviceMarkdown"]) == "" {
		t.Fatalf("expected non-empty adviceMarkdown payload")
	}
	if strings.TrimSpace(payload["adviceHtml"]) == "" {
		t.Fatalf("expected non-empty adviceHtml payload")
	}
}

func TestFleetAdviceGetEndpointIncludesMarkdownFields(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ai/fleet-advice", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if _, ok := payload["advice"]; !ok {
		t.Fatalf("expected advice key in payload")
	}
	if _, ok := payload["adviceMarkdown"]; !ok {
		t.Fatalf("expected adviceMarkdown key in payload")
	}
	if _, ok := payload["adviceHtml"]; !ok {
		t.Fatalf("expected adviceHtml key in payload")
	}
}

func TestAIUsageEndpointGracefulWithoutUsageStore(t *testing.T) {
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, fakeAIService{enabled: false}, nil, nil, nil, fakeNotificationService{}, fakeSettingsService{}, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, sched, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, m, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	mux := NewMuxWithDeps(nil, docker, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	mux := NewMuxWithDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, diagSvc, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/system/logs?limit=10&level=error&source=docker&since=250", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload struct {
		Logs  []diag.LogEntry `json:"logs"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload.Logs) != 1 {
		t.Fatalf("expected 1 filtered log, got %d", len(payload.Logs))
	}
	if payload.Logs[0].Level != "ERROR" {
		t.Fatalf("expected ERROR log, got %q", payload.Logs[0].Level)
	}
	if payload.Total != 1 {
		t.Fatalf("expected total 1, got %d", payload.Total)
	}
}

func TestDiagnosticsSnapshotEndpoint(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "diag.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrations.Run failed: %v", err)
	}
	expectedSchemaVersion, err := migrations.CurrentVersion(context.Background(), db)
	if err != nil {
		t.Fatalf("migrations.CurrentVersion failed: %v", err)
	}

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
	mux := NewMuxWithDeps(db, docker, fakeScanService{}, nil, nil, fakeAuditService{}, nil, fakeSchedulerService{}, nil, diagSvc, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)
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
	if got := asInt(payload["schemaVersion"]); got != expectedSchemaVersion {
		t.Fatalf("expected schemaVersion=%d in diagnostics snapshot, got %v", expectedSchemaVersion, payload["schemaVersion"])
	}
}

func TestDiagnosticsSnapshotDedupesActiveJobsByID(t *testing.T) {
	sharedJobs := []gen.JobProgress{
		{ID: "j1", Type: "scan:trivy", Target: "nginx:latest", Status: "queued", Progress: 0, StartedAt: 1},
		{ID: "j2", Type: "scan:clamav", Target: "/mnt/media", Status: "running", Progress: 25, StartedAt: 2},
	}
	snapshot, err := collectDiagnosticsSnapshot(context.Background(), diagnosticsDeps{
		db:            nil,
		scanService:   fakeScanService{activeJobs: sharedJobs},
		updateService: fakeUpdateService{activeJobs: sharedJobs},
	}, diagnosticsSnapshotOptions{LogLimit: 10, AuditLimit: 10})
	if err != nil {
		t.Fatalf("collectDiagnosticsSnapshot failed: %v", err)
	}
	if len(snapshot.ActiveJobs) != 2 {
		t.Fatalf("expected 2 deduped active jobs, got %d (%#v)", len(snapshot.ActiveJobs), snapshot.ActiveJobs)
	}
	if snapshot.ActiveJobs[0].ID != "j1" || snapshot.ActiveJobs[1].ID != "j2" {
		t.Fatalf("unexpected activeJobs order/content: %#v", snapshot.ActiveJobs)
	}
}

func TestNetworksTopologyRoute(t *testing.T) {
	mux := NewMuxWithDeps(nil, fakeDockerClient{
		topology: dockerengine.NetworkTopologySnapshot{
			GeneratedAt: 1700000000,
			Networks: []dockerengine.NetworkTopologyNetwork{
				{ID: "n1", Name: "app_net", Driver: "bridge"},
			},
			Containers: []dockerengine.NetworkTopologyContainer{
				{ID: "c1", Name: "web"},
			},
			Edges: []dockerengine.NetworkTopologyEdge{
				{NetworkID: "n1", ContainerID: "c1", IPv4Address: "172.20.0.2/16"},
			},
		},
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fakePortainerClient{}, fakeRulesService{}, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/networks/topology", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Networks []map[string]any `json:"networks"`
		Edges    []map[string]any `json:"edges"`
	}
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Networks) != 1 || len(payload.Edges) != 1 {
		t.Fatalf("unexpected topology payload: %s", rec.Body.String())
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	default:
		return false
	}
}

func asInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
