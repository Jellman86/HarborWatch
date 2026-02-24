package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/go-chi/chi/v5"
)

func registerScanRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/scans", func(r chi.Router) {
		r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			var req gen.ScanRunRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
				return
			}
			resp, err := deps.scanService.StartScan(req.Target)
			if err != nil {
				if errors.Is(err, scanning.ErrDuplicateActiveScan) {
					writeError(w, http.StatusConflict, "scan_already_running", err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, "scan_start_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, resp)
		})

		r.Post("/malware/run", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			var req gen.MalwareScanRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
				return
			}
			resp, err := deps.scanService.StartMalwareScan(req.Target)
			if err != nil {
				if errors.Is(err, scanning.ErrDuplicateActiveScan) {
					writeError(w, http.StatusConflict, "malware_scan_already_running", err.Error())
					return
				}
				writeError(w, http.StatusBadRequest, "malware_scan_start_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, resp)
		})

		r.Get("/malware/signatures/status", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			status, err := deps.scanService.ClamAVSignatureStatus(ctx)
			if err != nil {
				writeError(w, http.StatusBadGateway, "clamav_status_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, status)
		})

		r.Post("/malware/signatures/update", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
			defer cancel()
			summary, err := deps.scanService.UpdateClamAVSignatures(ctx)
			if err != nil {
				writeError(w, http.StatusBadGateway, "clamav_update_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "summary": summary})
		})

		r.Post("/malware/container/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			if deps.dockerClient == nil {
				writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
				return
			}
			containerID := chi.URLParam(r, "id")
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			container, err := deps.dockerClient.GetContainer(ctx, containerID)
			cancel()
			if err != nil {
				writeError(w, http.StatusNotFound, "container_not_found", err.Error())
				return
			}
			canonicalID := strings.TrimSpace(container.ID)
			if canonicalID == "" {
				canonicalID = strings.TrimSpace(containerID)
			}
			if canonicalID == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "container id is required")
				return
			}
			go triggerContainerMalwareScans(canonicalID, deps.scanService, deps.settingsService, deps.diagService)
			writeJSON(w, http.StatusAccepted, map[string]string{
				"status":      "queued",
				"containerId": canonicalID,
			})
		})

		r.Get("/malware/container/{id}/summary", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			containerID := strings.TrimSpace(chi.URLParam(r, "id"))
			if containerID == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "container id is required")
				return
			}
			if deps.dockerClient != nil {
				resolveCtx, resolveCancel := context.WithTimeout(r.Context(), 3*time.Second)
				if container, err := deps.dockerClient.GetContainer(resolveCtx, containerID); err == nil {
					if canonicalID := strings.TrimSpace(container.ID); canonicalID != "" {
						containerID = canonicalID
					}
				}
				resolveCancel()
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			summary := gen.ContainerSummary{ID: containerID}
			if deps.loadContainerSummary != nil {
				summary = deps.loadContainerSummary(ctx, containerID)
			}
			name := "unknown"
			if len(summary.Names) > 0 {
				name = strings.TrimPrefix(summary.Names[0], "/")
			}
			summaries, err := deps.scanService.MalwareSummariesForContainer(ctx, containerID, name)
			if err != nil {
				writeError(w, http.StatusBadGateway, "malware_scan_read_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, summaries)
		})

		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			limit := parseIntQuery(r.URL.Query().Get("limit"), 50, 1, 500)
			scanType := strings.TrimSpace(r.URL.Query().Get("type"))
			prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
			jobs, err := deps.scanService.ListJobs(r.Context(), scanType, prefix, limit)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "scan_jobs_list_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, jobs)
		})

		r.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			id := chi.URLParam(r, "id")
			job, err := deps.scanService.Job(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "job_not_found", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, job)
		})

		r.Post("/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			id := strings.TrimSpace(chi.URLParam(r, "id"))
			if id == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "scan job id is required")
				return
			}
			job, err := deps.scanService.CancelJob(r.Context(), id)
			if err != nil {
				msg := strings.ToLower(strings.TrimSpace(err.Error()))
				if strings.Contains(msg, "not found") {
					writeError(w, http.StatusNotFound, "job_not_found", err.Error())
					return
				}
				writeError(w, http.StatusConflict, "scan_cancel_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, job)
		})

		r.Get("/summary", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			summary, err := deps.scanService.LatestSummary(ctx)
			if err != nil {
				writeError(w, http.StatusBadGateway, "scan_read_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, summary)
		})

		r.Get("/details", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			target := strings.TrimSpace(r.URL.Query().Get("target"))
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			details, err := deps.scanService.LatestDetailsForTarget(ctx, target)
			if err != nil {
				writeError(w, http.StatusBadGateway, "scan_read_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, details)
		})

		r.Get("/malware/summary", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			target := r.URL.Query().Get("target")
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			summaries, err := deps.scanService.MalwareSummaries(ctx, target)
			if err != nil {
				writeError(w, http.StatusBadGateway, "malware_scan_read_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, summaries)
		})

		r.Get("/malware/details", func(w http.ResponseWriter, r *http.Request) {
			if deps.scanService == nil {
				writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
				return
			}
			target := strings.TrimSpace(r.URL.Query().Get("target"))
			prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
			limit := 25
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if parsed, err := strconv.Atoi(raw); err == nil {
					limit = parsed
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			details, err := deps.scanService.MalwareDetails(ctx, target, prefix, limit)
			if err != nil {
				writeError(w, http.StatusBadGateway, "malware_scan_read_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, details)
		})
	})
}
