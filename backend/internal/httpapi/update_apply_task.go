package httpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
)

type automatedUpdateApplyTask struct {
	dockerClient    DockerClient
	updateService   UpdateService
	rulesService    RulesService
	settingsService SettingsService
	intelService    ContainerIntelService
	releaseService  ReleaseService
	diagService     DiagService
	allow           func(ctx context.Context, containerID string) bool
	maxPerRun       int
	minRetryWindow  time.Duration
}

func newAutomatedUpdateApplyTask(
	dockerClient DockerClient,
	updateService UpdateService,
	rulesService RulesService,
	settingsService SettingsService,
	intelService ContainerIntelService,
	releaseService ReleaseService,
	diagService DiagService,
	allow func(ctx context.Context, containerID string) bool,
) scheduler.Task {
	return &automatedUpdateApplyTask{
		dockerClient:    dockerClient,
		updateService:   updateService,
		rulesService:    rulesService,
		settingsService: settingsService,
		intelService:    intelService,
		releaseService:  releaseService,
		diagService:     diagService,
		allow:           allow,
		maxPerRun:       envIntWithBounds("HW_AUTO_UPGRADE_MAX_CONCURRENCY", 1, 1, 20),
		minRetryWindow:  time.Duration(envIntWithBounds("HW_AUTO_UPGRADE_MIN_RETRY_MINUTES", 60, 1, 24*60)) * time.Minute,
	}
}

func (t *automatedUpdateApplyTask) Name() string { return "container_update_apply" }

func (t *automatedUpdateApplyTask) Run(ctx context.Context) error {
	if t.dockerClient == nil || t.updateService == nil {
		return nil
	}

	containersCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	containers, err := t.dockerClient.ListContainers(containersCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("list containers for auto-apply: %w", err)
	}

	started := 0
	skipped := 0
	for _, summary := range containers {
		if t.maxPerRun > 0 && started >= t.maxPerRun {
			break
		}
		if strings.TrimSpace(summary.ID) == "" || !summary.UpdateAvailable {
			continue
		}
		if t.allow != nil && !t.allow(ctx, summary.ID) {
			skipped++
			continue
		}

		built, err := buildUpdateRequestForContainer(
			ctx,
			summary.ID,
			"",
			"",
			t.dockerClient,
			t.rulesService,
			t.settingsService,
			t.intelService,
			t.releaseService,
			t.diagService,
			updateRequestBuildOptions{
				RequireAutoPolicy: true,
				EnforceLocked:     true,
			},
		)
		if err != nil {
			switch {
			case errors.Is(err, ErrUpdatePolicyLocked), errors.Is(err, ErrUpdatePolicyNotAuto):
				skipped++
				continue
			default:
				skipped++
				t.log("WARN", fmt.Sprintf("Auto-apply skipped for %s: %v", containerLabel(summary), err))
				continue
			}
		}

		canStart, reason := t.canStartUpdate(ctx, built.Summary.ID)
		if !canStart {
			skipped++
			if reason != "" {
				t.log("INFO", fmt.Sprintf("Auto-apply skipped for %s: %s", containerLabel(built.Summary), reason))
			}
			continue
		}

		resp, err := t.updateService.StartUpdate(built.Request)
		if err != nil {
			skipped++
			t.log("ERROR", fmt.Sprintf("Auto-apply failed to start for %s: %v", containerLabel(built.Summary), err))
			continue
		}

		started++
		t.log("INFO", fmt.Sprintf("Auto-apply started for %s (job=%s, image=%s)", containerLabel(built.Summary), resp.JobID, built.Request.TargetImage))
	}

	t.log("INFO", fmt.Sprintf("Auto-apply cycle completed: started=%d skipped=%d", started, skipped))
	return nil
}

func (t *automatedUpdateApplyTask) canStartUpdate(ctx context.Context, containerID string) (bool, string) {
	historyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	runs, err := t.updateService.ListContainerJobs(historyCtx, containerID, 1)
	if err != nil || len(runs) == 0 {
		return true, ""
	}
	last := runs[0]
	status := strings.ToLower(strings.TrimSpace(last.Status))
	if status == "running" {
		return false, "an update job is already running"
	}
	if (status == "failed" || status == "rolled_back") && t.minRetryWindow > 0 {
		lastUpdated := time.Unix(last.UpdatedAt, 0)
		if !lastUpdated.IsZero() && time.Since(lastUpdated) < t.minRetryWindow {
			remaining := t.minRetryWindow - time.Since(lastUpdated)
			if remaining < 0 {
				remaining = 0
			}
			return false, fmt.Sprintf("retry cooldown active (%s remaining)", remaining.Round(time.Minute))
		}
	}
	return true, ""
}

func (t *automatedUpdateApplyTask) log(level, message string) {
	if t.diagService != nil {
		t.diagService.Log(level, "UpdateAutomation", message)
	}
}

func containerLabel(summary gen.ContainerSummary) string {
	name := strings.TrimSpace(trimContainerName(summary.Names))
	if name != "" {
		return name
	}
	id := strings.TrimSpace(summary.ID)
	if id == "" {
		return "unknown"
	}
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func envIntWithBounds(key string, fallback, minValue, maxValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
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
