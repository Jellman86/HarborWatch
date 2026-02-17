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

	created, err := cli.ContainerCreate(ctx, newConfig, hostConfig, networkingConfig, nil, req.ContainerID)
	if err != nil {
		return fmt.Errorf("failed to create replacement container: %w", err)
	}
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start replacement container: %w", err)
	}

	return nil
}

func (CommandExecutor) Validate(ctx context.Context, req Request) error {
	if req.ValidateURL == "" {
		return errors.New("validateUrl is required")
	}

	hc := &http.Client{Timeout: 5 * time.Second}

	// Retry loop for up to 30 seconds
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, req.ValidateURL, nil)
		if err != nil {
			return err
		}

		resp, err := hc.Do(request)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				return nil // Success!
			}
			lastErr = fmt.Errorf("health check status %d", resp.StatusCode)
			resp.Body.Close()
		} else {
			lastErr = err
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("validation timed out: %w", lastErr)
}

func (CommandExecutor) Rollback(ctx context.Context, req Request, cause error) error {
	// 1. Stop and Remove the "new" container if it exists
	_ = exec.CommandContext(ctx, "docker", "stop", req.ContainerID).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", req.ContainerID).Run()

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
