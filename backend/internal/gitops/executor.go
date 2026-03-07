package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/composecli"
)

const maxDeployOutputSummary = 1200

type deployProgressFunc func(progress int, status, message string)

// DeployCompose executes `docker compose up -d --remove-orphans` for a given deployment.
func (s *Service) DeployCompose(ctx context.Context, sourceID string, depID string) error {
	return s.runDeploy(ctx, sourceID, depID, "", nil)
}

func (s *Service) RunTrackedDeploy(ctx context.Context, sourceID string, depID string, jobID string, progress deployProgressFunc) error {
	return s.runDeploy(ctx, sourceID, depID, jobID, progress)
}

func (s *Service) runDeploy(ctx context.Context, sourceID string, depID string, jobID string, progress deployProgressFunc) (resultErr error) {
	var (
		dep           *GitDeployment
		startedAt     int64
		outputSummary string
	)
	src, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to load source: %w", err)
	}
	if err := NormalizeSource(&src); err != nil {
		return fmt.Errorf("invalid git source configuration: %w", err)
	}

	deps, err := s.store.ListDeploymentsForSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to load deployments: %w", err)
	}

	dep = nil
	for _, d := range deps {
		if d.ID == depID {
			dep = &d
			break
		}
	}
	if dep == nil {
		return fmt.Errorf("deployment %s not found in source %s", depID, sourceID)
	}
	if err := NormalizeDeployment(dep); err != nil {
		return fmt.Errorf("invalid deployment configuration: %w", err)
	}
	if !dep.Enabled {
		return fmt.Errorf("deployment %s is disabled", dep.ID)
	}

	st, err := s.settingsStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	if st.GitOpsMasterDirectory == "" {
		return fmt.Errorf("GitOps master directory is not configured")
	}

	// Construct absolute paths while preventing traversal outside the configured root.
	repoPath, err := ResolvePathUnder(st.GitOpsMasterDirectory, src.TargetDir)
	if err != nil {
		return fmt.Errorf("invalid source target directory: %w", err)
	}
	composeFile, err := ResolvePathUnder(repoPath, dep.ComposePath)
	if err != nil {
		return fmt.Errorf("invalid deployment compose path: %w", err)
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("compose file not found at %s", composeFile)
	}

	workDir := filepath.Dir(composeFile)

	startedAt = time.Now().UTC().Unix()
	if jobID != "" {
		if err := s.store.UpdateDeploymentRuntimeStatus(ctx, GitDeployment{
			ID:                  dep.ID,
			LastJobID:           jobID,
			DeployStatus:        "running",
			DeployStatusMessage: "Validating deployment",
			DeployStartedAt:     startedAt,
			DeployFinishedAt:    0,
			DeployOutputSummary: "",
		}); err != nil {
			return fmt.Errorf("persist deploy runtime status: %w", err)
		}
	}
	notifyDeployProgress(progress, 10, "running", "Validating deployment")
	defer func() {
		if jobID == "" || dep == nil {
			return
		}
		status := "completed"
		message := "Deploy completed successfully"
		finishedAt := time.Now().UTC().Unix()
		if resultErr != nil {
			status = "failed"
			message = "Deploy failed"
		}
		if err := s.store.UpdateDeploymentRuntimeStatus(context.Background(), GitDeployment{
			ID:                  dep.ID,
			LastJobID:           jobID,
			DeployStatus:        status,
			DeployStatusMessage: message,
			DeployStartedAt:     startedAt,
			DeployFinishedAt:    finishedAt,
			DeployOutputSummary: outputSummary,
		}); err != nil {
			if resultErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("failed to save deploy runtime status: %w", err))
			} else {
				resultErr = fmt.Errorf("failed to save deploy runtime status: %w", err)
			}
		}
	}()

	// Build the command
	args := []string{"-f", composeFile}

	// Handle env-file mapping and environment overrides.
	inlineEnabled, inlineContent, err := deriveInlineEnvContent(*dep)
	if err != nil {
		resultErr = err
		outputSummary = truncateDeployOutput(err.Error())
		return resultErr
	}

	envFilePath := ""
	if inlineEnabled {
		envFilePath, err = writeManagedInlineEnvFile(st.GitOpsMasterDirectory, sourceID, dep.ID, inlineContent)
		if err != nil {
			resultErr = err
			outputSummary = truncateDeployOutput(err.Error())
			return resultErr
		}
		args = append(args, "--env-file", envFilePath)
	} else if dep.EnvFilePath != "" {
		envFilePath, err = resolveEnvFilePath(repoPath, dep.EnvFilePath)
		if err != nil {
			resultErr = fmt.Errorf("invalid deployment env file path: %w", err)
			outputSummary = truncateDeployOutput(resultErr.Error())
			return resultErr
		}
		if _, err := os.Stat(envFilePath); err != nil {
			resultErr = fmt.Errorf("env file not found at %s", envFilePath)
			outputSummary = truncateDeployOutput(resultErr.Error())
			return resultErr
		}
		args = append(args, "--env-file", envFilePath)
	}

	if !dep.EnvInlineEnabled && strings.TrimSpace(dep.EnvVarsJSON) != "" {
		legacyOverride, err := legacyEnvVarsJSONToEnvContent(dep.EnvVarsJSON)
		if err != nil {
			resultErr = err
			outputSummary = truncateDeployOutput(err.Error())
			return resultErr
		}
		overridePath, err := writeManagedInlineEnvFile(st.GitOpsMasterDirectory, sourceID, dep.ID, legacyOverride)
		if err != nil {
			resultErr = err
			outputSummary = truncateDeployOutput(err.Error())
			return resultErr
		}
		args = append(args, "--env-file", overridePath)
	} else if !dep.EnvInlineEnabled && strings.TrimSpace(dep.EnvVarsJSON) == "" {
		if path, pathErr := managedInlineEnvFilePath(st.GitOpsMasterDirectory, sourceID, dep.ID); pathErr == nil {
			_ = os.Remove(path)
		}
	}
	args = append(args, "up", "-d", "--remove-orphans")
	notifyDeployProgress(progress, 25, "running", "Resolving compose runtime and env sources")

	runner, err := composecli.Resolve(ctx)
	if err != nil {
		resultErr = fmt.Errorf("resolve compose runtime: %w", err)
		outputSummary = truncateDeployOutput(resultErr.Error())
		return resultErr
	}
	cmd := runner.CommandContext(ctx, args...)
	cmd.Dir = workDir

	// We want to capture both stdout and stderr for logging
	notifyDeployProgress(progress, 50, "running", "Running compose apply")
	out, err := cmd.CombinedOutput()
	outputSummary = truncateDeployOutput(string(out))

	errStr := ""
	if err != nil {
		errStr = fmt.Sprintf("deploy failed: %s\nOutput:\n%s", err.Error(), outputSummary)
	}

	// Save the deployment result
	notifyDeployProgress(progress, 90, "running", "Persisting deploy result")
	dbErr := s.store.UpdateDeploymentStatus(ctx, dep.ID, src.LastCommitHash, errStr)
	if dbErr != nil {
		if err != nil {
			err = errors.Join(err, fmt.Errorf("failed to save deployment status: %w", dbErr))
		} else {
			resultErr = fmt.Errorf("failed to save deployment status: %w", dbErr)
			return resultErr
		}
	}

	if err != nil {
		resultErr = errors.New(errStr)
		return resultErr
	}

	notifyDeployProgress(progress, 100, "completed", "Deploy completed successfully")
	return resultErr
}

// DeployAllForSource triggers a deployment for all defined GitDeployments tied to a source.
// This is typically called after a successful `SyncSource` that detects a new commit.
func (s *Service) DeployAllForSource(ctx context.Context, sourceID string) []error {
	deps, err := s.store.ListDeploymentsForSource(ctx, sourceID)
	if err != nil {
		return []error{fmt.Errorf("failed to list deployments: %w", err)}
	}

	var errs []error
	for _, dep := range deps {
		if !dep.Enabled {
			continue
		}
		if err := s.DeployCompose(ctx, sourceID, dep.ID); err != nil {
			errs = append(errs, fmt.Errorf("deployment %s failed: %w", dep.ID, err))
		}
	}

	return errs
}

func resolveEnvFilePath(repoPath, rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("env file path is empty")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return ResolvePathUnder(repoPath, path)
}

func buildEnvFile(envVars map[string]string) (string, error) {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		if strings.ContainsAny(key, "=\n\r") {
			return "", fmt.Errorf("invalid env var key: %q", key)
		}
		value := strings.ReplaceAll(envVars[key], "\r\n", "\n")
		value = strings.ReplaceAll(value, "\r", "\n")
		if strings.Contains(value, "\n") {
			return "", fmt.Errorf("invalid env var value for %q: multiline values are not supported", key)
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(value)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func notifyDeployProgress(progress deployProgressFunc, step int, status, message string) {
	if progress != nil {
		progress(step, status, message)
	}
}

func truncateDeployOutput(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	if len(out) <= maxDeployOutputSummary {
		return out
	}
	return out[:maxDeployOutputSummary-3] + "..."
}
