package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/diag"
	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
	"github.com/go-chi/chi/v5/middleware"
)

type diagnosticsDeps struct {
	dockerClient  DockerClient
	scanService   ScanService
	updateService UpdateService
	auditService  AuditService
	schedSvc      SchedulerService
	diagService   DiagService
}

type diagnosticsSnapshotOptions struct {
	LogLimit         int
	AuditLimit       int
	IncludeFleet     bool
	ContainerID      string
	ContainerLogTail int
	ContainerSince   string
}

type diagnosticsFleetStats struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

type diagnosticsSnapshot struct {
	GeneratedAt        int64                       `json:"generatedAt"`
	Components         map[string]string           `json:"components"`
	Errors             []string                    `json:"errors,omitempty"`
	SystemStatus       *diag.SystemStatus          `json:"systemStatus,omitempty"`
	InternalLogs       []diag.LogEntry             `json:"internalLogs,omitempty"`
	ActiveJobs         []gen.JobProgress           `json:"activeJobs,omitempty"`
	RecentAuditJobs    []gen.AuditJobSummary       `json:"recentAuditJobs,omitempty"`
	FailedAuditJobs    []gen.AuditJobSummary       `json:"failedAuditJobs,omitempty"`
	Schedules          []scheduler.ScheduleEntry   `json:"schedules,omitempty"`
	LatestScanSummary  *gen.ScanSummary            `json:"latestScanSummary,omitempty"`
	FleetStats         *diagnosticsFleetStats      `json:"fleetStats,omitempty"`
	FleetContainers    []gen.ContainerSummary      `json:"fleetContainers,omitempty"`
	FleetImages        []gen.ImageSummary          `json:"fleetImages,omitempty"`
	TargetContainer    *gen.ContainerSummary       `json:"targetContainer,omitempty"`
	TargetContainerLog *dockerengine.ContainerLogs `json:"targetContainerLogs,omitempty"`
}

func diagnosticsHTTPErrorLogger(diagService DiagService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if diagService == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			if status < 400 {
				return
			}

			level := "WARN"
			if status >= 500 {
				level = "ERROR"
			}

			reqID := middleware.GetReqID(r.Context())
			diagService.Log(level, "HTTP", fmt.Sprintf(
				"%s %s -> %d (%dms) req_id=%s remote=%s ua=%q",
				r.Method,
				r.URL.Path,
				status,
				time.Since(start).Milliseconds(),
				reqID,
				r.RemoteAddr,
				truncateString(r.UserAgent(), 128),
			))
		})
	}
}

func collectDiagnosticsSnapshot(ctx context.Context, deps diagnosticsDeps, opts diagnosticsSnapshotOptions) (diagnosticsSnapshot, error) {
	snapshot := diagnosticsSnapshot{
		GeneratedAt: time.Now().Unix(),
		Components: map[string]string{
			"docker":    componentStatus(deps.dockerClient != nil),
			"scanner":   componentStatus(deps.scanService != nil),
			"audit":     componentStatus(deps.auditService != nil),
			"scheduler": componentStatus(deps.schedSvc != nil),
			"diag":      componentStatus(deps.diagService != nil),
		},
	}

	if deps.diagService != nil {
		status := deps.diagService.GetSystemStatus()
		snapshot.SystemStatus = &status

		logs, err := deps.diagService.ListLogs(ctx, opts.LogLimit, 0, "", "", "", 0)
		if err != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("diag logs: %v", err))
		} else {
			snapshot.InternalLogs = logs
		}
	}

	activeJobs := []gen.JobProgress{}
	if deps.scanService != nil {
		activeJobs = append(activeJobs, deps.scanService.ActiveJobs()...)
	}
	if deps.updateService != nil {
		activeJobs = append(activeJobs, deps.updateService.ActiveJobs()...)
	}
	snapshot.ActiveJobs = activeJobs

	if deps.auditService != nil {
		jobs, err := deps.auditService.ListAuditJobs(ctx)
		if err != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("audit jobs: %v", err))
		} else {
			if len(jobs) > opts.AuditLimit {
				jobs = jobs[:opts.AuditLimit]
			}
			snapshot.RecentAuditJobs = jobs
			failed := make([]gen.AuditJobSummary, 0, len(jobs))
			for _, job := range jobs {
				if strings.EqualFold(job.Status, "failed") || strings.EqualFold(job.Status, "rolled_back") || strings.TrimSpace(job.Error) != "" {
					failed = append(failed, job)
				}
			}
			snapshot.FailedAuditJobs = failed
		}
	}

	if deps.schedSvc != nil {
		schedules, err := deps.schedSvc.ListSchedules(ctx)
		if err != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("scheduler: %v", err))
		} else {
			snapshot.Schedules = schedules
		}
	}

	if deps.scanService != nil {
		summary, err := deps.scanService.LatestSummary(ctx)
		if err != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("scan summary: %v", err))
		} else {
			snapshot.LatestScanSummary = summary
		}
	}

	if deps.dockerClient != nil {
		dockerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		containers, cErr := deps.dockerClient.ListContainers(dockerCtx)
		images, iErr := deps.dockerClient.ListImages(dockerCtx)
		cancel()

		if cErr != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("docker containers: %v", cErr))
		} else {
			stats := diagnosticsFleetStats{Total: len(containers)}
			for _, c := range containers {
				if strings.EqualFold(c.State, "running") {
					stats.Running++
				} else {
					stats.Stopped++
				}
			}
			snapshot.FleetStats = &stats
			if opts.IncludeFleet {
				snapshot.FleetContainers = containers
			}
		}
		if iErr != nil {
			snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("docker images: %v", iErr))
		} else if opts.IncludeFleet {
			snapshot.FleetImages = images
		}

		if opts.ContainerID != "" {
			containerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			container, err := deps.dockerClient.GetContainer(containerCtx, opts.ContainerID)
			if err != nil {
				snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("target container: %v", err))
			} else {
				snapshot.TargetContainer = &container
			}

			since, err := parseSinceQuery(opts.ContainerSince, 6*time.Hour)
			if err != nil {
				cancel()
				return diagnosticsSnapshot{}, fmt.Errorf("invalid containerLogSince: %w", err)
			}
			logs, err := deps.dockerClient.GetContainerLogs(containerCtx, opts.ContainerID, opts.ContainerLogTail, since, true)
			cancel()
			if err != nil {
				snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("target container logs: %v", err))
			} else {
				snapshot.TargetContainerLog = &logs
			}
		}
	}

	return snapshot, nil
}

func componentStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func isContainerNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") || strings.Contains(msg, "status=404") || strings.Contains(msg, "container not found")
}

func parseIntQuery(raw string, fallback, minValue, maxValue int) int {
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}

func parseBoolQuery(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseSinceQuery(raw string, fallback time.Duration) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Now().Add(-fallback), nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return time.Time{}, err
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("duration must be greater than zero")
	}
	return time.Now().Add(-d), nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
