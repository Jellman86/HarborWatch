package gitops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DeployCompose executes `docker compose up -d --remove-orphans` for a given deployment.
func (s *Service) DeployCompose(ctx context.Context, sourceID string, depID string) error {
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

	var dep *GitDeployment
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

	// Build the command
	args := []string{"compose", "-f", composeFile}

	// Handle environment overrides
	envFilePath := ""
	if dep.EnvVarsJSON != "" {
		var envVars map[string]string
		if err := json.Unmarshal([]byte(dep.EnvVarsJSON), &envVars); err != nil {
			return fmt.Errorf("failed to parse env vars JSON: %w", err)
		}

		if len(envVars) > 0 {
			envContent, err := buildEnvFile(envVars)
			if err != nil {
				return err
			}

			envFilePath = filepath.Join(workDir, ".env.harborwatch")
			if err := os.WriteFile(envFilePath, []byte(envContent), 0600); err != nil {
				return fmt.Errorf("failed to write env file: %w", err)
			}
			defer os.Remove(envFilePath) // Clean up afterwards

			args = append(args, "--env-file", envFilePath)
		}
	}

	args = append(args, "up", "-d", "--remove-orphans")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir

	// We want to capture both stdout and stderr for logging
	out, err := cmd.CombinedOutput()

	errStr := ""
	if err != nil {
		errStr = fmt.Sprintf("deploy failed: %s\nOutput:\n%s", err.Error(), string(out))
	}

	// Save the deployment result
	dbErr := s.store.UpdateDeploymentStatus(ctx, dep.ID, src.LastCommitHash, errStr)
	if dbErr != nil {
		if err != nil {
			err = errors.Join(err, fmt.Errorf("failed to save deployment status: %w", dbErr))
		} else {
			return fmt.Errorf("failed to save deployment status: %w", dbErr)
		}
	}

	if err != nil {
		return errors.New(errStr)
	}

	return nil
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
		if err := s.DeployCompose(ctx, sourceID, dep.ID); err != nil {
			errs = append(errs, fmt.Errorf("deployment %s failed: %w", dep.ID, err))
		}
	}

	return errs
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
