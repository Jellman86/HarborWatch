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
}

type AuditService interface {
	ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error)
}

type UpdateService interface {
	StartUpdate(req updates.Request) (gen.UpdateStartResponse, error)
	GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error)
	Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func())
}

func NewMux() http.Handler {
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
	return NewMuxWithDeps(dockerClient, scanService, releaseService, updateService, auditService, aiService)
}

func NewMuxWithDeps(dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService) http.Handler {
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
