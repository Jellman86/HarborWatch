package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/audit"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/notifications"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/releases"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/scanning"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type DockerClient interface {
	ListContainers(ctx context.Context) ([]gen.ContainerSummary, error)
	GetContainer(ctx context.Context, id string) (gen.ContainerSummary, error)
	GetContainerLogs(ctx context.Context, id string, tail int, since time.Time, timestamps bool) (dockerengine.ContainerLogs, error)
	GetContainerComposeConfig(ctx context.Context, id string, portainer *portainer.Client) (string, error)
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
	OpenEventStream(ctx context.Context) (io.ReadCloser, error)
}

type ScanService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScanPath(targetLabel, scanPath string, cleanup bool) (gen.ScanStartResponse, error)
	Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error)
	LatestSummary(ctx context.Context) (*gen.ScanSummary, error)
	LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error)
	LatestDetailsForTarget(ctx context.Context, target string) (*gen.TrivyScanDetails, error)
	MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error)
	MalwareSummariesForContainer(ctx context.Context, containerID string) ([]gen.MalwareScanSummary, error)
	MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error)
	MalwareDetailsForContainer(ctx context.Context, containerID string, limit int) ([]gen.MalwareScanDetail, error)
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
	RemoveDispatcher(name string)
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
	AddTask(spec string, task scheduler.Task, enabled bool) error
	RemoveTask(name string)
	ToggleTask(ctx context.Context, name string, enabled bool) error
	UpdateTaskSchedule(ctx context.Context, name string, spec string) error
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
	mux, schedSvc, err := NewMuxWithSchedulerE()
	if err != nil {
		log.Printf("failed to initialize harborwatch services: %v", err)
		return newDegradedMux(err), nil
	}
	return mux, schedSvc
}

func NewMuxWithSchedulerE() (http.Handler, *scheduler.Service, error) {
	dbPath := os.Getenv("HARBORWATCH_DB_PATH")
	if dbPath == "" {
		dbPath = "/tmp/harborwatch.db"
	}

	// Unified Database Connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL;`); err != nil {
		log.Printf("Warning: failed to set db pragmas: %v", err)
	}

	// 1. Diagnostics Setup
	diagService := diag.NewService(db)
	if diagService != nil {
		if err := diagService.Init(context.Background()); err != nil {
			return nil, nil, fmt.Errorf("init diagnostics service: %w", err)
		}
		diagService.Log("INFO", "System", "HarborWatch initializing...")
	}

	dockerClient, err := dockerengine.NewFromEnv()
	if err != nil {
		if diagService != nil {
			diagService.Log("ERROR", "Docker", fmt.Sprintf("Failed to init docker client: %v", err))
		}
		dockerClient = nil
	}

	// 2. Open Stores
	updatesStore := updates.NewStore(db)
	if err := updatesStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init updates store: %w", err)
	}

	settingsStore := settings.NewStore(db)
	if err := settingsStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init settings store: %w", err)
	}

	rulesStore := rules.NewStore(db)
	if err := rulesStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init rules store: %w", err)
	}

	// 3. Initialize Domain Services
	// Use shared store for scanning
	scanStore := scanning.NewStore(db)
	if err := scanStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init scanning store: %w", err)
	}
	scanService := scanning.NewService(scanning.NewTrivyScanner(), scanning.NewClamAVScanner(), scanStore, diagService)

	releaseService := releases.NewService()
	aiService := ai.NewService(nil)
	notificationService := notifications.NewService()

	var portainerService *portainer.Client
	st, err := settingsStore.Get(context.Background())
	if err != nil {
		if diagService != nil {
			diagService.Log("ERROR", "Settings", fmt.Sprintf("Failed to load settings on startup: %v", err))
		}
		aiService.SetProvider(ai.NewProviderFromEnv())
	} else {
		if st.DiscordEnabled && st.DiscordWebhookURL != "" {
			notificationService.AddDispatcher(notifications.NewDiscordDispatcher(st.DiscordWebhookURL))
		}
		if st.PortainerEnabled && st.PortainerURL != "" {
			portainerService = portainer.NewClient(st.PortainerURL, st.PortainerApiKey)
		}
		if st.AIEnabled {
			aiService.SetProvider(ai.NewProviderFromConfig(ai.ProviderConfig{
				Preferred:      st.AIProvider,
				OpenAIKey:      st.OpenAIKey,
				OpenAIModel:    st.OpenAIModel,
				AnthropicKey:   st.AnthropicKey,
				AnthropicModel: st.AnthropicModel,
				GeminiKey:      st.GeminiKey,
				GeminiModel:    st.GeminiModel,
			}))
		} else {
			aiService.SetProvider(nil)
		}
	}

	updateService := updates.NewService(updatesStore, updates.NewCommandExecutor(), aiService, notificationService, diagService)

	auditService := audit.NewService(db)

	// 4. Metrics Setup
	var metricService *metrics.Service
	if dockerClient != nil {
		rawDocker, err := dockerengine.NewRawClient()
		if err != nil {
			if diagService != nil {
				diagService.Log("ERROR", "Metrics", fmt.Sprintf("Failed to init raw docker client for metrics: %v", err))
			}
		} else if rawDocker != nil {
			metricStore := metrics.NewStore(db)
			if err := metricStore.Init(context.Background()); err != nil {
				return nil, nil, fmt.Errorf("init metrics store: %w", err)
			}
			metricService = metrics.NewService(metricStore, rawDocker)
		}
	}

	// 5. Scheduler Setup
	schedStore := scheduler.NewStore(db)
	if err := schedStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init scheduler store: %w", err)
	}
	schedSvc := scheduler.NewService(schedStore)

	isTaskGloballyEnabled := func(ctx context.Context, taskID string) bool {
		if schedSvc == nil || strings.TrimSpace(taskID) == "" {
			return true
		}
		entries, err := schedSvc.ListSchedules(ctx)
		if err != nil {
			return true
		}
		for _, entry := range entries {
			if entry.ID == taskID {
				return entry.Enabled
			}
		}
		return true
	}

	containerAutomationEnabled := func(ctx context.Context, containerID, domain string) bool {
		taskID := ""
		switch strings.ToLower(strings.TrimSpace(domain)) {
		case "upgrades":
			taskID = "container_update_check"
		case "maintenance":
			taskID = "docker_system_prune"
		case "security":
			// Security domain controls both Trivy and ClamAV sweeps.
			taskID = "security_sweep_trivy"
		default:
			return true
		}

		globalEnabled := isTaskGloballyEnabled(ctx, taskID)
		if rulesStore == nil {
			return globalEnabled
		}
		rulesCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		rule, err := rulesStore.Get(rulesCtx, containerID)
		if err != nil {
			return globalEnabled
		}
		if !rule.InheritAutomation && !rule.UpgradesAutomation && !rule.MaintenanceAutomation && !rule.SecurityAutomation {
			rule.InheritAutomation = true
		}
		if rule.InheritAutomation {
			return globalEnabled
		}
		switch strings.ToLower(strings.TrimSpace(domain)) {
		case "upgrades":
			return rule.UpgradesAutomation
		case "maintenance":
			return rule.MaintenanceAutomation
		case "security":
			return rule.SecurityAutomation
		default:
			return globalEnabled
		}
	}

	// 6. Register automated tasks
	if dockerClient != nil {
		rawDocker, err := dockerengine.NewRawClient()
		if err != nil {
			if diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to init raw docker client for scheduled tasks: %v", err))
			}
		} else if rawDocker != nil {
			// Maintenance: Weekly Prune
			schedSvc.RegisterTask("docker_system_prune", func() scheduler.Task {
				return scheduler.NewDockerPruneTask(rawDocker)
			})
			if err := schedSvc.AddTask("0 0 3 * * 0", scheduler.NewDockerPruneTask(rawDocker), false); err != nil && diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task docker_system_prune: %v", err))
			}

			schedSvc.RegisterTask("container_update_check", func() scheduler.Task {
				return dockerengine.NewUpdateCheckTask(rawDocker, func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "upgrades")
				})
			})
			if err := schedSvc.AddTask("0 0 * * * *", dockerengine.NewUpdateCheckTask(rawDocker, func(ctx context.Context, containerID string) bool {
				return containerAutomationEnabled(ctx, containerID, "upgrades")
			}), true); err != nil && diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task container_update_check: %v", err))
			}

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
				if err := schedSvc.AddTask("0 * * * * *", metricService.GetCollectorTask(), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task metrics_collector: %v", err))
				}

				// Seed initial datapoints shortly after boot so dashboards don't stay empty until next cron boundary.
				go func() {
					time.Sleep(15 * time.Second)
					_ = schedSvc.RunTask(context.Background(), "metrics_collector")
				}()

				schedSvc.RegisterTask("metrics_prune", func() scheduler.Task {
					return metricService.GetPruneTask()
				})
				if err := schedSvc.AddTask("0 0 0 * * *", metricService.GetPruneTask(), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task metrics_prune: %v", err))
				}
			}

			// Security: Scheduled Sweeps
			if scanService != nil {
				schedSvc.RegisterTask("security_sweep_trivy", func() scheduler.Task {
					return scheduler.NewTrivySweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
						return containerAutomationEnabled(ctx, containerID, "security")
					})
				})
				if err := schedSvc.AddTask("0 0 0 * * *", scheduler.NewTrivySweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "security")
				}), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task security_sweep_trivy: %v", err))
				}

				schedSvc.RegisterTask("malware_sweep_clamav", func() scheduler.Task {
					return scheduler.NewClamAVSweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
						return containerAutomationEnabled(ctx, containerID, "security")
					})
				})
				if err := schedSvc.AddTask("0 0 4 * * 0", scheduler.NewClamAVSweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "security")
				}), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task malware_sweep_clamav: %v", err))
				}
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
		if err := schedSvc.AddTask("0 0 1 * * *", scheduler.NewGenericTask("diag_log_prune", func(ctx context.Context) error {
			olderThan := time.Now().Add(-7 * 24 * time.Hour).Unix()
			_, err := diagService.PruneLogs(ctx, olderThan)
			return err
		}), true); err != nil {
			diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task diag_log_prune: %v", err))
		}
	}

	// Bootstrap schedules from DB
	if err := schedSvc.LoadSchedules(context.Background()); err != nil && diagService != nil {
		diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to load schedules from DB: %v", err))
	}

	return NewMuxWithDeps(dockerClient, scanService, releaseService, updateService, auditService, aiService, schedSvc, metricService, diagService, notificationService, settingsStore, portainerService, rulesStore), schedSvc, nil
}

func newDegradedMux(initErr error) http.Handler {
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":  "degraded",
			"service": "harborwatch",
			"error":   initErr.Error(),
		})
	})
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusServiceUnavailable, "startup_failed", initErr.Error())
	})
	return r
}

func NewMuxWithDeps(dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService, schedSvc SchedulerService, metricService MetricsService, diagService DiagService, notificationService NotificationService, settingsService SettingsService, portainerService *portainer.Client, rulesService RulesService) http.Handler {
	r := chi.NewRouter()
	currentPortainerService := portainerService

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(diagnosticsHTTPErrorLogger(diagService))

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

			r.Post("/prune", func(w http.ResponseWriter, r *http.Request) {
				if schedSvc == nil {
					writeError(w, http.StatusServiceUnavailable, "scheduler_unavailable", "Scheduler not initialized")
					return
				}
				if err := schedSvc.RunTask(r.Context(), "docker_system_prune"); err != nil {
					writeError(w, http.StatusInternalServerError, "prune_trigger_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "triggered", "task": "docker_system_prune"})
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
				heartbeat := time.NewTicker(10 * time.Second)
				defer heartbeat.Stop()
				scanner := bufio.NewScanner(stream)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				lineCh := make(chan []byte, 1)
				errCh := make(chan error, 1)
				go func() {
					defer close(lineCh)
					for scanner.Scan() {
						// Scanner.Bytes() is invalidated on next Scan; copy before handoff.
						line := append([]byte(nil), scanner.Bytes()...)
						select {
						case lineCh <- line:
						case <-r.Context().Done():
							return
						}
					}
					if err := scanner.Err(); err != nil {
						errCh <- err
					}
				}()

				for {
					select {
					case <-r.Context().Done():
						return
					case <-heartbeat.C:
						fmt.Fprint(w, ": ping\n\n")
						flusher.Flush()
					case err := <-errCh:
						if err != nil && r.Context().Err() == nil {
							fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
							flusher.Flush()
						}
						return
					case line, ok := <-lineCh:
						if !ok {
							return
						}
						event := convertEvent(line)
						payload, err := json.Marshal(event)
						if err != nil {
							continue
						}
						fmt.Fprintf(w, "event: docker\ndata: %s\n\n", payload)
						flusher.Flush()
					}
				}
			})

			getContainerDetail := func(w http.ResponseWriter, r *http.Request) {
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
					Summary:        summary,
					MalwareSummary: []gen.MalwareScanSummary{},
					RecentMetrics:  []gen.Metric{},
					ActionHistory:  []gen.AuditJobSummary{},
				}

				if du, err := collectContainerDiskUsage(ctx, id); err == nil {
					detail.DiskUsage = du
				}

				// Enrich with Security Data
				if scanService != nil {
					if vs, err := scanService.LatestSummaryForTarget(ctx, summary.Image); err == nil {
						detail.VulnerabilitySummary = vs
					}
					if ms, err := scanService.MalwareSummariesForContainer(ctx, id); err == nil {
						detail.MalwareSummary = ms
					} else {
						detail.MalwareSummary = []gen.MalwareScanSummary{}
					}
				}

				// Enrich with Metrics
				if metricService != nil {
					if m, err := metricService.GetMetrics(ctx, id, "1h"); err == nil {
						// Convert internal metrics to gen.Metric
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
					} else {
						detail.ActionHistory = []gen.AuditJobSummary{}
					}
				}

				// Enrich with Rules
				if rulesService != nil {
					if r, err := rulesService.Get(ctx, id); err == nil {
						r = effectiveContainerRules(ctx, summary, r, settingsService, diagService)
						detail.Rules = &gen.ContainerRules{
							ContainerID:           r.ContainerID,
							UpdatePolicy:          r.UpdatePolicy,
							ValidateURL:           r.ValidateURL,
							ValidateMode:          r.ValidateMode,
							ValidateTimeoutSec:    r.ValidateTimeoutSec,
							ValidateIntervalSec:   r.ValidateIntervalSec,
							AIValidateLogs:        r.AIValidateLogs,
							AutoRollback:          r.AutoRollback,
							InheritAutomation:     r.InheritAutomation,
							UpgradesAutomation:    r.UpgradesAutomation,
							MaintenanceAutomation: r.MaintenanceAutomation,
							SecurityAutomation:    r.SecurityAutomation,
						}
					}
				}

				// Fetch Compose Config if requested or for enrichment
				// For now we don't return full YAML in the Detail object to keep it light,
				// but the backend is ready for the Doctor.

				writeJSON(w, http.StatusOK, detail)
			}

			getContainerLogs := func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
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
				logs, err := dockerClient.GetContainerLogs(ctx, id, tail, since, timestamps)
				if err != nil {
					if diagService != nil {
						diagService.Log("ERROR", "DockerLogs", fmt.Sprintf("Failed to load logs for %s: %v", id, err))
					}
					status := http.StatusBadGateway
					if isContainerNotFoundError(err) {
						status = http.StatusNotFound
					}
					writeError(w, status, "container_logs_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, logs)
			}

			r.Get("/{id}", getContainerDetail)
			// Backward-compatible alias for older frontend builds that still call /docker/containers/{id}.
			r.Get("/containers/{id}", getContainerDetail)
			r.Get("/{id}/logs", getContainerLogs)
			// Backward-compatible alias for older frontend builds.
			r.Get("/containers/{id}/logs", getContainerLogs)

			registerRulesRoutes := func(router chi.Router) {
				getRules := func(w http.ResponseWriter, r *http.Request) {
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
					summary := gen.ContainerSummary{ID: id}
					if dockerClient != nil {
						ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
						if c, err := dockerClient.GetContainer(ctx, id); err == nil {
							summary = c
						}
						cancel()
					}
					res = effectiveContainerRules(r.Context(), summary, res, settingsService, diagService)
					writeJSON(w, http.StatusOK, res)
				}

				saveRules := func(w http.ResponseWriter, r *http.Request) {
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
					req.UpdatePolicy = normalizeUpdatePolicy(req.UpdatePolicy)
					summary := gen.ContainerSummary{ID: req.ContainerID}
					if dockerClient != nil {
						ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
						if c, err := dockerClient.GetContainer(ctx, req.ContainerID); err == nil {
							summary = c
						}
						cancel()
					}
					req = effectiveContainerRules(r.Context(), summary, req, settingsService, diagService)
					if err := rulesService.Save(r.Context(), req); err != nil {
						writeError(w, http.StatusInternalServerError, "rules_save_failed", err.Error())
						return
					}
					writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
				}

				// Support both trailing-slash and non-trailing-slash forms.
				router.Get("/", getRules)
				router.Post("/", saveRules)
			}

			r.Route("/{id}/rules", registerRulesRoutes)
			// Backward-compatible route for older frontend builds.
			r.Route("/containers/{id}/rules", registerRulesRoutes)
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

			r.Post("/malware/container/{id}", func(w http.ResponseWriter, r *http.Request) {
				if scanService == nil {
					writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
					return
				}
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				containerID := chi.URLParam(r, "id")
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				_, err := dockerClient.GetContainer(ctx, containerID)
				cancel()
				if err != nil {
					writeError(w, http.StatusNotFound, "container_not_found", err.Error())
					return
				}
				go triggerContainerMalwareScans(containerID, scanService, diagService)
				writeJSON(w, http.StatusAccepted, map[string]string{
					"status":      "queued",
					"containerId": containerID,
				})
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
				// Return 200 null if no summary exists, preventing 404 console errors
				writeJSON(w, http.StatusOK, summary)
			})

			r.Get("/details", func(w http.ResponseWriter, r *http.Request) {
				if scanService == nil {
					writeError(w, http.StatusServiceUnavailable, "scanner_unavailable", "Scanner service not initialized")
					return
				}
				target := strings.TrimSpace(r.URL.Query().Get("target"))
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				details, err := scanService.LatestDetailsForTarget(ctx, target)
				if err != nil {
					writeError(w, http.StatusBadGateway, "scan_read_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, details)
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

			r.Get("/malware/details", func(w http.ResponseWriter, r *http.Request) {
				if scanService == nil {
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
				details, err := scanService.MalwareDetails(ctx, target, prefix, limit)
				if err != nil {
					writeError(w, http.StatusBadGateway, "malware_scan_read_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, details)
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
				req.ContainerID = strings.TrimSpace(req.ContainerID)
				req.TargetImage = strings.TrimSpace(req.TargetImage)
				req.ValidateURL = strings.TrimSpace(req.ValidateURL)
				if req.ContainerID == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "containerId is required")
					return
				}

				containerCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				summary := gen.ContainerSummary{ID: req.ContainerID}
				if dockerClient != nil {
					if c, err := dockerClient.GetContainer(containerCtx, req.ContainerID); err == nil {
						summary = c
					}
				}
				cancel()

				if req.TargetImage == "" {
					req.TargetImage = strings.TrimSpace(summary.Image)
				}

				effectiveRules := rules.ContainerRules{
					ContainerID:           req.ContainerID,
					UpdatePolicy:          "manual",
					ValidateMode:          "both",
					ValidateTimeoutSec:    45,
					ValidateIntervalSec:   2,
					AIValidateLogs:        false,
					AutoRollback:          true,
					InheritAutomation:     true,
					UpgradesAutomation:    true,
					MaintenanceAutomation: true,
					SecurityAutomation:    true,
				}
				if rulesService != nil {
					rulesCtx, rulesCancel := context.WithTimeout(r.Context(), 3*time.Second)
					if loaded, err := rulesService.Get(rulesCtx, req.ContainerID); err == nil {
						effectiveRules = loaded
					}
					rulesCancel()
				}
				effectiveRules = effectiveContainerRules(r.Context(), summary, effectiveRules, settingsService, diagService)

				if req.ValidateURL == "" {
					req.ValidateURL = strings.TrimSpace(effectiveRules.ValidateURL)
				}
				if req.TargetImage == "" || req.ValidateURL == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "targetImage and validateUrl could not be auto-derived; provide explicit values")
					return
				}

				repoURL := firstNonEmpty(
					summary.Labels["harborwatch.intel.url"],
					summary.Labels["org.opencontainers.image.source"],
					summary.Labels["org.label-schema.vcs-url"],
				)
				releaseContext := ""
				if releaseService != nil {
					if repo, ok := deriveGithubRepo(repoURL); ok {
						currentTag := imageTagFromRef(summary.Image)
						targetTag := imageTagFromRef(req.TargetImage)
						releaseCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
						releaseContext = buildReleaseContext(releaseCtx, releaseService, repo, currentTag, targetTag)
						cancel()
					}
				}

				resp, err := updateService.StartUpdate(updates.Request{
					ContainerID:         req.ContainerID,
					TargetImage:         req.TargetImage,
					ValidateURL:         req.ValidateURL,
					CurrentImage:        strings.TrimSpace(summary.Image),
					ContainerName:       trimContainerName(summary.Names),
					Labels:              summary.Labels,
					RepositoryURL:       repoURL,
					ReleaseContext:      releaseContext,
					ValidateMode:        effectiveRules.ValidateMode,
					ValidateTimeoutSec:  effectiveRules.ValidateTimeoutSec,
					ValidateIntervalSec: effectiveRules.ValidateIntervalSec,
					AIValidateLogs:      effectiveRules.AIValidateLogs,
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
			r.Post("/fleet-advice", func(w http.ResponseWriter, r *http.Request) {
				var containers []gen.ContainerSummary
				if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&containers); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
					return
				}

				advice := generateFleetAdvice(containers, aiService != nil && aiService.HasProvider())
				writeJSON(w, http.StatusOK, map[string]string{"advice": advice})
			})

			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				enabled := false
				if aiService != nil {
					enabled = aiService.HasProvider()
				}
				writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
			})

			r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
				if settingsService == nil {
					writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
					return
				}
				st, err := settingsService.Get(r.Context())
				if err != nil {
					writeError(w, http.StatusInternalServerError, "settings_get_failed", err.Error())
					return
				}
				catalogCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()
				provider := strings.TrimSpace(r.URL.Query().Get("provider"))
				catalog := loadProviderModelCatalog(catalogCtx, st, provider)
				writeJSON(w, http.StatusOK, catalog)
			})

			r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
				if settingsService == nil {
					writeError(w, http.StatusServiceUnavailable, "settings_unavailable", "Settings service not initialized")
					return
				}
				var req struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
				}
				if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
					return
				}
				req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
				if req.Provider == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "provider is required")
					return
				}

				st, err := settingsService.Get(r.Context())
				if err != nil {
					writeError(w, http.StatusInternalServerError, "settings_get_failed", err.Error())
					return
				}
				testCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()
				provider, model, summary, err := testProviderModel(testCtx, req.Provider, req.Model, st)
				if err != nil {
					writeError(w, http.StatusBadGateway, "ai_test_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{
					"status":   "ok",
					"provider": provider,
					"model":    model,
					"summary":  summary,
				})
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
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				id := chi.URLParam(r, "id")
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()

				config, err := dockerClient.GetContainerComposeConfig(ctx, id, currentPortainerService)
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

			r.Post("/update", func(w http.ResponseWriter, r *http.Request) {
				if schedSvc == nil {
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
				if err := schedSvc.UpdateTaskSchedule(r.Context(), req.ID, req.CronSpec); err != nil {
					writeError(w, http.StatusBadRequest, "update_failed", err.Error())
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

		r.Post("/metrics/batch", func(w http.ResponseWriter, r *http.Request) {
			if metricService == nil {
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
				data, err := metricService.GetMetrics(r.Context(), id, duration)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "metrics_query_failed", err.Error())
					return
				}
				out[id] = data
			}
			writeJSON(w, http.StatusOK, out)
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
				limit := parseIntQuery(r.URL.Query().Get("limit"), 100, 1, 2000)
				fetchLimit := limit
				if fetchLimit < 500 {
					fetchLimit = 500
				}
				level := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("level")))
				source := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("source")))
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

				logs, err := diagService.ListLogs(r.Context(), fetchLimit)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "diag_log_failed", err.Error())
					return
				}

				filtered := make([]diag.LogEntry, 0, min(limit, len(logs)))
				for _, entry := range logs {
					if level != "" && !strings.EqualFold(entry.Level, level) {
						continue
					}
					if source != "" && !strings.Contains(strings.ToLower(entry.Source), source) {
						continue
					}
					if since > 0 && entry.Timestamp < since {
						continue
					}
					filtered = append(filtered, entry)
					if len(filtered) >= limit {
						break
					}
				}

				writeJSON(w, http.StatusOK, filtered)
			})
		})

		r.Route("/diagnostics", func(r chi.Router) {
			r.Get("/containers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
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
				logs, err := dockerClient.GetContainerLogs(ctx, id, tail, since, timestamps)
				if err != nil {
					if diagService != nil {
						diagService.Log("ERROR", "DockerLogs", fmt.Sprintf("Failed to load logs for %s: %v", id, err))
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
						dockerClient: dockerClient,
						scanService:  scanService,
						auditService: auditService,
						schedSvc:     schedSvc,
						diagService:  diagService,
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

				fresh := st
				if settingsService != nil {
					if loaded, err := settingsService.Get(r.Context()); err == nil {
						fresh = loaded
					}
				}

				if notificationService != nil {
					if !fresh.DiscordEnabled || strings.TrimSpace(fresh.DiscordWebhookURL) == "" {
						notificationService.RemoveDispatcher("discord")
					} else {
						notificationService.AddDispatcher(notifications.NewDiscordDispatcher(fresh.DiscordWebhookURL))
					}
				}

				if fresh.PortainerEnabled && strings.TrimSpace(fresh.PortainerURL) != "" {
					currentPortainerService = portainer.NewClient(fresh.PortainerURL, fresh.PortainerApiKey)
				} else {
					currentPortainerService = nil
				}

				if setter, ok := aiService.(interface{ SetProvider(ai.Provider) }); ok {
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

		r.Get("/portainer/stacks", func(w http.ResponseWriter, r *http.Request) {
			if currentPortainerService == nil {
				writeError(w, http.StatusServiceUnavailable, "portainer_unavailable", "Portainer integration not configured")
				return
			}
			stacks, err := currentPortainerService.ListStacks(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "portainer_error", err.Error())
				return
			}
			if stacks == nil {
				stacks = []portainer.Stack{}
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

func generateFleetAdvice(containers []gen.ContainerSummary, aiEnabled bool) string {
	if len(containers) == 0 {
		return "No fleet inventory was provided. Refresh inventory and run this analysis again."
	}

	total := len(containers)
	running := 0
	stopped := 0
	updates := 0

	for _, c := range containers {
		if strings.EqualFold(c.State, "running") {
			running++
		} else {
			stopped++
		}
		if c.UpdateAvailable {
			updates++
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Fleet snapshot: %d containers (%d running, %d not running).\n", total, running, stopped))

	if updates > 0 {
		b.WriteString(fmt.Sprintf("- %d container(s) report image updates. Prioritize staging validation before production rollout.\n", updates))
	} else {
		b.WriteString("- No image updates are currently flagged for active containers.\n")
	}

	if stopped > 0 {
		b.WriteString("- Review stopped containers for orphaned workloads or intentional maintenance state.\n")
	}

	if running == total && updates == 0 {
		b.WriteString("- Operationally stable posture detected; continue routine vulnerability and malware scans.\n")
	}

	if !aiEnabled {
		b.WriteString("- AI provider is not configured. This report is heuristic-only.")
	}

	return strings.TrimSpace(b.String())
}

func spaHandler(static http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(filepath.Join(staticDir, r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Cache immutable assets for 1 year
			if strings.Contains(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			static.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
