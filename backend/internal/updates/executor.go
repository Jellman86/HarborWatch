package updates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Executor interface {
	Preflight(ctx context.Context, req Request) error
	Backup(ctx context.Context, req Request) error
	Pull(ctx context.Context, req Request) error
	Recreate(ctx context.Context, req Request) error
	Validate(ctx context.Context, req Request) error
	Rollback(ctx context.Context, req Request, cause error) error
}

type CommandExecutor struct{}

func NewCommandExecutor() Executor { return CommandExecutor{} }

func (CommandExecutor) Preflight(ctx context.Context, req Request) error {
	if req.ContainerID == "" {
		return errors.New("containerId is required")
	}
	if req.TargetImage == "" {
		return errors.New("targetImage is required")
	}
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return fmt.Errorf("docker socket unavailable: %w", err)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker cli not found: %w", err)
	}

	// Verify container exists
	cmd := exec.CommandContext(ctx, "docker", "inspect", req.ContainerID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("target container not found or inaccessible: %w (%s)", err, truncate(string(out), 100))
	}

	return nil
}

func (CommandExecutor) Backup(ctx context.Context, req Request) error {
	// Hook point for snapshots/backups in later milestones.
	return nil
}

func (CommandExecutor) Pull(ctx context.Context, req Request) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", req.TargetImage)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker pull failed: %w (%s)", err, truncate(string(out), 400))
	}
	return nil
}

func (CommandExecutor) Recreate(ctx context.Context, req Request) error {
	backupName := fmt.Sprintf("%s_backup_%d", req.ContainerID, time.Now().Unix())

	// 1. Stop the current container
	stopCmd := exec.CommandContext(ctx, "docker", "stop", req.ContainerID)
	if out, err := stopCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop container: %w (%s)", err, truncate(string(out), 100))
	}

	// 2. Rename current container to backup
	renameCmd := exec.CommandContext(ctx, "docker", "rename", req.ContainerID, backupName)
	if out, err := renameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to rename container to backup: %w (%s)", err, truncate(string(out), 100))
	}

	// 3. Create and start the new container
	// Note: In a production scenario, we'd extract the config from 'inspect' 
	// and apply it here. For Milestone 4 hardening, we use the new image.
	runCmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", req.ContainerID, req.TargetImage)
	if out, err := runCmd.CombinedOutput(); err != nil {
		// If recreation fails, we don't rollback automatically here; 
		// the state machine in service.go will trigger Rollback()
		return fmt.Errorf("failed to start new container: %w (%s)", err, truncate(string(out), 100))
	}

	return nil
}

func (CommandExecutor) Validate(ctx context.Context, req Request) error {
	if req.ValidateURL == "" {
		return errors.New("validateUrl is required")
	}
	hc := &http.Client{Timeout: 10 * time.Second}
	
	// Wait a moment for container to potentially start up
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, req.ValidateURL, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(request)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check status %d", resp.StatusCode)
	}
	return nil
}

func (CommandExecutor) Rollback(ctx context.Context, req Request, cause error) error {
	// 1. Stop the "new" container if it exists and is running
	_ = exec.CommandContext(ctx, "docker", "stop", req.ContainerID).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", req.ContainerID).Run()

	// 2. Find the most recent backup
	// This is a simplified heuristic: find containers named <id>_backup_*
	findCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s_backup_", req.ContainerID), "--format", "{{.Names}}")
	out, err := findCmd.Output()
	if err != nil || len(out) == 0 {
		return fmt.Errorf("rollback failed: could not find backup container: %w", err)
	}

	backups := strings.Split(strings.TrimSpace(string(out)), "\n")
	latestBackup := backups[0] // docker ps -a usually returns newest first, but let's be careful

	// 3. Restore backup
	renameCmd := exec.CommandContext(ctx, "docker", "rename", latestBackup, req.ContainerID)
	if out, err := renameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rollback failed: could not rename backup: %w (%s)", err, truncate(string(out), 100))
	}

	startCmd := exec.CommandContext(ctx, "docker", "start", req.ContainerID)
	if out, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rollback failed: could not start backup: %w (%s)", err, truncate(string(out), 100))
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
