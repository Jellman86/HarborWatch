package updates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	if req.TargetImage == "" {
		return errors.New("targetImage is required")
	}
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return fmt.Errorf("docker socket unavailable: %w", err)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker cli not found: %w", err)
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
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", req.TargetImage)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("image inspect failed: %w (%s)", err, truncate(string(out), 400))
	}
	return nil
}

func (CommandExecutor) Validate(ctx context.Context, req Request) error {
	if req.ValidateURL == "" {
		return errors.New("validateUrl is required")
	}
	hc := &http.Client{Timeout: 8 * time.Second}
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
	// Placeholder rollback hook for future container replacement rollback.
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
