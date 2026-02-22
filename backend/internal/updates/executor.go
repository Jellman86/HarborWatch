package updates

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/moby/moby/client"
	"os/exec"
)

type Executor interface {
	Preflight(ctx context.Context, req Request) error
	Backup(ctx context.Context, req Request) error
	Pull(ctx context.Context, req Request) error
	Recreate(ctx context.Context, req Request) error
	Validate(ctx context.Context, req Request) error
	Cleanup(ctx context.Context, req Request) error
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
	liveRef := liveContainerRef(req)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, req.ContainerID)
	if err != nil {
		return fmt.Errorf("failed to inspect existing container: %w", err)
	}
	if inspect.Config == nil {
		return errors.New("container inspect missing config")
	}

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

	// 3. Create and start a replacement container while preserving runtime configuration.
	newConfig := inspect.Config
	newConfig.Image = req.TargetImage

	hostConfig := inspect.HostConfig
	if hostConfig == nil {
		hostConfig = &container.HostConfig{}
	}

	networkingConfig := &network.NetworkingConfig{}
	if inspect.NetworkSettings != nil && len(inspect.NetworkSettings.Networks) > 0 {
		endpoints := make(map[string]*network.EndpointSettings, len(inspect.NetworkSettings.Networks))
		for name, endpoint := range inspect.NetworkSettings.Networks {
			endpoints[name] = endpoint
		}
		networkingConfig.EndpointsConfig = endpoints
	}

	created, err := cli.ContainerCreate(ctx, newConfig, hostConfig, networkingConfig, nil, liveRef)
	if err != nil {
		return fmt.Errorf("failed to create replacement container: %w", err)
	}
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start replacement container: %w", err)
	}

	return nil
}

func (CommandExecutor) Validate(ctx context.Context, req Request) error {
	mode := normalizeValidateMode(req.ValidateMode)
	timeout := validationTimeout(req.ValidateTimeoutSec)
	interval := validationInterval(req.ValidateIntervalSec)
	if (mode == "http" || mode == "both") && strings.TrimSpace(req.ValidateURL) == "" {
		return errors.New("validateUrl is required for http or both validation mode")
	}

	hc := &http.Client{Timeout: 5 * time.Second}
	var dockerClient *client.Client
	if mode == "docker" || mode == "both" {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return fmt.Errorf("init docker client for validation: %w", err)
		}
		dockerClient = cli
		defer dockerClient.Close()
	}

	deadline := time.Now().Add(timeout)
	currentInterval := interval
	maxInterval := 30 * time.Second
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		httpOK := mode == "docker"
		dockerOK := mode == "http"

		if mode == "http" || mode == "both" {
			ok, err := validateHTTP(ctx, hc, req.ValidateURL)
			if err != nil {
				lastErr = err
			}
			httpOK = ok
		}
		if mode == "docker" || mode == "both" {
			ok, err := validateDockerState(ctx, dockerClient, liveContainerRef(req))
			if err != nil {
				lastErr = err
			}
			dockerOK = ok
		}

		if httpOK && dockerOK {
			return nil
		}

		// Wait with current interval, then backoff.
		timer := time.NewTimer(currentInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Exponential backoff
			currentInterval *= 2
			if currentInterval > maxInterval {
				currentInterval = maxInterval
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("health checks did not reach healthy state before timeout")
	}
	return fmt.Errorf("validation timed out: %w", lastErr)
}

func (CommandExecutor) Cleanup(ctx context.Context, req Request) error {
	// Find all backups for this container
	findCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s_backup_", req.ContainerID), "--format", "{{.Names}}")
	out, err := findCmd.Output()
	if err != nil {
		return nil // Ignore if we can't find them or command fails
	}
	backups := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, b := range backups {
		name := strings.TrimSpace(b)
		if name != "" {
			// Silently remove backup containers
			_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
		}
	}
	return nil
}

func (CommandExecutor) Rollback(ctx context.Context, req Request, cause error) error {
	liveRef := liveContainerRef(req)
	// 1. Stop and Remove the "new" container if it exists
	_ = exec.CommandContext(ctx, "docker", "stop", liveRef).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", liveRef).Run()

	// 2. Find the most recent backup
	findCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s_backup_", req.ContainerID), "--format", "{{.Names}}")
	out, err := findCmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return fmt.Errorf("rollback failed: could not find backup container: %w", err)
	}

	backups := strings.Split(strings.TrimSpace(string(out)), "\n")
	latestBackup, err := newestBackupName(backups, req.ContainerID)
	if err != nil {
		return fmt.Errorf("rollback failed: could not determine latest backup: %w", err)
	}

	// 3. Restore backup
	renameCmd := exec.CommandContext(ctx, "docker", "rename", latestBackup, liveRef)
	if out, err := renameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rollback failed: could not rename backup: %w (%s)", err, truncate(string(out), 100))
	}

	startCmd := exec.CommandContext(ctx, "docker", "start", req.ContainerID)
	if out, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rollback failed: could not start backup: %w (%s)", err, truncate(string(out), 100))
	}

	return nil
}

func liveContainerRef(req Request) string {
	if name := strings.TrimSpace(req.ContainerName); name != "" {
		return name
	}
	return strings.TrimSpace(req.ContainerID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func newestBackupName(backups []string, containerID string) (string, error) {
	prefix := containerID + "_backup_"
	latestTS := int64(-1)
	latestName := ""
	fallback := make([]string, 0, len(backups))

	for _, raw := range backups {
		name := strings.TrimSpace(raw)
		if name == "" || !strings.HasPrefix(name, prefix) {
			continue
		}

		tsPart := strings.TrimPrefix(name, prefix)
		ts, err := strconv.ParseInt(tsPart, 10, 64)
		if err != nil {
			fallback = append(fallback, name)
			continue
		}

		if ts > latestTS {
			latestTS = ts
			latestName = name
		}
	}

	if latestName != "" {
		return latestName, nil
	}

	if len(fallback) > 0 {
		sort.Strings(fallback)
		return fallback[len(fallback)-1], nil
	}

	return "", errors.New("no valid backup names found")
}

func normalizeValidateMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "http":
		return "http"
	case "docker":
		return "docker"
	case "both":
		return "both"
	default:
		return "both"
	}
}

func validationTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 45 * time.Second
	}
	if seconds > 600 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func validationInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return 2 * time.Second
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func validateHTTP(ctx context.Context, hc *http.Client, validateURL string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := hc.Do(request)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}
	return false, fmt.Errorf("http health check status %d", resp.StatusCode)
}

func validateDockerState(ctx context.Context, cli *client.Client, containerID string) (bool, error) {
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	if inspect.State == nil {
		return false, errors.New("container state is unavailable")
	}
	// If Docker healthcheck exists, require "healthy".
	if inspect.State.Health != nil {
		status := strings.ToLower(strings.TrimSpace(inspect.State.Health.Status))
		switch status {
		case "healthy":
			return true, nil
		case "unhealthy":
			return false, errors.New("docker healthcheck reports unhealthy")
		default:
			return false, fmt.Errorf("docker healthcheck status=%s", status)
		}
	}
	// Fallback when no explicit Docker healthcheck is configured.
	if inspect.State.Running {
		return true, nil
	}
	return false, fmt.Errorf("container is not running (status=%s)", inspect.State.Status)
}
