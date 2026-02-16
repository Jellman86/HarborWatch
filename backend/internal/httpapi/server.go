package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/audit"
	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/releases"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
)

type DockerClient interface {
	ListContainers(ctx context.Context) ([]gen.ContainerSummary, error)
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
	OpenEventStream(ctx context.Context) (io.ReadCloser, error)
}

type ScanService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
	Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error)
	LatestSummary(ctx context.Context) (*gen.ScanSummary, error)
	MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error)
}

type ReleaseService interface {
	Analyze(ctx context.Context, repo string) (gen.ReleaseRiskSummary, error)
}

type AIService interface {
	HasProvider() bool
	AnalyzeReleaseNotes(ctx context.Context, notes string) (ai.AnalysisResult, error)
	AuditCompose(ctx context.Context, yaml string) (string, error)
	AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error)
}

type DiagService interface {
	Log(level, source, message string)
	ListLogs(ctx context.Context, limit int) ([]diag.LogEntry, error)
	GetSystemStatus() diag.SystemStatus
}

type AuditService interface {
	ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error)
	GetAuditJobSteps(ctx context.Context, id string) ([]gen.UpdateStepEvent, error)
}

type SchedulerService interface {
	AddTask(spec string, task scheduler.Task) error
	RemoveTask(name string)
	ToggleTask(ctx context.Context, name string, enabled bool) error
	RunTask(ctx context.Context, name string) error
	ListSchedules(ctx context.Context) ([]scheduler.ScheduleEntry, error)
}

type MetricsService interface {
	GetMetrics(ctx context.Context, containerID string, duration string) ([]metrics.Metric, error)
}

type UpdateService interface {
	StartUpdate(req updates.Request) (gen.UpdateStartResponse, error)
	GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error)
	Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func())
}

func NewMux() http.Handler {
	mux, _ := NewMuxWithScheduler()
	return mux
}

func NewMuxWithScheduler() (http.Handler, *scheduler.Service) {
	dbPath := os.Getenv("HARBORWATCH_DB_PATH")
	if dbPath == "" {
		dbPath = "/tmp/harborwatch.db"
	}

	// 1. Diagnostics Setup (First, to capture other init logs)
	diagService, _ := diag.NewService(dbPath)
	if diagService != nil {
		diagService.Log("INFO", "System", "HarborWatch initializing...")
	}

	dockerClient, err := dockerengine.NewFromEnv()
	if err != nil {
		dockerClient = nil
	}
	scanService, err := scanning.NewServiceFromEnv()
	if err != nil {
		scanService = nil
	}
	releaseService := releases.NewService()
	updateService, err := updates.NewServiceFromEnv()
	if err != nil {
		updateService = nil
	}
	auditService, err := audit.NewServiceFromEnv()
	if err != nil {
		auditService = nil
	}
	aiService := ai.NewService(ai.NewProviderFromEnv())
	
	// Metrics Setup
	var metricService *metrics.Service
	if dockerClient != nil {
		rawDocker, _ := dockerengine.NewRawClient()
		if rawDocker != nil {
			ms, err := metrics.NewServiceFromEnv(rawDocker)
			if err == nil {
				metricService = ms
			}
		}
	}

	// Scheduler Setup
	schedStore, _ := scheduler.OpenStore(dbPath)
	if schedStore != nil {
		_ = schedStore.Init(context.Background())
	}
	schedSvc := scheduler.NewService(schedStore)

	// Register available tasks
	if dockerClient != nil {
		rawDocker, _ := dockerengine.NewRawClient() 
		if rawDocker != nil {
			// 1. Docker Prune (Weekly Sun 3 AM)
			schedSvc.RegisterTask("docker_system_prune", func() scheduler.Task {
				return scheduler.NewDockerPruneTask(rawDocker)
			})
			_ = schedSvc.AddTask("0 0 3 * * 0", scheduler.NewDockerPruneTask(rawDocker))

			// 2. Metrics Collector (Every minute) & Pruner (Daily)
			if metricService != nil {
				schedSvc.RegisterTask("metrics_collector", func() scheduler.Task {
					return metricService.GetCollectorTask()
				})
				// Enable by default for "Light Touch" observability
				_ = schedSvc.AddTask("* * * * *", metricService.GetCollectorTask())

				schedSvc.RegisterTask("metrics_prune", func() scheduler.Task {
					return metricService.GetPruneTask()
				})
				_ = schedSvc.AddTask("0 0 0 * * *", metricService.GetPruneTask())
			}

			// 3. Trivy Sweep (Daily Midnight)
			if scanService != nil {
				schedSvc.RegisterTask("security_sweep_trivy", func() scheduler.Task {
					return scheduler.NewTrivySweepTask(rawDocker, scanService)
				})
				_ = schedSvc.AddTask("0 0 0 * * *", scheduler.NewTrivySweepTask(rawDocker, scanService))

				// 3. ClamAV Sweep (Weekly Sun 4 AM)
				schedSvc.RegisterTask("malware_sweep_clamav", func() scheduler.Task {
					return scheduler.NewClamAVSweepTask(rawDocker, scanService)
				})
				_ = schedSvc.AddTask("0 0 4 * * 0", scheduler.NewClamAVSweepTask(rawDocker, scanService))
			}
		}
	}

	// Load all enabled schedules from DB
	_ = schedSvc.LoadSchedules(context.Background())

	return NewMuxWithDeps(dockerClient, scanService, releaseService, updateService, auditService, aiService, schedSvc, metricService, diagService), schedSvc
}

func NewMuxWithDeps(dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService, schedSvc SchedulerService, metricService MetricsService, diagService DiagService) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, gen.HealthResponse{Status: "ok", Service: "harborwatch", Version: appVersion()})
	})

	mux.HandleFunc("GET /api/docker/containers", func(w http.ResponseWriter, r *http.Request) {
		if dockerClient == nil {
			writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		containers, err := dockerClient.ListContainers(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, "docker_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, containers)
	})

	mux.HandleFunc("GET /api/docker/images", func(w http.ResponseWriter, r *http.Request) {
		if dockerClient == nil {
			writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		images, err := dockerClient.ListImages(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, "docker_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, images)
	})

	mux.HandleFunc("GET /api/docker/events", func(w http.ResponseWriter, r *http.Request) {
		if dockerClient == nil {
			writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
			return
		}
		stream, err := dockerClient.OpenEventStream(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "docker_error", err.Error())
			return
		}
		defer stream.Close()
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported by response writer")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
			default:
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil && r.Context().Err() == nil {
						fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
						flusher.Flush()
					}
					return
				}
				event := convertEvent(scanner.Bytes())
				payload, err := json.Marshal(event)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: docker\ndata: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("POST /api/scans/run", func(w http.ResponseWriter, r *http.Request) {
		if scanService == nil {
			writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
			return
		}
		var req gen.ScanRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
			return
		}
		resp, err := scanService.StartScan(req.Target)
		if err != nil {
			writeError(w, http.StatusBadRequest, "scan_start_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, resp)
	})

	mux.HandleFunc("POST /api/scans/malware/run", func(w http.ResponseWriter, r *http.Request) {
		if scanService == nil {
			writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
			return
		}
		var req gen.MalwareScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
			return
		}
		resp, err := scanService.StartMalwareScan(req.Target)
		if err != nil {
			writeError(w, http.StatusBadRequest, "malware_scan_start_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, resp)
	})

	mux.HandleFunc("GET /api/scans/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if scanService == nil {
			writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
			return
		}
		id := r.PathValue("id")
		job, err := scanService.Job(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "job_not_found", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, job)
	})

	mux.HandleFunc("GET /api/scans/summary", func(w http.ResponseWriter, r *http.Request) {
		if scanService == nil {
			writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		summary, err := scanService.LatestSummary(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, "scan_read_failed", err.Error())
			return
		}
		if summary == nil {
			writeError(w, http.StatusNotFound, "scan_not_found", "No scan results available")
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("GET /api/scans/malware/summary", func(w http.ResponseWriter, r *http.Request) {
		if scanService == nil {
			writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
			return
		}
		target := r.URL.Query().Get("target")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		summaries, err := scanService.MalwareSummaries(ctx, target)
		if err != nil {
			writeError(w, http.StatusBadGateway, "malware_scan_read_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summaries)
	})

	mux.HandleFunc("GET /api/releases/summary", func(w http.ResponseWriter, r *http.Request) {
		if releaseService == nil {
			writeError(w, http.StatusServiceUnavailable, "release_service_unavailable", "Release service unavailable")
			return
		}
		repo := r.URL.Query().Get("repo")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		summary, err := releaseService.Analyze(ctx, repo)
		if err != nil {
			writeError(w, http.StatusBadGateway, "release_analysis_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("POST /api/updates/run", func(w http.ResponseWriter, r *http.Request) {
		if updateService == nil {
			writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
			return
		}
		var req gen.UpdateStartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
			return
		}
		resp, err := updateService.StartUpdate(updates.Request{
			ContainerID: req.ContainerID,
			TargetImage: req.TargetImage,
			ValidateURL: req.ValidateURL,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "update_start_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, resp)
	})

	mux.HandleFunc("GET /api/audit/jobs", func(w http.ResponseWriter, r *http.Request) {
		if auditService == nil {
			writeError(w, http.StatusServiceUnavailable, "audit_service_unavailable", "Audit service not initialized")
			return
		}
		jobs, err := auditService.ListAuditJobs(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "audit_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, jobs)
	})

	mux.HandleFunc("GET /api/audit/jobs/{id}/steps", func(w http.ResponseWriter, r *http.Request) {
		if auditService == nil {
			writeError(w, http.StatusServiceUnavailable, "audit_service_unavailable", "Audit service not initialized")
			return
		}
		steps, err := auditService.GetAuditJobSteps(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "audit_steps_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, steps)
	})

	mux.HandleFunc("GET /api/ai/status", func(w http.ResponseWriter, r *http.Request) {
		enabled := false
		if aiService != nil {
			enabled = aiService.HasProvider()
		}
		writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
	})

	mux.HandleFunc("POST /api/ai/audit-compose", func(w http.ResponseWriter, r *http.Request) {
		if aiService == nil || !aiService.HasProvider() {
			writeError(w, http.StatusServiceUnavailable, "ai_unavailable", "AI provider not configured")
			return
		}
		var req struct {
			YAML string `json:"yaml"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
			return
		}
		analysis, err := aiService.AuditCompose(r.Context(), req.YAML)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ai_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"analysis": analysis})
	})

	mux.HandleFunc("POST /api/ai/analyze-metrics", func(w http.ResponseWriter, r *http.Request) {
		if aiService == nil || !aiService.HasProvider() {
			writeError(w, http.StatusServiceUnavailable, "ai_unavailable", "AI provider not configured")
			return
		}
		var req struct {
			ContainerID string `json:"containerId"`
			Metrics     []any  `json:"metrics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
			return
		}
		analysis, err := aiService.AnalyzeMetrics(r.Context(), req.ContainerID, req.Metrics)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ai_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"analysis": analysis})
	})

	mux.HandleFunc("GET /api/scheduler/status", func(w http.ResponseWriter, r *http.Request) {
		enabled := schedSvc != nil
		writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
	})

	mux.HandleFunc("GET /api/metrics/{id}", func(w http.ResponseWriter, r *http.Request) {
		if metricService == nil {
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable", "Metrics service not initialized")
			return
		}
		duration := r.URL.Query().Get("duration")
		if duration == "" {
			duration = "24h"
		}
		data, err := metricService.GetMetrics(r.Context(), r.PathValue("id"), duration)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "metrics_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("GET /api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if diagService == nil {
			writeError(w, http.StatusServiceUnavailable, "diag_unavailable", "Diagnostic service not initialized")
			return
		}
		writeJSON(w, http.StatusOK, diagService.GetSystemStatus())
	})

	mux.HandleFunc("GET /api/system/logs", func(w http.ResponseWriter, r *http.Request) {
		if diagService == nil {
			writeError(w, http.StatusServiceUnavailable, "diag_unavailable", "Diagnostic service not initialized")
			return
		}
		logs, err := diagService.ListLogs(r.Context(), 100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "diag_log_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, logs)
	})

	mux.HandleFunc("GET /api/scheduler/schedules", func(w http.ResponseWriter, r *http.Request) {
		if schedSvc == nil {
			writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
			return
		}
		list, err := schedSvc.ListSchedules(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scheduler_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/scheduler/toggle", func(w http.ResponseWriter, r *http.Request) {
		if schedSvc == nil {
			writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
			return
		}
		var req struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
			return
		}
		if err := schedSvc.ToggleTask(r.Context(), req.ID, req.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, "toggle_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/scheduler/run", func(w http.ResponseWriter, r *http.Request) {
		if schedSvc == nil {
			writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
			return
		}
		if err := schedSvc.RunTask(r.Context(), req.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "trigger_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
	})

	mux.HandleFunc("GET /api/updates/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if updateService == nil {
			writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		run, err := updateService.GetJob(ctx, r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadGateway, "update_read_failed", err.Error())
			return
		}
		if run == nil {
			writeError(w, http.StatusNotFound, "update_not_found", "Update job not found")
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	mux.HandleFunc("GET /api/updates/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		if updateService == nil {
			writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported by response writer")
			return
		}
		ch, cancel := updateService.Subscribe(r.PathValue("id"))
		defer cancel()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				payload, _ := json.Marshal(e)
				fmt.Fprintf(w, "event: update\ndata: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})

	staticDir := filepath.Clean(filepath.Join("..", "web", "dist"))
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", spaHandler(fs, staticDir))
	return mux
}

func appVersion() string {
	if v := os.Getenv("HARBORWATCH_VERSION"); v != "" {
		return v
	}
	return "dev"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func convertEvent(raw []byte) gen.DockerEvent {
	var event struct {
		Type   string `json:"Type"`
		Action string `json:"Action"`
		Status string `json:"status"`
		ID     string `json:"id"`
		From   string `json:"from"`
		Time   int64  `json:"time"`
		Actor  struct {
			ID         string            `json:"ID"`
			Attributes map[string]string `json:"Attributes"`
		} `json:"Actor"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return gen.DockerEvent{Type: "unknown", Action: "unparseable"}
	}
	id := event.ID
	if id == "" {
		id = event.Actor.ID
	}
	action := event.Action
	if action == "" {
		action = event.Status
	}
	return gen.DockerEvent{Type: event.Type, Action: action, ID: id, From: event.From, Attributes: event.Actor.Attributes, Time: event.Time}
}

func spaHandler(static http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(filepath.Join(staticDir, r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			static.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
