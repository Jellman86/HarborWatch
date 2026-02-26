package httpapi

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/composesnapshots"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/scheduler"
)

const composeSnapshotOnChangeTaskID = "compose_snapshot_on_change"

type composeSnapshotSweepSummary struct {
	ProjectsDiscovered int
	ProjectsEligible   int
	SnapshotsCreated   int
	SkippedUnchanged   int
	SkippedUnavailable int
	ProjectErrors      int
}

func (s composeSnapshotSweepSummary) String() string {
	return fmt.Sprintf(
		"projects=%d eligible=%d created=%d skipped_unchanged=%d skipped_unavailable=%d errors=%d",
		s.ProjectsDiscovered,
		s.ProjectsEligible,
		s.SnapshotsCreated,
		s.SkippedUnchanged,
		s.SkippedUnavailable,
		s.ProjectErrors,
	)
}

func composeSnapshotTaskLog(diagService DiagService, level, message string) {
	log.Printf("%s", message)
	if diagService != nil {
		diagService.Log(level, "Scheduler", message)
	}
}

func newComposeSnapshotOnChangeTask(dockerClient DockerClient, settingsService SettingsService, diagService DiagService) scheduler.Task {
	return scheduler.NewGenericTask(composeSnapshotOnChangeTaskID, func(ctx context.Context) error {
		if dockerClient == nil {
			return nil
		}

		snapshotRoot := ""
		if settingsService != nil {
			stCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			st, err := settingsService.Get(stCtx)
			if err != nil {
				composeSnapshotTaskLog(diagService, "WARN", fmt.Sprintf("Compose snapshot sweep: settings read failed, using default snapshot root (%v)", err))
			} else {
				snapshotRoot = strings.TrimSpace(st.ComposeSnapshotRootPath)
			}
		}

		containers, err := dockerClient.ListContainers(ctx)
		if err != nil {
			return fmt.Errorf("compose snapshot sweep list containers: %w", err)
		}

		summary, sweepErr := runComposeSnapshotOnChangeSweep(ctx, containers, snapshotRoot, diagService)
		if sweepErr != nil {
			composeSnapshotTaskLog(diagService, "ERROR", fmt.Sprintf("Compose snapshot on-change sweep completed with errors (%s): %v", summary.String(), sweepErr))
			return sweepErr
		}
		composeSnapshotTaskLog(diagService, "INFO", fmt.Sprintf("Compose snapshot on-change sweep completed (%s)", summary.String()))
		return nil
	})
}

func runComposeSnapshotOnChangeSweep(ctx context.Context, containers []gen.ContainerSummary, snapshotRoot string, diagService DiagService) (composeSnapshotSweepSummary, error) {
	projects := discoverLocalComposeProjectsWithSnapshotRoot(containers, snapshotRoot)
	summary := composeSnapshotSweepSummary{
		ProjectsDiscovered: len(projects),
	}

	var firstErr error
	for _, project := range projects {
		if err := ctx.Err(); err != nil {
			return summary, err
		}

		projectName := strings.TrimSpace(project.ProjectName)
		if projectName == "" || len(project.ConfigFiles) == 0 || !project.SourceVerified {
			summary.SkippedUnavailable++
			continue
		}
		summary.ProjectsEligible++

		if strings.TrimSpace(project.SnapshotError) != "" {
			summary.ProjectErrors++
			err := fmt.Errorf("project %s snapshot status error: %s", projectName, strings.TrimSpace(project.SnapshotError))
			if firstErr == nil {
				firstErr = err
			}
			composeSnapshotTaskLog(diagService, "WARN", fmt.Sprintf("Compose snapshot sweep skipping project=%s reason=%s", projectName, strings.TrimSpace(project.SnapshotError)))
			continue
		}

		status := strings.TrimSpace(project.SnapshotStatus)
		if status == "" {
			status = string(composesnapshots.SnapshotChangeStatusNone)
		}
		if status == string(composesnapshots.SnapshotChangeStatusUnchanged) {
			summary.SkippedUnchanged++
			continue
		}
		if status != string(composesnapshots.SnapshotChangeStatusChanged) && status != string(composesnapshots.SnapshotChangeStatusNone) {
			summary.ProjectErrors++
			err := fmt.Errorf("project %s has unsupported snapshot status %q", projectName, status)
			if firstErr == nil {
				firstErr = err
			}
			composeSnapshotTaskLog(diagService, "WARN", fmt.Sprintf("Compose snapshot sweep skipping project=%s unsupported_status=%s", projectName, status))
			continue
		}

		if _, err := composesnapshots.CreateProjectSnapshot(composesnapshots.CreateSnapshotInput{
			RootDir:      snapshotRoot,
			ProjectName:  project.ProjectName,
			WorkingDir:   project.WorkingDir,
			ConfigFiles:  project.ConfigFiles,
			CreatedAtUTC: time.Now().UTC(),
		}); err != nil {
			summary.ProjectErrors++
			if firstErr == nil {
				firstErr = fmt.Errorf("project %s create snapshot: %w", projectName, err)
			}
			composeSnapshotTaskLog(diagService, "ERROR", fmt.Sprintf("Compose snapshot sweep failed project=%s: %v", projectName, err))
			continue
		}

		summary.SnapshotsCreated++
		composeSnapshotTaskLog(diagService, "INFO", fmt.Sprintf("Compose snapshot created for project=%s mode=on-change", projectName))
	}

	if summary.ProjectErrors > 0 && firstErr != nil {
		return summary, fmt.Errorf("%d compose project snapshot errors (first: %w)", summary.ProjectErrors, firstErr)
	}
	return summary, nil
}
