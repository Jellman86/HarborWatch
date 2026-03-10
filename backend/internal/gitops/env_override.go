package gitops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func deriveInlineEnvContent(dep GitDeployment) (bool, string, error) {
	if dep.EnvInlineEnabled {
		return true, normalizeEnvContent(dep.EnvInlineContent), nil
	}
	return false, "", nil
}

func legacyEnvVarsJSONToEnvContent(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var envVars map[string]string
	if err := json.Unmarshal([]byte(raw), &envVars); err != nil {
		return "", fmt.Errorf("failed to parse env vars JSON: %w", err)
	}
	return buildEnvFile(envVars)
}

func HydrateDeploymentForResponse(dep *GitDeployment) error {
	if dep == nil {
		return nil
	}
	if dep.EnvInlineEnabled {
		dep.EnvInlineContent = normalizeEnvContent(dep.EnvInlineContent)
		return nil
	}
	if dep.EnvInlineContent == "" && strings.TrimSpace(dep.EnvVarsJSON) != "" {
		content, err := legacyEnvVarsJSONToEnvContent(dep.EnvVarsJSON)
		if err != nil {
			return err
		}
		dep.EnvInlineContent = content
	}
	return nil
}

func managedInlineEnvFilePath(masterDir, sourceID, deploymentID string) (string, error) {
	root := strings.TrimSpace(masterDir)
	if root == "" {
		return "", fmt.Errorf("gitops master directory is empty")
	}
	source := strings.TrimSpace(sourceID)
	deployment := strings.TrimSpace(deploymentID)
	if source == "" || deployment == "" {
		return "", fmt.Errorf("source and deployment identifiers are required")
	}
	return filepath.Join(root, ".harborwatch", "gitops-env", source, deployment+".env"), nil
}

func writeManagedInlineEnvFile(masterDir, sourceID, deploymentID, content string) (string, error) {
	path, err := managedInlineEnvFilePath(masterDir, sourceID, deploymentID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create managed env directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(normalizeEnvContent(content)), 0o600); err != nil {
		return "", fmt.Errorf("write managed env file: %w", err)
	}
	return path, nil
}

func RemoveManagedInlineEnvFile(masterDir, sourceID, deploymentID string) error {
	path, err := managedInlineEnvFilePath(masterDir, sourceID, deploymentID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RemoveManagedInlineEnvSourceDir(masterDir, sourceID string) error {
	root := strings.TrimSpace(masterDir)
	source := strings.TrimSpace(sourceID)
	if root == "" || source == "" {
		return nil
	}
	path := filepath.Join(root, ".harborwatch", "gitops-env", source)
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ResolveDeploymentOverrideEnvFiles(masterDir, repoPath string, dep GitDeployment) ([]string, error) {
	envFiles := make([]string, 0, 2)

	inlineEnabled, inlineContent, err := deriveInlineEnvContent(dep)
	if err != nil {
		return nil, err
	}
	if inlineEnabled {
		path, err := writeManagedInlineEnvFile(masterDir, dep.GitSourceID, dep.ID, inlineContent)
		if err != nil {
			return nil, err
		}
		envFiles = append(envFiles, path)
	} else if dep.EnvFilePath != "" {
		path, err := resolveEnvFilePath(repoPath, dep.EnvFilePath)
		if err != nil {
			return nil, fmt.Errorf("invalid deployment env file path: %w", err)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("env file not found at %s", path)
		}
		envFiles = append(envFiles, path)
	}

	if !dep.EnvInlineEnabled && strings.TrimSpace(dep.EnvVarsJSON) != "" {
		legacyOverride, err := legacyEnvVarsJSONToEnvContent(dep.EnvVarsJSON)
		if err != nil {
			return nil, err
		}
		path, err := writeManagedInlineEnvFile(masterDir, dep.GitSourceID, dep.ID, legacyOverride)
		if err != nil {
			return nil, err
		}
		envFiles = append(envFiles, path)
	} else if !dep.EnvInlineEnabled && strings.TrimSpace(dep.EnvVarsJSON) == "" {
		if path, pathErr := managedInlineEnvFilePath(masterDir, dep.GitSourceID, dep.ID); pathErr == nil {
			_ = os.Remove(path)
		}
	}

	return envFiles, nil
}
