package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/go-chi/chi/v5"
)

type adminRouteDeps struct {
	db                    *sql.DB
	dockerClient          DockerClient
	scanService           ScanService
	updateService         UpdateService
	auditService          AuditService
	schedSvc              SchedulerService
	metricService         MetricsService
	diagService           DiagService
	settingsService       SettingsService
	notificationService   NotificationService
	aiService             AIService
	jobManager            *jobs.Manager
	currentPortainerState *PortainerClient
	loadContainerSummary  func(context.Context, string) gen.ContainerSummary
}

func registerSystemRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/system", func(r chi.Router) {
		r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
			if deps.diagService == nil {
				writeError(w, http.StatusServiceUnavailable, "diag_unavailable", "Diagnostic service not initialized")
				return
			}
			status := deps.diagService.GetSystemStatus()
			if deps.jobManager != nil {
				status.ActiveJobs = deps.jobManager.ActiveJobs()
			} else {
				status.ActiveJobs = []gen.JobProgress{}
			}
			writeJSON(w, http.StatusOK, status)
		})

		r.Post("/jobs/cancel-all", func(w http.ResponseWriter, r *http.Request) {
			if deps.jobManager == nil {
				writeError(w, http.StatusServiceUnavailable, "job_manager_unavailable", "Job manager not initialized")
				return
			}
			deps.jobManager.CancelAll()
			if deps.diagService != nil {
				deps.diagService.Log("WARN", "System", "All background jobs cancelled by user")
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "All jobs cancelled"})
		})

		r.Post("/clear-history", func(w http.ResponseWriter, r *http.Request) {
			if deps.db == nil {
				writeError(w, http.StatusServiceUnavailable, "db_unavailable", "Database not available")
				return
			}

			tables := []string{
				"update_runs",
				"update_steps",
				"scan_jobs",
				"scan_results",
				"malware_scan_results",
				"compose_audit_history",
				"ai_usage_events",
				"remediation_runs",
				"container_metrics",
				"internal_logs",
			}

			tx, err := deps.db.BeginTx(r.Context(), nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			defer tx.Rollback()

			for _, table := range tables {
				if _, err := tx.ExecContext(r.Context(), fmt.Sprintf("DELETE FROM %s", table)); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", fmt.Sprintf("failed to clear %s: %v", table, err))
					return
				}
			}

			if err := tx.Commit(); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}

			if deps.diagService != nil {
				deps.diagService.Log("WARN", "System", "All historical data cleared by user")
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "All historical data cleared"})
		})

		r.Get("/logs", func(w http.ResponseWriter, r *http.Request) {
			if deps.diagService == nil {
				writeError(w, http.StatusServiceUnavailable, "diag_unavailable", "Diagnostic service not initialized")
				return
			}
			limit := parseIntQuery(r.URL.Query().Get("limit"), 100, 1, 2000)
			offset := parseIntQuery(r.URL.Query().Get("offset"), 0, 0, 1000000)
			level := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("level")))
			source := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("source")))
			search := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("search")))
			if search == "" {
				search = strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
			}
			sinceRaw := strings.TrimSpace(r.URL.Query().Get("since"))
			var since int64
			if sinceRaw != "" {
				parsed, err := strconv.ParseInt(sinceRaw, 10, 64)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", "since must be a unix timestamp in seconds")
					return
				}
				since = parsed
			}

			logs, total, err := deps.diagService.ListLogs(r.Context(), limit, offset, level, source, search, since)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "diag_log_failed", err.Error())
				return
			}

			if logs == nil {
				logs = []diag.LogEntry{}
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"logs":  logs,
				"total": total,
			})
		})
	})
}

func registerDiagnosticsRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/diagnostics", func(r chi.Router) {
		r.Get("/containers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
			if deps.dockerClient == nil {
				writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
				return
			}
			id := chi.URLParam(r, "id")
			tail := parseIntQuery(r.URL.Query().Get("tail"), 300, 1, 5000)
			timestamps := strings.EqualFold(r.URL.Query().Get("timestamps"), "true") || r.URL.Query().Get("timestamps") == "1"
			since, err := parseSinceQuery(r.URL.Query().Get("since"), 6*time.Hour)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "since must be a duration (e.g. 15m, 1h) or unix timestamp")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			logs, err := deps.dockerClient.GetContainerLogs(ctx, id, tail, since, timestamps)
			if err != nil {
				if deps.diagService != nil {
					deps.diagService.Log("ERROR", "DockerLogs", fmt.Sprintf("Failed to load logs for %s: %v", id, err))
				}
				status := http.StatusBadGateway
				if isContainerNotFoundError(err) {
					status = http.StatusNotFound
				}
				writeError(w, status, "container_logs_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, logs)
		})

		r.Get("/snapshot", func(w http.ResponseWriter, r *http.Request) {
			snapshot, err := collectDiagnosticsSnapshot(
				r.Context(),
				diagnosticsDeps{
					db:            deps.db,
					dockerClient:  deps.dockerClient,
					scanService:   deps.scanService,
					updateService: deps.updateService,
					auditService:  deps.auditService,
					schedSvc:      deps.schedSvc,
					diagService:   deps.diagService,
				},
				diagnosticsSnapshotOptions{
					LogLimit:         parseIntQuery(r.URL.Query().Get("logLimit"), 300, 10, 2000),
					AuditLimit:       parseIntQuery(r.URL.Query().Get("auditLimit"), 100, 10, 500),
					IncludeFleet:     parseBoolQuery(r.URL.Query().Get("includeFleet"), true),
					ContainerID:      strings.TrimSpace(r.URL.Query().Get("containerId")),
					ContainerLogTail: parseIntQuery(r.URL.Query().Get("containerLogTail"), 250, 1, 5000),
					ContainerSince:   r.URL.Query().Get("containerLogSince"),
				},
			)
			if err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "invalid containerLogSince") {
					status = http.StatusBadRequest
				}
				writeError(w, status, "diagnostics_snapshot_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, snapshot)
		})
	})
}

func registerSettingsRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/settings", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if deps.settingsService == nil {
				writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
				return
			}
			st, err := deps.settingsService.Get(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "settings_get_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, st)
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if deps.settingsService == nil {
				writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
				return
			}
			var st settings.Settings
			if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
				return
			}
			if err := deps.settingsService.Save(r.Context(), st); err != nil {
				writeError(w, http.StatusInternalServerError, "settings_save_failed", err.Error())
				return
			}

			fresh := st
			if loaded, err := deps.settingsService.Get(r.Context()); err == nil {
				fresh = loaded
			}
			if deps.jobManager != nil {
				deps.jobManager.SetMaxConcurrency(fresh.AutoUpgradeMaxConcurrency)
			}

			if deps.notificationService != nil {
				if !fresh.DiscordEnabled || strings.TrimSpace(fresh.DiscordWebhookURL) == "" {
					deps.notificationService.RemoveDispatcher("discord")
				} else {
					deps.notificationService.AddDispatcher(notifications.NewDiscordDispatcher(fresh.DiscordWebhookURL))
				}
			}

			if fresh.PortainerEnabled && strings.TrimSpace(fresh.PortainerURL) != "" {
				client := portainer.NewClient(fresh.PortainerURL, fresh.PortainerApiKey)
				if deps.currentPortainerState != nil {
					*deps.currentPortainerState = client
				}
				testCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				if _, err := client.ListStacks(testCtx); err == nil {
					fresh.PortainerTestingPassed = true
				} else {
					fresh.PortainerTestingPassed = false
				}
			} else {
				if deps.currentPortainerState != nil {
					*deps.currentPortainerState = nil
				}
				fresh.PortainerTestingPassed = false
			}

			if !fresh.AIEnabled || (strings.TrimSpace(fresh.OpenAIKey) == "" && strings.TrimSpace(fresh.AnthropicKey) == "" && strings.TrimSpace(fresh.GeminiKey) == "") {
				fresh.AITestingPassed = false
			}

			_ = deps.settingsService.Save(r.Context(), fresh)

			if setter, ok := deps.aiService.(interface{ SetProvider(ai.Provider) }); ok {
				if fresh.AIEnabled {
					setter.SetProvider(ai.NewProviderFromConfig(ai.ProviderConfig{
						Preferred:      fresh.AIProvider,
						OpenAIKey:      fresh.OpenAIKey,
						OpenAIModel:    fresh.OpenAIModel,
						AnthropicKey:   fresh.AnthropicKey,
						AnthropicModel: fresh.AnthropicModel,
						GeminiKey:      fresh.GeminiKey,
						GeminiModel:    fresh.GeminiModel,
					}))
				} else {
					setter.SetProvider(nil)
				}
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})
}

func registerSchedulerRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/scheduler", func(r chi.Router) {
		r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
			enabled := deps.schedSvc != nil
			writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
		})

		r.Get("/schedules", func(w http.ResponseWriter, r *http.Request) {
			if deps.schedSvc == nil {
				writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
				return
			}
			list, err := deps.schedSvc.ListSchedules(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "scheduler_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, list)
		})

		r.Post("/toggle", func(w http.ResponseWriter, r *http.Request) {
			if deps.schedSvc == nil {
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
			if err := deps.schedSvc.ToggleTask(r.Context(), req.ID, req.Enabled); err != nil {
				writeError(w, http.StatusInternalServerError, "toggle_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		r.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			if deps.schedSvc == nil {
				writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
				return
			}
			var req struct {
				ID       string `json:"id"`
				CronSpec string `json:"cronSpec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
				return
			}
			req.ID = strings.TrimSpace(req.ID)
			req.CronSpec = strings.TrimSpace(req.CronSpec)
			if req.ID == "" || req.CronSpec == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "id and cronSpec are required")
				return
			}
			if err := deps.schedSvc.UpdateTaskSchedule(r.Context(), req.ID, req.CronSpec); err != nil {
				writeError(w, http.StatusBadRequest, "update_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
			if deps.schedSvc == nil {
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
			if err := deps.schedSvc.RunTask(r.Context(), req.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "trigger_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered"})
		})
	})
}

func registerMetricsRoutes(r chi.Router, deps adminRouteDeps) {
	r.Post("/metrics/batch", func(w http.ResponseWriter, r *http.Request) {
		if deps.metricService == nil {
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable", "Metrics service not initialized")
			return
		}
		var req struct {
			IDs      []string `json:"ids"`
			Duration string   `json:"duration"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
			return
		}
		if len(req.IDs) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "ids must not be empty")
			return
		}
		if len(req.IDs) > 200 {
			writeError(w, http.StatusBadRequest, "invalid_request", "ids length exceeds maximum (200)")
			return
		}
		duration := req.Duration
		if duration == "" {
			duration = "1h"
		}
		out := make(map[string][]metrics.Metric, len(req.IDs))
		for _, id := range req.IDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			data, err := deps.metricService.GetMetrics(r.Context(), id, duration)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "metrics_query_failed", err.Error())
				return
			}
			out[id] = data
		}
		writeJSON(w, http.StatusOK, out)
	})

	r.Get("/metrics/{id}", func(w http.ResponseWriter, r *http.Request) {
		if deps.metricService == nil {
			writeError(w, http.StatusServiceUnavailable, "metrics_unavailable", "Metrics service not initialized")
			return
		}
		duration := r.URL.Query().Get("duration")
		if duration == "" {
			duration = "24h"
		}
		data, err := deps.metricService.GetMetrics(r.Context(), chi.URLParam(r, "id"), duration)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "metrics_query_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, data)
	})
}
