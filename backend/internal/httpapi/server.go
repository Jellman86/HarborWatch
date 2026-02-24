package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"github.com/Jellman86/HarborWatch/backend/internal/containerintel"
	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/healthremediation"
	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	"github.com/Jellman86/HarborWatch/backend/internal/metrics"
	"github.com/Jellman86/HarborWatch/backend/internal/migrations"
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
	RestartContainer(ctx context.Context, id string) error
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
	OpenEventStream(ctx context.Context) (io.ReadCloser, error)
}

type ScanService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScanPath(targetLabel, containerName, scanPath string, cleanup bool) (gen.ScanStartResponse, error)
	CancelJob(ctx context.Context, jobID string) (gen.ScanJobStatus, error)
	ClamAVSignatureStatus(ctx context.Context) (scanning.ClamAVSignatureStatus, error)
	UpdateClamAVSignatures(ctx context.Context) (string, error)
	Job(ctx context.Context, jobID string) (gen.ScanJobStatus, error)
	ListJobs(ctx context.Context, scanType, targetPrefix string, limit int) ([]gen.ScanJobStatus, error)
	ActiveJobs() []gen.JobProgress
	LatestSummary(ctx context.Context) (*gen.ScanSummary, error)
	LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error)
	LatestDetailsForTarget(ctx context.Context, target string) (*gen.TrivyScanDetails, error)
	MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error)
	MalwareSummariesForContainer(ctx context.Context, containerID, containerName string) ([]gen.MalwareScanSummary, error)
	MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error)
	MalwareDetailsForContainer(ctx context.Context, containerID, containerName string, limit int) ([]gen.MalwareScanDetail, error)
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
}

type ReleaseService interface {
	Analyze(ctx context.Context, repo string) (gen.ReleaseRiskSummary, error)
}

type AIService interface {
	HasProvider() bool
	AnalyzeReleaseNotes(ctx context.Context, notes string) (ai.AnalysisResult, error)
	AnalyzeFleet(ctx context.Context, inventory string) (string, error)
	AuditCompose(ctx context.Context, yaml string) (string, error)
	AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error)
	ListConversations(ctx context.Context, limit, offset int) ([]ai.ConversationRecord, error)
	GetLatestFleetAdvice(ctx context.Context) (ai.FleetAdviceRecord, error)
	SaveFleetAdvice(ctx context.Context, rec ai.FleetAdviceRecord) error
}

type DiagService interface {
	Log(level, source, message string)
	ListLogs(ctx context.Context, limit, offset int, level, source, search string, since int64) ([]diag.LogEntry, int, error)
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
	Get(ctx context.Context, id, name string) (rules.ContainerRules, error)
	Save(ctx context.Context, r rules.ContainerRules) error
}

type AuditService interface {
	ListAuditJobs(ctx context.Context) ([]gen.AuditJobSummary, error)
	ListAuditJobsForContainer(ctx context.Context, containerID, containerName string) ([]gen.AuditJobSummary, error)
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
	ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error)
	ActiveJobs() []gen.JobProgress
	Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func())
}

type ContainerIntelService interface {
	Get(ctx context.Context, id, name string) (containerintel.Override, error)
	Save(ctx context.Context, ov containerintel.Override) error
	List(ctx context.Context) ([]containerintel.Override, error)
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
	if migrationResult, err := migrations.Run(context.Background(), db); err != nil {
		return nil, nil, fmt.Errorf("run schema migrations: %w", err)
	} else {
		log.Printf("schema migrations initialized: version=%d applied=%d", migrationResult.CurrentVersion, migrationResult.AppliedCount)
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
	if recovered, err := updatesStore.MarkRunningRunsFailed(context.Background(), "update interrupted by HarborWatch restart"); err == nil {
		if recovered > 0 && diagService != nil {
			diagService.Log("WARN", "UpdateEngine", fmt.Sprintf("Recovered %d stale running update jobs after restart", recovered))
		}
	} else if diagService != nil {
		diagService.Log("ERROR", "UpdateEngine", fmt.Sprintf("Failed to reconcile stale update jobs on startup: %v", err))
	}

	settingsStore := settings.NewStore(db)
	if err := settingsStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init settings store: %w", err)
	}

	rulesStore := rules.NewStore(db)
	if err := rulesStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init rules store: %w", err)
	}
	intelStore := containerintel.NewStore(db)
	if err := intelStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init container intel store: %w", err)
	}

	// 3. Initialize Domain Services
	// Global Job Manager
	maxConcurrency := 1
	st, err := settingsStore.Get(context.Background())
	if err == nil {
		if st.AutoUpgradeMaxConcurrency > 0 {
			maxConcurrency = st.AutoUpgradeMaxConcurrency
		}
	}
	jobManager := jobs.NewManager(maxConcurrency)

	// Use shared store for scanning
	scanStore := scanning.NewStore(db)
	if err := scanStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init scanning store: %w", err)
	}
	if recovered, err := scanStore.MarkRunningJobsFailed(context.Background(), "scan interrupted by HarborWatch restart"); err == nil {
		if recovered > 0 && diagService != nil {
			diagService.Log("WARN", "Scanner", fmt.Sprintf("Recovered %d stale running scan jobs after restart", recovered))
		}
	} else if diagService != nil {
		diagService.Log("ERROR", "Scanner", fmt.Sprintf("Failed to reconcile stale scan jobs on startup: %v", err))
	}
	scanService := scanning.NewService(scanning.NewTrivyScanner(), scanning.NewClamAVScanner(), dockerClient, scanStore, diagService, jobManager)

	releaseService := releases.NewService()
	aiService := ai.NewService(nil)
	aiUsageStore := ai.NewUsageSQLiteStore(db)
	if err := aiUsageStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init ai usage store: %w", err)
	}
	aiService.SetUsageStore(aiUsageStore)
	composeAuditStore := ai.NewComposeAuditSQLiteStore(db)
	if err := composeAuditStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init compose audit history store: %w", err)
	}
	notificationService := notifications.NewService()

	// 3.5 Unhealthy Auto-Remediation
	hrSvc := healthremediation.NewService(db, dockerClient, rulesStore, settingsStore, diagService, jobManager)
	if err := hrSvc.Init(context.Background()); err != nil && diagService != nil {
		diagService.Log("ERROR", "HealthRemediation", fmt.Sprintf("Failed to init remediation store: %v", err))
	}

	var portainerService *portainer.Client
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

	updateService := updates.NewService(updatesStore, updates.NewCommandExecutor(), aiService, notificationService, diagService, portainerService, jobManager)

	auditService := audit.NewService(db)

	// 4. Metrics Setup
	var metricService *metrics.Service
	var metricStore *metrics.Store
	if dockerClient != nil {
		rawDocker, err := dockerengine.NewRawClient()
		if err != nil {
			if diagService != nil {
				diagService.Log("ERROR", "Metrics", fmt.Sprintf("Failed to init raw docker client for metrics: %v", err))
			}
		} else if rawDocker != nil {
			metricStore = metrics.NewStore(db)
			if err := metricStore.Init(context.Background()); err != nil {
				return nil, nil, fmt.Errorf("init metrics store: %w", err)
			}
			metricService = metrics.NewService(metricStore, rawDocker, settingsStore)
		}
	}

	// 5. Scheduler Setup
	schedStore := scheduler.NewStore(db)
	if err := schedStore.Init(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("init scheduler store: %w", err)
	}
	schedSvc := scheduler.NewService(schedStore)
	if diagService != nil {
		schedSvc.SetLogger(diagService)
	}

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

	loadRuntimeSettings := func(ctx context.Context) settings.Settings {
		st := settings.Settings{
			AutomationIgnoredContainers:        "harborwatch",
			TrivySweepMode:                     scheduler.TrivySweepModeRunningOnly,
			RetentionLogsDays:                  30,
			RetentionMetricsDays:               14,
			RetentionScanResultsDays:           30,
			RetentionScanJobsDays:              30,
			RetentionUpdateRunsDays:            90,
			RetentionComposeAuditDays:          90,
			RetentionAIUsageDays:               180,
			UnhealthyAutoRemediationEnabled:    true,
			UnhealthyRestartCooldownSecDefault: 300,
			MaxRestartsPerWindow:               3,
		}
		if settingsStore == nil {
			return st
		}
		settingsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		loaded, err := settingsStore.Get(settingsCtx)
		if err != nil {
			return st
		}
		return loaded
	}

	retentionDays := func(value int, fallback int) int {
		if value <= 0 {
			return fallback
		}
		if value > 3650 {
			return 3650
		}
		return value
	}

	isIgnoredContainer := func(ctx context.Context, containerID string) bool {
		containerID = strings.TrimSpace(containerID)
		if containerID == "" {
			return false
		}
		st := loadRuntimeSettings(ctx)
		tokens := append(splitDelimitedTokens(st.AutomationIgnoredContainers), "harborwatch", "portainer", "portainer-ce", "ix-portainer")
		seen := map[string]struct{}{}
		unique := make([]string, 0, len(tokens))
		for _, token := range tokens {
			key := strings.ToLower(strings.TrimSpace(token))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			unique = append(unique, token)
		}

		summary := gen.ContainerSummary{ID: containerID}
		if dockerClient != nil {
			inspectCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if c, err := dockerClient.GetContainer(inspectCtx, containerID); err == nil {
				summary = c
			}
			cancel()
		}
		return containerMatchesAnyToken(summary, unique)
	}

	isIgnoredMalwareMount := func(ctx context.Context, sourcePath string) bool {
		st := loadRuntimeSettings(ctx)
		patterns := splitDelimitedTokens(st.MalwareIgnoredMounts)
		return pathMatchesAnyPattern(sourcePath, patterns)
	}

	allowMalwareMountScan := func(ctx context.Context, containerID, sourcePath string) bool {
		if isIgnoredMalwareMount(ctx, sourcePath) {
			if diagService != nil {
				diagService.Log("INFO", "Scheduler", fmt.Sprintf("Skipping malware mount for %s: %s (ignored by settings)", containerID, sourcePath))
			}
			return false
		}
		if _, err := os.Stat(sourcePath); err != nil {
			if diagService != nil {
				diagService.Log("WARN", "Scheduler", fmt.Sprintf("Skipping malware mount for %s: %s (path not accessible inside HarborWatch container: %v)", containerID, sourcePath, err))
			}
			return false
		}
		return true
	}

	containerAutomationEnabled := func(ctx context.Context, containerID, domain string, taskIDs ...string) bool {
		if isIgnoredContainer(ctx, containerID) {
			return false
		}

		globalEnabled := true
		if len(taskIDs) > 0 {
			globalEnabled = false
			for _, taskID := range taskIDs {
				if isTaskGloballyEnabled(ctx, taskID) {
					globalEnabled = true
					break
				}
			}
		} else {
			taskID := ""
			switch strings.ToLower(strings.TrimSpace(domain)) {
			case "upgrades":
				taskID = "container_update_check"
			case "maintenance":
				taskID = "docker_system_prune"
			case "security":
				// Security domain controls both Trivy and ClamAV sweeps.
				taskID = "security_sweep_trivy"
			}
			if taskID != "" {
				globalEnabled = isTaskGloballyEnabled(ctx, taskID)
			}
		}
		if rulesStore == nil {
			return globalEnabled
		}
		rulesCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		containerName := "unknown"
		if dockerClient != nil {
			if c, err := dockerClient.GetContainer(rulesCtx, containerID); err == nil {
				if len(c.Names) > 0 {
					containerName = strings.TrimPrefix(c.Names[0], "/")
				}
			}
		}
		rule, err := rulesStore.Get(rulesCtx, containerID, containerName)
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
				return scheduler.NewDockerPruneTask(rawDocker).WithLogger(diagService)
			})
			if err := schedSvc.AddTask("0 0 3 * * 0", scheduler.NewDockerPruneTask(rawDocker).WithLogger(diagService), false); err != nil && diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task docker_system_prune: %v", err))
			}

			newUpdateCheckTask := func() *dockerengine.UpdateCheckTask {
				task := dockerengine.NewUpdateCheckTask(rawDocker, func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "upgrades", "container_update_check")
				})
				if settingsStore != nil {
					task = task.WithCompletionCallback(func(ctx context.Context, summary dockerengine.UpdateCheckSummary) {
						if err := settingsStore.SetDashboardUpdateCheckSnapshot(ctx, summary.AvailableCount, summary.CheckedAt); err != nil && diagService != nil {
							diagService.Log("WARN", "Scheduler", fmt.Sprintf("Failed to persist update check dashboard snapshot: %v", err))
						}
					})
				}
				return task
			}

			schedSvc.RegisterTask("container_update_check", func() scheduler.Task {
				return newUpdateCheckTask()
			})
			if err := schedSvc.AddTask("0 0 0 * * *", newUpdateCheckTask(), true); err != nil && diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task container_update_check: %v", err))
			}
			refreshUpdateStatus := func(ctx context.Context) error {
				return newUpdateCheckTask().Run(ctx)
			}

			schedSvc.RegisterTask("container_update_apply", func() scheduler.Task {
				return newAutomatedUpdateApplyTask(
					dockerClient,
					portainerService,
					updateService,
					rulesStore,
					settingsStore,
					intelStore,
					releaseService,
					diagService,
					func(ctx context.Context, containerID string) bool {
						return containerAutomationEnabled(ctx, containerID, "upgrades", "container_update_apply")
					},
					refreshUpdateStatus,
				)
			})
			if err := schedSvc.AddTask("0 10 0 * * *", newAutomatedUpdateApplyTask(
				dockerClient,
				portainerService,
				updateService,
				rulesStore,
				settingsStore,
				intelStore,
				releaseService,
				diagService,
				func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "upgrades", "container_update_apply")
				},
				refreshUpdateStatus,
			), false); err != nil && diagService != nil {
				diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task container_update_apply: %v", err))
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
					return scheduler.NewGenericTask("metrics_prune", func(ctx context.Context) error {
						if metricStore == nil {
							return nil
						}
						st := loadRuntimeSettings(ctx)
						days := retentionDays(st.RetentionMetricsDays, 14)
						olderThan := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
						pruned, err := metricStore.PruneMetrics(ctx, olderThan)
						if err != nil {
							return err
						}
						if diagService != nil {
							diagService.Log("INFO", "Scheduler", fmt.Sprintf("Metrics retention prune completed: deleted=%d days=%d", pruned, days))
						}
						return nil
					})
				})
				if err := schedSvc.AddTask("0 0 0 * * *", scheduler.NewGenericTask("metrics_prune", func(ctx context.Context) error {
					if metricStore == nil {
						return nil
					}
					st := loadRuntimeSettings(ctx)
					days := retentionDays(st.RetentionMetricsDays, 14)
					olderThan := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
					pruned, err := metricStore.PruneMetrics(ctx, olderThan)
					if err != nil {
						return err
					}
					if diagService != nil {
						diagService.Log("INFO", "Scheduler", fmt.Sprintf("Metrics retention prune completed: deleted=%d days=%d", pruned, days))
					}
					return nil
				}), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task metrics_prune: %v", err))
				}
			}

			// Security: Scheduled Sweeps
			if scanService != nil {
				newTrivyTask := func(ctx context.Context) scheduler.Task {
					st := loadRuntimeSettings(ctx)
					return scheduler.NewTrivySweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
						return containerAutomationEnabled(ctx, containerID, "security", "security_sweep_trivy")
					}).WithMode(st.TrivySweepMode).WithLogger(diagService)
				}

				schedSvc.RegisterTask("security_sweep_trivy", func() scheduler.Task {
					return newTrivyTask(context.Background())
				})
				if err := schedSvc.AddTask("0 0 0 * * *", newTrivyTask(context.Background()), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task security_sweep_trivy: %v", err))
				}

				schedSvc.RegisterTask("malware_sweep_clamav", func() scheduler.Task {
					return scheduler.NewClamAVSweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
						return containerAutomationEnabled(ctx, containerID, "security", "malware_sweep_clamav")
					}).WithMountPolicy(func(ctx context.Context, containerID, sourcePath string) bool {
						return allowMalwareMountScan(ctx, containerID, sourcePath)
					}).WithLogger(diagService)
				})
				if err := schedSvc.AddTask("0 0 4 * * 0", scheduler.NewClamAVSweepTask(rawDocker, scanService, func(ctx context.Context, containerID string) bool {
					return containerAutomationEnabled(ctx, containerID, "security", "malware_sweep_clamav")
				}).WithMountPolicy(func(ctx context.Context, containerID, sourcePath string) bool {
					return allowMalwareMountScan(ctx, containerID, sourcePath)
				}).WithLogger(diagService), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task malware_sweep_clamav: %v", err))
				}

				schedSvc.RegisterTask("clamav_signature_update", func() scheduler.Task {
					return scheduler.NewClamAVSignatureUpdateTask(scanService)
				})
				if err := schedSvc.AddTask("0 30 2 * * *", scheduler.NewClamAVSignatureUpdateTask(scanService), true); err != nil && diagService != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task clamav_signature_update: %v", err))
				}
			}

			// Start remediation listener
			go hrSvc.Start(context.Background())
		}
	}

	// 7. Internal Maintenance: data lifecycle retention pruning
	if diagService != nil {
		schedSvc.RegisterTask("diag_log_prune", func() scheduler.Task {
			return scheduler.NewGenericTask("diag_log_prune", func(ctx context.Context) error {
				st := loadRuntimeSettings(ctx)
				days := retentionDays(st.RetentionLogsDays, 30)
				olderThan := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
				pruned, err := diagService.PruneLogs(ctx, olderThan)
				if err != nil {
					return err
				}
				diagService.Log("INFO", "Scheduler", fmt.Sprintf("Diagnostic log retention prune completed: deleted=%d days=%d", pruned, days))
				return nil
			})
		})
		if err := schedSvc.AddTask("0 0 1 * * *", scheduler.NewGenericTask("diag_log_prune", func(ctx context.Context) error {
			st := loadRuntimeSettings(ctx)
			days := retentionDays(st.RetentionLogsDays, 30)
			olderThan := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
			pruned, err := diagService.PruneLogs(ctx, olderThan)
			if err != nil {
				return err
			}
			diagService.Log("INFO", "Scheduler", fmt.Sprintf("Diagnostic log retention prune completed: deleted=%d days=%d", pruned, days))
			return nil
		}), true); err != nil {
			diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task diag_log_prune: %v", err))
		}

		runHistoryRetentionPrune := func(ctx context.Context) error {
			st := loadRuntimeSettings(ctx)
			now := time.Now().UTC()

			var runErr error
			setErr := func(err error) {
				if err != nil && runErr == nil {
					runErr = err
				}
			}

			if scanStore != nil {
				resultDays := retentionDays(st.RetentionScanResultsDays, 30)
				resultCutoff := now.Add(-time.Duration(resultDays) * 24 * time.Hour).Unix()
				vulnDeleted, err := scanStore.PruneVulnerabilityResults(ctx, resultCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (scan_results): %v", err))
					setErr(err)
				} else {
					malwareDeleted, malErr := scanStore.PruneMalwareResults(ctx, resultCutoff)
					if malErr != nil {
						diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (malware_scan_results): %v", malErr))
						setErr(malErr)
					} else {
						diagService.Log("INFO", "Scheduler", fmt.Sprintf("Scan result retention prune completed: vulnerability=%d malware=%d days=%d", vulnDeleted, malwareDeleted, resultDays))
					}
				}

				jobDays := retentionDays(st.RetentionScanJobsDays, 30)
				jobCutoff := now.Add(-time.Duration(jobDays) * 24 * time.Hour).Unix()
				jobDeleted, err := scanStore.PruneScanJobs(ctx, jobCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (scan_jobs): %v", err))
					setErr(err)
				} else {
					diagService.Log("INFO", "Scheduler", fmt.Sprintf("Scan job retention prune completed: deleted=%d days=%d", jobDeleted, jobDays))
				}
			}

			if updatesStore != nil {
				updateDays := retentionDays(st.RetentionUpdateRunsDays, 90)
				updateCutoff := now.Add(-time.Duration(updateDays) * 24 * time.Hour).Unix()
				runDeleted, stepDeleted, err := updatesStore.PruneRuns(ctx, updateCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (update_runs): %v", err))
					setErr(err)
				} else {
					diagService.Log("INFO", "Scheduler", fmt.Sprintf("Update history retention prune completed: runs=%d steps=%d days=%d", runDeleted, stepDeleted, updateDays))
				}
			}

			if composeAuditStore != nil {
				composeDays := retentionDays(st.RetentionComposeAuditDays, 90)
				composeCutoff := now.Add(-time.Duration(composeDays) * 24 * time.Hour).Unix()
				composeDeleted, err := composeAuditStore.PruneComposeAudits(ctx, composeCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (compose_audit_history): %v", err))
					setErr(err)
				} else {
					diagService.Log("INFO", "Scheduler", fmt.Sprintf("Compose audit retention prune completed: deleted=%d days=%d", composeDeleted, composeDays))
				}
			}

			if aiUsageStore != nil {
				usageDays := retentionDays(st.RetentionAIUsageDays, 180)
				usageCutoff := now.Add(-time.Duration(usageDays) * 24 * time.Hour).Unix()
				usageDeleted, err := aiUsageStore.PruneUsage(ctx, usageCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (ai_usage_events): %v", err))
					setErr(err)
				} else {
					diagService.Log("INFO", "Scheduler", fmt.Sprintf("AI usage retention prune completed: deleted=%d days=%d", usageDeleted, usageDays))
				}
			}

			if hrSvc != nil {
				remedyDays := retentionDays(st.RetentionUpdateRunsDays, 90) // Reuse update runs retention for now
				remedyCutoff := now.Add(-time.Duration(remedyDays) * 24 * time.Hour).Unix()
				remedyDeleted, err := hrSvc.PruneRuns(ctx, remedyCutoff)
				if err != nil {
					diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Retention prune failed (remediation_runs): %v", err))
					setErr(err)
				} else if remedyDeleted > 0 {
					diagService.Log("INFO", "Scheduler", fmt.Sprintf("Remediation history retention prune completed: deleted=%d days=%d", remedyDeleted, remedyDays))
				}
			}

			return runErr
		}

		schedSvc.RegisterTask("history_retention_prune", func() scheduler.Task {
			return scheduler.NewGenericTask("history_retention_prune", runHistoryRetentionPrune)
		})
		if err := schedSvc.AddTask("0 30 1 * * *", scheduler.NewGenericTask("history_retention_prune", runHistoryRetentionPrune), true); err != nil {
			diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to register task history_retention_prune: %v", err))
		}
	}

	// Bootstrap schedules from DB
	if err := schedSvc.LoadSchedules(context.Background()); err != nil && diagService != nil {
		diagService.Log("ERROR", "Scheduler", fmt.Sprintf("Failed to load schedules from DB: %v", err))
	}

	return newMuxWithDepsAndComposeAuditStore(
		db,
		dockerClient,
		scanService,
		releaseService,
		updateService,
		auditService,
		aiService,
		schedSvc,
		metricService,
		diagService,
		notificationService,
		settingsStore,
		portainerService,
		rulesStore,
		intelStore,
		composeAuditStore,
		jobManager,
	), schedSvc, nil
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

type PortainerClient interface {
	GetStack(ctx context.Context, stackID int) (*portainer.Stack, error)
	GetStackFile(ctx context.Context, stackID int) (string, error)
	UpdateStack(ctx context.Context, stackID int, endpointID int, yaml string, env []map[string]string, prune bool, pullImage bool) error
	ListStacks(ctx context.Context) ([]portainer.Stack, error)
}

func NewMuxWithDeps(db *sql.DB, dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService, schedSvc SchedulerService, metricService MetricsService, diagService DiagService, notificationService NotificationService, settingsService SettingsService, portainerService PortainerClient, rulesService RulesService, intelService ContainerIntelService, jobManager *jobs.Manager) http.Handler {
	return newMuxWithDepsAndComposeAuditStore(db, dockerClient, scanService, releaseService, updateService, auditService, aiService, schedSvc, metricService, diagService, notificationService, settingsService, portainerService, rulesService, intelService, nil, jobManager)
}

type updateAIBlockedSignal struct {
	Blocked   bool   `json:"blocked"`
	Reason    string `json:"reason,omitempty"`
	RiskScore int    `json:"riskScore,omitempty"`
	RiskLevel string `json:"riskLevel,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

func extractAIBlockedSignal(run gen.UpdateJobStatus) (updateAIBlockedSignal, bool) {
	reason := ""
	errMsg := strings.TrimSpace(run.Error)
	if strings.Contains(strings.ToLower(errMsg), "ai blocked update:") {
		reason = strings.TrimSpace(strings.TrimPrefix(errMsg, "AI blocked update:"))
	}
	if reason == "" {
		for _, step := range run.Steps {
			if strings.TrimSpace(step.Step) != "release_analysis" {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(step.Status), "failed") {
				continue
			}
			msg := strings.TrimSpace(step.Message)
			if strings.Contains(strings.ToLower(msg), "ai blocked update:") {
				reason = strings.TrimSpace(strings.TrimPrefix(msg, "AI blocked update:"))
				break
			}
		}
	}
	if reason == "" {
		return updateAIBlockedSignal{}, false
	}
	signal := updateAIBlockedSignal{
		Blocked:   true,
		Reason:    reason,
		UpdatedAt: run.UpdatedAt,
	}
	if run.AIAnalysis != nil {
		signal.RiskScore = run.AIAnalysis.RiskScore
		signal.RiskLevel = run.AIAnalysis.RiskLevel
	}
	return signal, true
}

func newMuxWithDepsAndComposeAuditStore(db *sql.DB, dockerClient DockerClient, scanService ScanService, releaseService ReleaseService, updateService UpdateService, auditService AuditService, aiService AIService, schedSvc SchedulerService, metricService MetricsService, diagService DiagService, notificationService NotificationService, settingsService SettingsService, portainerService PortainerClient, rulesService RulesService, intelService ContainerIntelService, composeAuditStore composeAuditHistoryStore, jobManager *jobs.Manager) http.Handler {
	r := chi.NewRouter()
	currentPortainerService := portainerService

	loadContainerSummary := func(ctx context.Context, id string) gen.ContainerSummary {
		summary := gen.ContainerSummary{ID: id}
		if dockerClient == nil {
			return summary
		}
		inspectCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if c, err := dockerClient.GetContainer(inspectCtx, id); err == nil {
			summary = c
		}
		cancel()
		return summary
	}

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

			r.Get("/containers/intel-readiness", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
				defer cancel()
				containers, err := dockerClient.ListContainers(ctx)
				if err != nil {
					writeError(w, http.StatusBadGateway, "docker_error", err.Error())
					return
				}

				overrideMap := map[string]containerintel.Override{}
				if intelService != nil {
					if list, err := intelService.List(ctx); err == nil {
						overrideMap = mapContainerIntelOverrides(list)
					} else if diagService != nil {
						diagService.Log("WARN", "Intel", fmt.Sprintf("Failed to load intelligence overrides for readiness endpoint: %v", err))
					}
				}

				rows := make([]containerIntelResponse, 0, len(containers))
				for _, summary := range containers {
					rows = append(rows, effectiveContainerIntel(summary, overrideForContainerID(summary.ID, overrideMap), currentPortainerService))
				}
				writeJSON(w, http.StatusOK, rows)
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

			r.Get("/images/intelligence", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
				defer cancel()
				images, err := buildImageIntelligence(ctx, dockerClient, scanService)
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
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					if ms, err := scanService.MalwareSummariesForContainer(ctx, id, name); err == nil {
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
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					if ah, err := auditService.ListAuditJobsForContainer(ctx, id, name); err == nil {
						detail.ActionHistory = ah
					} else {
						detail.ActionHistory = []gen.AuditJobSummary{}
					}
				}

				// Enrich with Rules
				if rulesService != nil {
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					if r, err := rulesService.Get(ctx, id, name); err == nil {
						r = effectiveContainerRules(ctx, summary, r, settingsService, diagService)
						detail.Rules = &gen.ContainerRules{
							ContainerID:                   r.ContainerID,
							UpdatePolicy:                  r.UpdatePolicy,
							ValidateURL:                   r.ValidateURL,
							ValidateMode:                  r.ValidateMode,
							ValidateTimeoutSec:            r.ValidateTimeoutSec,
							ValidateIntervalSec:           r.ValidateIntervalSec,
							BypassAI:                      r.BypassAI,
							SkipHealthCheck:               r.SkipHealthCheck,
							AIValidateLogs:                r.AIValidateLogs,
							AutoRollback:                  r.AutoRollback,
							RestartOnUnhealthy:            r.RestartOnUnhealthy,
							UnhealthyRestartCooldownSec:   r.UnhealthyRestartCooldownSec,
							RestartDependentsAfterUpgrade: r.RestartDependentsAfterUpgrade,
							DependentRestartDelaySec:      r.DependentRestartDelaySec,
							InheritAutomation:             r.InheritAutomation,
							UpgradesAutomation:            r.UpgradesAutomation,
							MaintenanceAutomation:         r.MaintenanceAutomation,
							SecurityAutomation:            r.SecurityAutomation,
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

			r.Get("/intel/overrides", func(w http.ResponseWriter, r *http.Request) {
				if intelService == nil {
					writeError(w, http.StatusServiceUnavailable, "intel_unavailable", "Container intelligence store unavailable")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				list, err := intelService.List(ctx)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "intel_list_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, list)
			})

			r.Get("/{id}", getContainerDetail)
			// Backward-compatible alias for older frontend builds that still call /docker/containers/{id}.
			r.Get("/containers/{id}", getContainerDetail)
			r.Get("/{id}/logs", getContainerLogs)
			// Backward-compatible alias for older frontend builds.
			r.Get("/containers/{id}/logs", getContainerLogs)

			r.Post("/{id}/restart", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				id := chi.URLParam(r, "id")
				ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
				defer cancel()
				if err := dockerClient.RestartContainer(ctx, id); err != nil {
					status := http.StatusBadGateway
					if isContainerNotFoundError(err) {
						status = http.StatusNotFound
					}
					writeError(w, status, "restart_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
			})

			registerRulesRoutes := func(router chi.Router) {
				getRules := func(w http.ResponseWriter, r *http.Request) {
					if rulesService == nil {
						writeError(w, http.StatusServiceUnavailable, "rules_unavailable", "Rules service not initialized")
						return
					}
					id := chi.URLParam(r, "id")
					summary := gen.ContainerSummary{ID: id}
					if dockerClient != nil {
						ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
						if c, err := dockerClient.GetContainer(ctx, id); err == nil {
							summary = c
						}
						cancel()
					}
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					res, err := rulesService.Get(r.Context(), id, name)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "rules_get_failed", err.Error())
						return
					}
					res = effectiveContainerRules(r.Context(), summary, res, settingsService, diagService)
					writeJSON(w, http.StatusOK, res)
				}

				saveRules := func(w http.ResponseWriter, r *http.Request) {
					if rulesService == nil {
						writeError(w, http.StatusServiceUnavailable, "rules_unavailable", "Rules service not initialized")
						return
					}
					var raw map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
						writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
						return
					}
					body, err := json.Marshal(raw)
					if err != nil {
						writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
						return
					}
					var incoming rules.ContainerRules
					if err := json.Unmarshal(body, &incoming); err != nil {
						writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
						return
					}
					containerID := chi.URLParam(r, "id")
					summary := gen.ContainerSummary{ID: containerID}
					if dockerClient != nil {
						ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
						if c, err := dockerClient.GetContainer(ctx, containerID); err == nil {
							summary = c
						}
						cancel()
					}
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					req, err := rulesService.Get(r.Context(), containerID, name)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "rules_get_failed", err.Error())
						return
					}
					req.ContainerID = containerID
					if strings.TrimSpace(name) != "" && !strings.EqualFold(name, "unknown") {
						req.ContainerName = name
					}
					if _, ok := raw["containerName"]; ok {
						req.ContainerName = incoming.ContainerName
					}
					if _, ok := raw["updatePolicy"]; ok {
						req.UpdatePolicy = incoming.UpdatePolicy
					}
					if _, ok := raw["validateUrl"]; ok {
						req.ValidateURL = incoming.ValidateURL
					}
					if _, ok := raw["validateMode"]; ok {
						req.ValidateMode = incoming.ValidateMode
					}
					if _, ok := raw["validateTimeoutSec"]; ok {
						req.ValidateTimeoutSec = incoming.ValidateTimeoutSec
					}
					if _, ok := raw["validateIntervalSec"]; ok {
						req.ValidateIntervalSec = incoming.ValidateIntervalSec
					}
					if _, ok := raw["bypassAi"]; ok {
						req.BypassAI = incoming.BypassAI
					}
					if _, ok := raw["skipHealthCheck"]; ok {
						req.SkipHealthCheck = incoming.SkipHealthCheck
					}
					if _, ok := raw["aiValidateLogs"]; ok {
						req.AIValidateLogs = incoming.AIValidateLogs
					}
					if _, ok := raw["autoRollback"]; ok {
						req.AutoRollback = incoming.AutoRollback
					}
					if _, ok := raw["inheritAutomation"]; ok {
						req.InheritAutomation = incoming.InheritAutomation
					}
					if _, ok := raw["upgradesAutomation"]; ok {
						req.UpgradesAutomation = incoming.UpgradesAutomation
					}
					if _, ok := raw["maintenanceAutomation"]; ok {
						req.MaintenanceAutomation = incoming.MaintenanceAutomation
					}
					if _, ok := raw["securityAutomation"]; ok {
						req.SecurityAutomation = incoming.SecurityAutomation
					}
					if _, ok := raw["restartOnUnhealthy"]; ok {
						req.RestartOnUnhealthy = incoming.RestartOnUnhealthy
					}
					if _, ok := raw["unhealthyRestartCooldownSec"]; ok {
						req.UnhealthyRestartCooldownSec = incoming.UnhealthyRestartCooldownSec
					}
					if _, ok := raw["restartDependentsAfterUpgrade"]; ok {
						req.RestartDependentsAfterUpgrade = incoming.RestartDependentsAfterUpgrade
					}
					if _, ok := raw["dependentRestartDelaySec"]; ok {
						req.DependentRestartDelaySec = incoming.DependentRestartDelaySec
					}
					req.UpdatePolicy = normalizeUpdatePolicy(req.UpdatePolicy)
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

			registerIntelRoutes := func(router chi.Router) {
				getIntel := func(w http.ResponseWriter, r *http.Request) {
					if intelService == nil {
						writeError(w, http.StatusServiceUnavailable, "intel_unavailable", "Container intelligence store unavailable")
						return
					}
					id := strings.TrimSpace(chi.URLParam(r, "id"))
					if id == "" {
						writeError(w, http.StatusBadRequest, "invalid_request", "container id is required")
						return
					}
					ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
					defer cancel()
					summary := loadContainerSummary(ctx, id)
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					ov, err := intelService.Get(ctx, id, name)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "intel_get_failed", err.Error())
						return
					}
					writeJSON(w, http.StatusOK, effectiveContainerIntel(summary, ov, currentPortainerService))
				}

				saveIntel := func(w http.ResponseWriter, r *http.Request) {
					if intelService == nil {
						writeError(w, http.StatusServiceUnavailable, "intel_unavailable", "Container intelligence store unavailable")
						return
					}
					id := strings.TrimSpace(chi.URLParam(r, "id"))
					if id == "" {
						writeError(w, http.StatusBadRequest, "invalid_request", "container id is required")
						return
					}
					var req struct {
						RepositoryURL string `json:"repositoryUrl"`
						ChangelogURL  string `json:"changelogUrl"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON")
						return
					}
					ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
					defer cancel()
					summary := loadContainerSummary(ctx, id)
					name := "unknown"
					if len(summary.Names) > 0 {
						name = strings.TrimPrefix(summary.Names[0], "/")
					}
					err := intelService.Save(ctx, containerintel.Override{
						ContainerID:   id,
						ContainerName: name,
						RepositoryURL: req.RepositoryURL,
						ChangelogURL:  req.ChangelogURL,
						UpdatedAt:     time.Now().UTC().Unix(),
					})
					if err != nil {
						writeError(w, http.StatusInternalServerError, "intel_save_failed", err.Error())
						return
					}
					ov, _ := intelService.Get(ctx, id, name)
					writeJSON(w, http.StatusOK, effectiveContainerIntel(summary, ov, currentPortainerService))
				}

				router.Get("/", getIntel)
				router.Post("/", saveIntel)
			}

			r.Route("/{id}/intel", registerIntelRoutes)
			r.Route("/containers/{id}/intel", registerIntelRoutes)
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

		r.Route("/ai", func(r chi.Router) {
			r.Post("/fleet-advice", func(w http.ResponseWriter, r *http.Request) {
				var containers []gen.ContainerSummary
				if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&containers); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
					return
				}

				heuristic := generateFleetAdvice(containers, aiService != nil && aiService.HasProvider())
				advice := heuristic

				if aiService != nil && aiService.HasProvider() {
					// Prepare detailed inventory for AI
					var b strings.Builder
					b.WriteString(heuristic + "\n\nDetailed Container List:\n")
					for _, c := range containers {
						b.WriteString(fmt.Sprintf("- %s (Image: %s, State: %s, Update: %t)\n", trimContainerName(c.Names), c.Image, c.State, c.UpdateAvailable))
					}

					ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
					defer cancel()
					aiAdvice, err := aiService.AnalyzeFleet(ctx, b.String())
					if err == nil {
						advice = aiAdvice
					}
				}
				adviceMarkdown, adviceHTML, err := ai.NormalizeAndRenderMarkdown(advice)
				if err != nil {
					adviceMarkdown = strings.TrimSpace(advice)
					adviceHTML = ""
				}

				// Persist the advice
				if aiService != nil {
					_ = aiService.SaveFleetAdvice(r.Context(), ai.FleetAdviceRecord{
						Timestamp: time.Now().UTC().Unix(),
						Inventory: fmt.Sprintf("%d containers", len(containers)),
						Advice:    adviceMarkdown,
					})
				}

				writeJSON(w, http.StatusOK, map[string]string{
					"advice":         adviceMarkdown,
					"adviceMarkdown": adviceMarkdown,
					"adviceHtml":     adviceHTML,
				})
			})

			r.Get("/fleet-advice", func(w http.ResponseWriter, r *http.Request) {
				if aiService == nil {
					writeJSON(w, http.StatusOK, map[string]string{
						"advice":         "",
						"adviceMarkdown": "",
						"adviceHtml":     "",
					})
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer cancel()
				rec, err := aiService.GetLatestFleetAdvice(ctx)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "fetch_failed", err.Error())
					return
				}
				adviceMarkdown, adviceHTML, renderErr := ai.NormalizeAndRenderMarkdown(rec.Advice)
				if renderErr != nil {
					adviceMarkdown = strings.TrimSpace(rec.Advice)
					adviceHTML = ""
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"advice":         adviceMarkdown,
					"adviceMarkdown": adviceMarkdown,
					"adviceHtml":     adviceHTML,
					"timestamp":      rec.Timestamp,
				})
			})

			r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
				enabled := false
				if aiService != nil {
					enabled = aiService.HasProvider()
				}
				writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
			})

			r.Get("/usage", func(w http.ResponseWriter, r *http.Request) {
				span, window := parseAIUsageSpan(r.URL.Query().Get("span"))
				to := time.Now().UTC().Unix()
				from := to - int64(window.Seconds())
				empty := aiUsageResponse{
					Span:      span,
					From:      from,
					To:        to,
					Breakdown: []aiUsageBreakdownResponse{},
					Daily:     []aiUsageDailyResponse{},
				}
				summarizer, ok := aiService.(aiUsageSummarizer)
				if aiService == nil || !ok {
					writeJSON(w, http.StatusOK, empty)
					return
				}

				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				summary, err := summarizer.UsageSummary(ctx, from, to)
				if err != nil {
					writeError(w, http.StatusBadGateway, "ai_usage_failed", err.Error())
					return
				}

				pricingJSON := ""
				if settingsService != nil {
					if st, err := settingsService.Get(ctx); err == nil {
						pricingJSON = st.AIPricingJSON
					}
				}
				writeJSON(w, http.StatusOK, buildAIUsageResponse(summary, span, pricingJSON))
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

				// Update settings to reflect that testing has passed for this config
				st.AITestingPassed = true
				_ = settingsService.Save(r.Context(), st)

				writeJSON(w, http.StatusOK, map[string]string{
					"status":   "ok",
					"provider": provider,
					"model":    model,
					"summary":  summary,
				})
			})

			r.Get("/conversations", func(w http.ResponseWriter, r *http.Request) {
				if aiService == nil {
					writeJSON(w, http.StatusOK, []ai.ConversationRecord{})
					return
				}
				limit := 50
				if raw := r.URL.Query().Get("limit"); raw != "" {
					if n, err := strconv.Atoi(raw); err == nil && n > 0 {
						limit = n
					}
				}
				if limit > 200 {
					limit = 200
				}
				offset := 0
				if raw := r.URL.Query().Get("offset"); raw != "" {
					if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
						offset = n
					}
				}
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				convs, err := aiService.ListConversations(ctx, limit, offset)
				if err != nil {
					writeError(w, http.StatusBadGateway, "ai_conv_fetch_failed", err.Error())
					return
				}
				if convs == nil {
					convs = []ai.ConversationRecord{}
				}
				writeJSON(w, http.StatusOK, convs)
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
				markdown, rendered, err := ai.NormalizeAndRenderMarkdown(analysis)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "ai_render_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, composeAuditResponse{
					Analysis:         markdown,
					AnalysisMarkdown: markdown,
					AnalysisHTML:     rendered,
					Persisted:        false,
				})
			})

			r.Get("/audit-compose/history/{recordID}", func(w http.ResponseWriter, r *http.Request) {
				if composeAuditStore == nil {
					writeError(w, http.StatusServiceUnavailable, "history_unavailable", "Compose audit history store is not available")
					return
				}
				recordID := strings.TrimSpace(chi.URLParam(r, "recordID"))
				if recordID == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "recordID is required")
					return
				}
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				record, err := composeAuditStore.GetComposeAudit(ctx, recordID)
				if err != nil {
					if errors.Is(err, ai.ErrComposeAuditNotFound) {
						writeError(w, http.StatusNotFound, "not_found", "Compose audit record not found")
						return
					}
					writeError(w, http.StatusBadGateway, "history_fetch_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, record)
			})

			r.Get("/audit-compose/{id}/history", func(w http.ResponseWriter, r *http.Request) {
				if composeAuditStore == nil {
					writeJSON(w, http.StatusOK, []ai.ComposeAuditRecordSummary{})
					return
				}
				containerID := strings.TrimSpace(chi.URLParam(r, "id"))
				if containerID == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "id is required")
					return
				}
				limit := 20
				if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
					if n, err := strconv.Atoi(raw); err == nil {
						if n > 0 {
							limit = n
						}
					}
				}
				if limit > 100 {
					limit = 100
				}
				offset := 0
				if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
					if n, err := strconv.Atoi(raw); err == nil && n > 0 {
						offset = n
					}
				}
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				history, err := composeAuditStore.ListComposeAudits(ctx, containerID, limit, offset)
				if err != nil {
					writeError(w, http.StatusBadGateway, "history_list_failed", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, history)
			})

			r.Get("/audit-compose/{id}/config", func(w http.ResponseWriter, r *http.Request) {
				if dockerClient == nil {
					writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker socket is not available")
					return
				}
				id := chi.URLParam(r, "id")
				scope := normalizeComposeConfigScope(r.URL.Query().Get("scope"))
				ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
				defer cancel()

				summary := gen.ContainerSummary{ID: id}
				if s, err := dockerClient.GetContainer(ctx, id); err == nil {
					summary = s
				}

				var pc *portainer.Client
				if currentPortainerService != nil {
					if c, ok := currentPortainerService.(*portainer.Client); ok {
						pc = c
					}
				}

				config, err := dockerClient.GetContainerComposeConfig(ctx, id, pc)
				if err != nil {
					writeError(w, http.StatusNotFound, "config_not_found", err.Error())
					return
				}

				view := buildComposeConfigView(config, summary, scope)
				writeJSON(w, http.StatusOK, composeConfigPreviewResponse{
					Config:               view.Config,
					ConfigRequestedScope: view.RequestedScope,
					ConfigAppliedScope:   view.AppliedScope,
					ConfigMode:           view.Mode,
					ComposeProject:       view.ComposeProject,
					ComposeService:       view.ComposeService,
					ConfigNote:           view.Note,
				})
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
				scope := normalizeComposeConfigScope(r.URL.Query().Get("scope"))
				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()

				summary := gen.ContainerSummary{ID: id}
				if s, err := dockerClient.GetContainer(ctx, id); err == nil {
					summary = s
				}

				var pc *portainer.Client
				if currentPortainerService != nil {
					if c, ok := currentPortainerService.(*portainer.Client); ok {
						pc = c
					}
				}

				config, err := dockerClient.GetContainerComposeConfig(ctx, id, pc)
				if err != nil {
					writeError(w, http.StatusNotFound, "config_not_found", err.Error())
					return
				}

				view := buildComposeConfigView(config, summary, scope)
				analysis, err := aiService.AuditCompose(ctx, view.Config)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "ai_error", err.Error())
					return
				}
				markdown, rendered, err := ai.NormalizeAndRenderMarkdown(analysis)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "ai_render_error", err.Error())
					return
				}

				provider := "unknown"
				model := "unknown"
				if settingsService != nil {
					if st, err := settingsService.Get(ctx); err == nil {
						provider, model = resolveAuditProviderModel(st)
					}
				}
				containerName := id
				if name := containerDisplayName(summary); name != "" {
					containerName = name
				}

				resp := composeAuditResponse{
					Config:               view.Config,
					ConfigRequestedScope: view.RequestedScope,
					ConfigAppliedScope:   view.AppliedScope,
					ConfigMode:           view.Mode,
					ComposeProject:       view.ComposeProject,
					ComposeService:       view.ComposeService,
					ConfigNote:           view.Note,
					Analysis:             markdown,
					AnalysisMarkdown:     markdown,
					AnalysisHTML:         rendered,
					Provider:             provider,
					Model:                model,
				}
				if composeAuditStore == nil {
					resp.Persisted = false
					resp.PersistError = "history store unavailable"
					writeJSON(w, http.StatusOK, resp)
					return
				}

				record, saveErr := composeAuditStore.SaveComposeAudit(ctx, ai.ComposeAuditRecord{
					ContainerID:      id,
					ContainerName:    containerName,
					Provider:         provider,
					Model:            model,
					ComposeConfig:    view.Config,
					AnalysisMarkdown: markdown,
					AnalysisHTML:     rendered,
				})
				if saveErr != nil {
					resp.Persisted = false
					resp.PersistError = saveErr.Error()
					if diagService != nil {
						diagService.Log("WARN", "AI", "Compose audit persisted failed: "+saveErr.Error())
					}
					writeJSON(w, http.StatusOK, resp)
					return
				}

				resp.RecordID = record.ID
				resp.CreatedAt = record.CreatedAt
				resp.Persisted = true
				writeJSON(w, http.StatusOK, resp)
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

		adminDeps := adminRouteDeps{
			db:                    db,
			dockerClient:          dockerClient,
			scanService:           scanService,
			releaseService:        releaseService,
			updateService:         updateService,
			auditService:          auditService,
			schedSvc:              schedSvc,
			metricService:         metricService,
			diagService:           diagService,
			settingsService:       settingsService,
			notificationService:   notificationService,
			aiService:             aiService,
			rulesService:          rulesService,
			intelService:          intelService,
			jobManager:            jobManager,
			currentPortainerState: &currentPortainerService,
			loadContainerSummary:  loadContainerSummary,
		}
		registerSchedulerRoutes(r, adminDeps)
		registerMetricsRoutes(r, adminDeps)
		registerSystemRoutes(r, adminDeps)
		registerDiagnosticsRoutes(r, adminDeps)
		registerSettingsRoutes(r, adminDeps)
		registerScanRoutes(r, adminDeps)
		registerUpdateRoutes(r, adminDeps)
		registerAuditRoutes(r, adminDeps)
		registerPortainerRoutes(r, adminDeps)
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

func splitDelimitedTokens(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		token := strings.TrimSpace(field)
		if token == "" {
			continue
		}
		out = append(out, token)
	}
	return out
}

func containerMatchesAnyToken(summary gen.ContainerSummary, tokens []string) bool {
	for _, token := range tokens {
		if containerMatchesToken(summary, token) {
			return true
		}
	}
	return false
}

func containerMatchesToken(summary gen.ContainerSummary, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}

	id := strings.ToLower(strings.TrimSpace(summary.ID))
	if id != "" && strings.Contains(id, token) {
		return true
	}

	image := strings.ToLower(strings.TrimSpace(summary.Image))
	if image != "" && (image == token || strings.Contains(image, token)) {
		return true
	}

	for _, name := range summary.Names {
		normalized := strings.ToLower(strings.Trim(strings.TrimSpace(name), "/"))
		if normalized == "" {
			continue
		}
		if normalized == token || strings.Contains(normalized, token) {
			return true
		}
	}

	for _, value := range summary.Labels {
		labelValue := strings.ToLower(strings.TrimSpace(value))
		if labelValue == "" {
			continue
		}
		if labelValue == token || strings.Contains(labelValue, token) {
			return true
		}
	}

	return false
}

func pathMatchesAnyPattern(sourcePath string, patterns []string) bool {
	sourcePath = strings.ToLower(filepath.Clean(strings.TrimSpace(sourcePath)))
	if sourcePath == "" {
		return false
	}
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		patternLower := strings.ToLower(pattern)
		if strings.ContainsAny(patternLower, "*?[]") {
			if ok, _ := filepath.Match(patternLower, sourcePath); ok {
				return true
			}
			if ok, _ := filepath.Match(patternLower, filepath.Base(sourcePath)); ok {
				return true
			}
		}
		cleaned := strings.ToLower(filepath.Clean(patternLower))
		if strings.HasPrefix(cleaned, "/") {
			if sourcePath == cleaned || strings.HasPrefix(sourcePath, cleaned+"/") {
				return true
			}
			continue
		}
		if strings.Contains(sourcePath, cleaned) {
			return true
		}
	}
	return false
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
