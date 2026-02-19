package httpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
)

type automatedUpdateApplyTask struct {
	dockerClient          DockerClient
	updateService         UpdateService
	rulesService          RulesService
	settingsService       SettingsService
	intelService          ContainerIntelService
	releaseService        ReleaseService
	diagService           DiagService
	allow                 func(ctx context.Context, containerID string) bool
	refreshUpdates        func(ctx context.Context) error
	defaultMaxPerRun      int
	defaultMinRetryWindow time.Duration
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
	refreshUpdates func(ctx context.Context) error,
) scheduler.Task {
	return &automatedUpdateApplyTask{
		dockerClient:          dockerClient,
		updateService:         updateService,
		rulesService:          rulesService,
		settingsService:       settingsService,
		intelService:          intelService,
		releaseService:        releaseService,
		diagService:           diagService,
		allow:                 allow,
		refreshUpdates:        refreshUpdates,
		defaultMaxPerRun:      envIntWithBounds("HW_AUTO_UPGRADE_MAX_CONCURRENCY", 1, 1, 20),
		defaultMinRetryWindow: time.Duration(envIntWithBounds("HW_AUTO_UPGRADE_MIN_RETRY_MINUTES", 60, 1, 24*60)) * time.Minute,
	}
}

func (t *automatedUpdateApplyTask) Name() string { return "container_update_apply" }

func (t *automatedUpdateApplyTask) Run(ctx context.Context) error {
	if t.dockerClient == nil || t.updateService == nil {
		return nil
	}
	maxPerRun, minRetryWindow := t.loadRunConfig(ctx)
	if t.refreshUpdates != nil {
		refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := t.refreshUpdates(refreshCtx)
		cancel()
		if err != nil {
			t.log("WARN", fmt.Sprintf("Auto-apply preflight update refresh failed: %v", err))
		} else {
			t.log("INFO", "Auto-apply preflight update refresh completed")
		}
	}

	containersCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	containers, err := t.dockerClient.ListContainers(containersCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("list containers for auto-apply: %w", err)
	}

	inspectable := 0
	started := 0
	skipped := 0
	skipReasons := map[string]int{}
	for _, summary := range containers {
		if maxPerRun > 0 && started >= maxPerRun {
			break
		}
		if strings.TrimSpace(summary.ID) == "" || !summary.UpdateAvailable {
			continue
		}
		inspectable++
		if t.allow != nil && !t.allow(ctx, summary.ID) {
			skipped++
			skipReasons["global_exclusion"]++
			t.log("INFO", fmt.Sprintf("Auto-apply skipped for %s: globally excluded from automations", containerLabel(summary)))
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
				if errors.Is(err, ErrUpdatePolicyLocked) {
					skipReasons["policy_locked"]++
				} else {
					skipReasons["policy_manual"]++
				}
				t.log("INFO", fmt.Sprintf("Auto-apply skipped for %s: %v", containerLabel(summary), err))
				continue
			default:
				skipped++
				skipReasons["request_build_failed"]++
				t.log("WARN", fmt.Sprintf("Auto-apply skipped for %s: %v", containerLabel(summary), err))
				continue
			}
		}

		canStart, reason := t.canStartUpdate(ctx, built.Summary.ID, minRetryWindow)
		if !canStart {
			skipped++
			if reason != "" {
				if strings.Contains(strings.ToLower(reason), "already running") {
					skipReasons["already_running"]++
				} else if strings.Contains(strings.ToLower(reason), "retry cooldown") {
					skipReasons["retry_cooldown"]++
				} else {
					skipReasons["gated"]++
				}
				t.log("INFO", fmt.Sprintf("Auto-apply skipped for %s: %s", containerLabel(built.Summary), reason))
			} else {
				skipReasons["gated"]++
			}
			continue
		}

		resp, err := t.updateService.StartUpdate(built.Request)
		if err != nil {
			skipped++
			skipReasons["start_failed"]++
			t.log("ERROR", fmt.Sprintf("Auto-apply failed to start for %s: %v", containerLabel(built.Summary), err))
			continue
		}

		started++
		t.log("INFO", fmt.Sprintf("Auto-apply started for %s (job=%s, image=%s)", containerLabel(built.Summary), resp.JobID, built.Request.TargetImage))
	}

	if inspectable == 0 {
		t.log("INFO", "Auto-apply cycle completed: no update candidates available")
		return nil
	}
	if skipped > 0 {
		t.log("INFO", fmt.Sprintf(
			"Auto-apply cycle completed: candidates=%d started=%d skipped=%d skip_reasons=%s",
			inspectable,
			started,
			skipped,
			formatSkipReasonSummary(skipReasons),
		))
		return nil
	}
	t.log("INFO", fmt.Sprintf("Auto-apply cycle completed: candidates=%d started=%d skipped=%d", inspectable, started, skipped))
	return nil
}

func (t *automatedUpdateApplyTask) canStartUpdate(ctx context.Context, containerID string, minRetryWindow time.Duration) (bool, string) {
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
	if (status == "failed" || status == "rolled_back") && minRetryWindow > 0 {
		lastUpdated := time.Unix(last.UpdatedAt, 0)
		if !lastUpdated.IsZero() && time.Since(lastUpdated) < minRetryWindow {
			remaining := minRetryWindow - time.Since(lastUpdated)
			if remaining < 0 {
				remaining = 0
			}
			return false, fmt.Sprintf("retry cooldown active (%s remaining)", remaining.Round(time.Minute))
		}
	}
	return true, ""
}

func (t *automatedUpdateApplyTask) loadRunConfig(ctx context.Context) (int, time.Duration) {
	maxPerRun := t.defaultMaxPerRun
	minRetryWindow := t.defaultMinRetryWindow
	if t.settingsService == nil {
		return maxPerRun, minRetryWindow
	}
	settingsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	st, err := t.settingsService.Get(settingsCtx)
	if err != nil {
		return maxPerRun, minRetryWindow
	}
	if st.AutoUpgradeMaxConcurrency >= 1 && st.AutoUpgradeMaxConcurrency <= 20 {
		maxPerRun = st.AutoUpgradeMaxConcurrency
	}
	if st.AutoUpgradeMinRetryMinutes >= 1 && st.AutoUpgradeMinRetryMinutes <= 24*60 {
		minRetryWindow = time.Duration(st.AutoUpgradeMinRetryMinutes) * time.Minute
	}
	return maxPerRun, minRetryWindow
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

func formatSkipReasonSummary(reasons map[string]int) string {
	if len(reasons) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(reasons))
	for key, count := range reasons {
		if count <= 0 {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return "none"
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, reasons[key]))
	}
	return strings.Join(parts, ",")
}
