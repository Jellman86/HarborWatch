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
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/releases"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type DockerClient interface {
	ListContainers(ctx context.Context) ([]gen.ContainerSummary, error)
	GetContainer(ctx context.Context, id string) (gen.ContainerSummary, error)
	GetContainerComposeConfig(ctx context.Context, id string, portainer *portainer.Client) (string, error)
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
	OpenEventStream(ctx context.Context) (io.ReadCloser, error)
}

type ScanService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
	Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error)
	LatestSummary(ctx context.Context) (*gen.ScanSummary, error)
	LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error)
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
	PruneLogs(ctx context.Context, olderThan int64) (int64, error)
}

type NotificationService interface {
	Dispatch(ctx context.Context, msg notifications.Message)
	AddDispatcher(d notifications.Dispatcher)
}

type SettingsService interface {
	Get(ctx context.Context) (settings.Settings, error)
	Save(ctx context.Context, s settings.Settings) error
}

type RulesService interface {
	Get(ctx context.Context, id string) (rules.ContainerRules, error)
	Save(ctx context.Context, r rules.ContainerRules) error
}

type AuditService interface {
	ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error)
	ListAuditJobsForContainer(ctx context.Context, containerID string) ([]gen.AuditJobSummary, error)
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
	GetCollectorTask() *metrics.Collector
	GetPruneTask() *metrics.PruneTask
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

	// 1. Diagnostics Setup
	diagService, _ := diag.NewService(dbPath)
	if diagService != nil {
		diagService.Log("INFO", "System", "HarborWatch initializing...")
	}

	dockerClient, err := dockerengine.NewFromEnv()
	if err != nil && diagService != nil {
		diagService.Log("ERROR", "Docker", fmt.Sprintf("Failed to init docker client: %v", err))
	}
	
	// 2. Open Stores
	updatesStore, _ := updates.OpenStore(dbPath)
	if updatesStore != nil {
		_ = updatesStore.Init(context.Background())
	}
	
	settingsStore, _ := settings.OpenStore(dbPath)
	if settingsStore != nil {
		_ = settingsStore.Init(context.Background())
	}

	rulesStore, _ := rules.OpenStore(dbPath)
	if rulesStore != nil {
		_ = rulesStore.Init(context.Background())
	}

	// 3. Initialize Domain Services
	scanService, _ := scanning.NewServiceFromEnv()
	releaseService := releases.NewService()
	aiService := ai.NewService(ai.NewProviderFromEnv())
	notificationService := notifications.NewService()
	
	var portainerService *portainer.Client
	if settingsStore != nil {
		st, _ := settingsStore.Get(context.Background())
		if st.DiscordWebhookURL != "" {
			notificationService.AddDispatcher(notifications.NewDiscordDispatcher(st.DiscordWebhookURL))
		}
		if st.PortainerURL != "" {
			portainerService = portainer.NewClient(st.PortainerURL, st.PortainerApiKey)
		}
	}

	updateService := updates.NewService(updatesStore, updates.NewCommandExecutor(), aiService, notificationService, diagService)
	
	var auditService *audit.Service
	if settingsStore != nil {
		auditService = audit.NewService(settingsStore.GetDB())
	}

	// 4. Metrics Setup
	var metricService *metrics.Service
	if dockerClient != nil {
		rawDocker, _ := dockerengine.NewRawClient()
		if rawDocker != nil {
			metricService, _ = metrics.NewServiceFromEnv(rawDocker)
		}
	}

	// 5. Scheduler Setup
	schedStore, _ := scheduler.OpenStore(dbPath)
	if schedStore != nil {
		_ = schedStore.Init(context.Background())
	}
	schedSvc := scheduler.NewService(schedStore)

	// 6. Register automated tasks
	if dockerClient != nil {
		rawDocker, _ := dockerengine.NewRawClient() 
		if rawDocker != nil {
			// Maintenance: Weekly Prune
			schedSvc.RegisterTask("docker_system_prune", func() scheduler.Task {
				return scheduler.NewDockerPruneTask(rawDocker)
			})
			_ = schedSvc.AddTask("0 0 3 * * 0", scheduler.NewDockerPruneTask(rawDocker))

			schedSvc.RegisterTask("container_update_check", func() scheduler.Task {
				return dockerengine.NewUpdateCheckTask(rawDocker)
			})
			_ = schedSvc.AddTask("0 * * * *", dockerengine.NewUpdateCheckTask(rawDocker))

			// Trigger immediate update check on boot
			go func() {
				time.Sleep(5 * time.Second)
				_ = schedSvc.RunTask(context.Background(), "container_update_check")
			}()

			// Metrics Engine
			if metricService != nil {
				schedSvc.RegisterTask("metrics_collector", func() scheduler.Task {
					return metricService.GetCollectorTask()
				})
				_ = schedSvc.AddTask("* * * * *", metricService.GetCollectorTask())

				schedSvc.RegisterTask("metrics_prune", func() scheduler.Task {
					return metricService.GetPruneTask()
				})
				_ = schedSvc.AddTask("0 0 0 * * *", metricService.GetPruneTask())
			}

			// Security: Scheduled Sweeps
			if scanService != nil {
				schedSvc.RegisterTask("security_sweep_trivy", func() scheduler.Task {
					return scheduler.NewTrivySweepTask(rawDocker, scanService)
				})
				_ = schedSvc.AddTask("0 0 0 * * *", scheduler.NewTrivySweepTask(rawDocker, scanService))

				schedSvc.RegisterTask("malware_sweep_clamav", func() scheduler.Task {
					return scheduler.NewClamAVSweepTask(rawDocker, scanService)
				})
				_ = schedSvc.AddTask("0 0 4 * * 0", scheduler.NewClamAVSweepTask(rawDocker, scanService))
			}
		}
	}

	// 7. Internal Maintenance: Diagnostic log pruning
	if diagService != nil {
		schedSvc.RegisterTask("diag_log_prune", func() scheduler.Task {
			return scheduler.NewGenericTask("diag_log_prune", func(ctx context.Context) error {
				olderThan := time.Now().Add(-7 * 24 * time.Hour).Unix()
				_, err := diagService.PruneLogs(ctx, olderThan)
				return err
			})
		})
		_ = schedSvc.AddTask("0 0 1 * * *", scheduler.NewGenericTask("diag_log_prune", func(ctx context.Context) error {
			olderThan := time.Now().Add(-7 * 24 * time.Hour).Unix()
			_, err := diagService.PruneLogs(ctx, olderThan)
			return err
		}))
	}

	// Bootstrap schedules from DB
	_ = schedSvc.LoadSchedules(context.Background())

	return NewMuxWithDeps(dockerClient, scanService, releaseService, updateService, auditService, aiService, schedSvc, metricService, diagService, notificationService, settingsStore, portainerService, rulesStore), schedSvc
}

func NewMuxWithDeps(dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService, schedSvc SchedulerService, metricService MetricsService, diagService DiagService, notificationService NotificationService, settingsService SettingsService, portainerService *portainer.Client, rulesService RulesService) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, gen.HealthResponse{Status: "ok", Service: "harborwatch", Version: appVersion()})
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/docker", func(r chi.Router) {
			r.Get("/containers", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				id := chi.URLParam(r, "id")
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				defer cancel()

				summary, err := dockerClient.GetContainer(ctx, id)
				if err != nil {
					writeError(w, http.StatusNotFound, "container_not_found", err.Error())
					return
				}

				detail := gen.ContainerDetail{
					Summary: summary,
				}

				// Enrich with Security Data
				if scanService != nil {
					if vs, err := scanService.LatestSummaryForTarget(ctx, summary.Image); err == nil {
						detail.VulnerabilitySummary = vs
					}
					if ms, err := scanService.MalwareSummaries(ctx, id); err == nil {
						detail.MalwareSummary = ms
					}
				}

				// Enrich with Metrics
				if metricService != nil {
					if m, err := metricService.GetMetrics(ctx, id, "1h"); err == nil {
						// Convert internal metrics to gen.Metric
						detail.RecentMetrics = make([]gen.Metric, 0, len(m))
						for _, item := range m {
							detail.RecentMetrics = append(detail.RecentMetrics, gen.Metric{
								ContainerID: item.ContainerID,
								Timestamp:   item.Timestamp,
								CPUPercent:  item.CPUPercent,
								MemoryUsage: item.MemoryUsage,
								MemoryLimit: item.MemoryLimit,
								Pids:        item.Pids,
							})
						}
					}
				}

				// Enrich with Audit History
				if auditService != nil {
					if ah, err := auditService.ListAuditJobsForContainer(ctx, id); err == nil {
						detail.ActionHistory = ah
					}
				}

				// Enrich with Rules
				if rulesService != nil {
					if r, err := rulesService.Get(ctx, id); err == nil {
						detail.Rules = &gen.ContainerRules{
							ContainerID:  r.ContainerID,
							UpdatePolicy: r.UpdatePolicy,
							ValidateURL:  r.ValidateURL,
							AutoRollback: r.AutoRollback,
						}
					}
				}

				// Fetch Compose Config if requested or for enrichment
				// For now we don't return full YAML in the Detail object to keep it light,
				// but the backend is ready for the Doctor.

				writeJSON(w, http.StatusOK, detail)
			})

			r.Route("/{id}/rules", func(r chi.Router) {
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					if rulesService == nil {
						writeError(w, http.StatusServiceUnavailable, "rules_unavailable", "Rules service not initialized")
						return
					}
					id := chi.URLParam(r, "id")
					res, err := rulesService.Get(r.Context(), id)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "rules_get_failed", err.Error())
						return
					}
					writeJSON(w, http.StatusOK, res)
				})

				r.Post("/", func(w http.ResponseWriter, r *http.Request) {
					if rulesService == nil {
						writeError(w, http.StatusServiceUnavailable, "rules_unavailable", "Rules service not initialized")
						return
					}
					var req rules.ContainerRules
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
						return
					}
					req.ContainerID = chi.URLParam(r, "id")
					if err := rulesService.Save(r.Context(), req); err != nil {
						writeError(w, http.StatusInternalServerError, "rules_save_failed", err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
				})
			})

			r.Get("/images", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
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
		})

		r.Route("/scans", func(r chi.Router) {
			r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
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

			r.Post("/malware/run", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
				if scanService == nil {
					writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
					return
				}
				id := chi.URLParam(r, "id")
				job, err := scanService.Job(r.Context(), id)
				if err != nil {
					writeError(w, http.StatusNotFound, "job_not_found", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, job)
			})

			r.Get("/summary", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/malware/summary", func(w http.ResponseWriter, r *http.Request) {
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
		})

		r.Get("/releases/summary", func(w http.ResponseWriter, r *http.Request) {
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

		r.Route("/updates", func(r chi.Router) {
			r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
				if updateService == nil {
					writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer cancel()
				run, err := updateService.GetJob(ctx, chi.URLParam(r, "id"))
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

			r.Get("/events/{id}", func(w http.ResponseWriter, r *http.Request) {
				if updateService == nil {
					writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
					return
				}
				flusher, ok := w.(http.Flusher)
				if !ok {
					writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported by response writer")
					return
				}
				ch, cancel := updateService.Subscribe(chi.URLParam(r, "id"))
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
		})

		r.Route("/audit", func(r chi.Router) {
			r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/jobs/{id}/steps", func(w http.ResponseWriter, r *http.Request) {
				if auditService == nil {
					writeError(w, http.StatusServiceUnavailable, "audit_service_unavailable", "Audit service not initialized")
					return
				}
				steps, err := auditService.GetAuditJobSteps(r.Context(), chi.URLParam(r, "id"))
				if err != nil {
					writeError(w, http.StatusInternalServerError, "audit_steps_query_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, steps)
			})
		})

		r.Route("/ai", func(r chi.Router) {
			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				enabled := false
				if aiService != nil {
					enabled = aiService.HasProvider()
				}
				writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
			})

			r.Post("/audit-compose", func(w http.ResponseWriter, r *http.Request) {
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

			r.Get("/audit-compose/{id}", func(w http.ResponseWriter, r *http.Request) {
				if aiService == nil || !aiService.HasProvider() {
					writeError(w, http.StatusServiceUnavailable, "ai_unavailable", "AI provider not configured")
					return
				}
				id := chi.URLParam(r, "id")
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()

				config, err := dockerClient.GetContainerComposeConfig(ctx, id, portainerService)
				if err != nil {
					writeError(w, http.StatusNotFound, "config_not_found", err.Error())
					return
				}

				analysis, err := aiService.AuditCompose(ctx, config)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "ai_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"analysis": analysis, "config": config})
			})

			r.Post("/analyze-metrics", func(w http.ResponseWriter, r *http.Request) {
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
		})

		r.Route("/scheduler", func(r chi.Router) {
			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				enabled := schedSvc != nil
				writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
			})

			r.Get("/schedules", func(w http.ResponseWriter, r *http.Request) {
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

			r.Post("/toggle", func(w http.ResponseWriter, r *http.Request) {
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

			r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
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
		})

		r.Get("/metrics/{id}", func(w http.ResponseWriter, r *http.Request) {
			if metricService == nil {
				writeError(w, http.StatusServiceUnavailable, "metrics_unavailable", "Metrics service not initialized")
				return
			}
			duration := r.URL.Query().Get("duration")
			if duration == "" {
				duration = "24h"
			}
			data, err := metricService.GetMetrics(r.Context(), chi.URLParam(r, "id"), duration)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "metrics_query_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, data)
		})

		r.Route("/system", func(r chi.Router) {
			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				if diagService == nil {
					writeError(w, http.StatusServiceUnavailable, "diag_unavailable", "Diagnostic service not initialized")
					return
				}
				writeJSON(w, http.StatusOK, diagService.GetSystemStatus())
			})

			r.Get("/logs", func(w http.ResponseWriter, r *http.Request) {
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
		})

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				if settingsService == nil {
					writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
					return
				}
				st, err := settingsService.Get(r.Context())
				if err != nil {
					writeError(w, http.StatusInternalServerError, "settings_get_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, st)
			})

			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				if settingsService == nil {
					writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
					return
				}
				var st settings.Settings
				if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
					return
				}
				if err := settingsService.Save(r.Context(), st); err != nil {
					writeError(w, http.StatusInternalServerError, "settings_save_failed", err.Error())
					return
				}

				if st.DiscordWebhookURL != "" {
					notificationService.AddDispatcher(notifications.NewDiscordDispatcher(st.DiscordWebhookURL))
				}

				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			})
		})

		r.Get("/portainer/stacks", func(w http.ResponseWriter, r *http.Request) {
			if portainerService == nil {
				writeError(w, http.StatusServiceUnavailable, "portainer_unavailable", "Portainer integration not configured")
				return
			}
			stacks, err := portainerService.ListStacks(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "portainer_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, stacks)
		})
	})

	staticDir := filepath.Clean(filepath.Join("..", "web", "dist"))
	fs := http.FileServer(http.Dir(staticDir))
	r.Handle("/*", spaHandler(fs, staticDir))

	return r
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
