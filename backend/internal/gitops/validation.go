package gitops

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NormalizeSource trims and validates a Git source payload for safe storage/use.
func NormalizeSource(src *GitSource) error {
	src.Name = strings.TrimSpace(src.Name)
	src.URL = strings.TrimSpace(src.URL)
	src.Branch = strings.TrimSpace(src.Branch)
	src.TargetDir = strings.TrimSpace(src.TargetDir)
	src.AuthSecret = strings.TrimSpace(src.AuthSecret)

	if src.Name == "" || src.URL == "" || src.Branch == "" || src.TargetDir == "" {
		return fmt.Errorf("name, url, branch, and targetDir are required")
	}
	if err := validateRelativePath(src.TargetDir, "targetDir"); err != nil {
		return err
	}

	if src.SyncIntervalMins <= 0 {
		src.SyncIntervalMins = 5
	}

	if src.AuthMethod == "" {
		src.AuthMethod = AuthMethodNone
	}
	switch src.AuthMethod {
	case AuthMethodNone:
		src.AuthSecret = ""
	case AuthMethodHTTPToken, AuthMethodSSHKey:
		if src.AuthSecret == "" {
			return fmt.Errorf("authSecret is required when authMethod is %s", src.AuthMethod)
		}
	default:
		return fmt.Errorf("invalid authMethod: %s", src.AuthMethod)
	}

	return nil
}

// NormalizeDeployment trims and validates a deployment payload for safe storage/use.
func NormalizeDeployment(dep *GitDeployment) error {
	dep.GitSourceID = strings.TrimSpace(dep.GitSourceID)
	dep.ComposePath = strings.TrimSpace(dep.ComposePath)
	dep.EnvVarsJSON = strings.TrimSpace(dep.EnvVarsJSON)
	dep.EnvFilePath = strings.TrimSpace(dep.EnvFilePath)

	if dep.GitSourceID == "" || dep.ComposePath == "" {
		return fmt.Errorf("gitSourceId and composePath are required")
	}
	if err := validateRelativePath(dep.ComposePath, "composePath"); err != nil {
		return err
	}
	if dep.EnvFilePath != "" {
		if filepath.IsAbs(dep.EnvFilePath) {
			cleaned := filepath.Clean(dep.EnvFilePath)
			if cleaned == "." {
				return fmt.Errorf("envFilePath is invalid")
			}
			dep.EnvFilePath = cleaned
		} else if err := validateRelativePath(dep.EnvFilePath, "envFilePath"); err != nil {
			return err
		}
	}
	if dep.ID == "" {
		// New deployment defaults to active; existing records keep their value.
		dep.Enabled = true
	}

	return nil
}

// ResolvePathUnder joins relPath under root and rejects path traversal/absolute paths.
func ResolvePathUnder(root, relPath string) (string, error) {
	base := filepath.Clean(strings.TrimSpace(root))
	if base == "" || base == "." {
		return "", fmt.Errorf("base path is empty")
	}

	rel := strings.TrimSpace(relPath)
	if err := validateRelativePath(rel, "path"); err != nil {
		return "", err
	}

	resolved := filepath.Clean(filepath.Join(base, rel))
	relative, err := filepath.Rel(base, resolved)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}

	return resolved, nil
}

func validateRelativePath(raw string, fieldName string) error {
	path := strings.TrimSpace(raw)
	if path == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("%s must be a relative path", fieldName)
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must stay within the configured root", fieldName)
	}
	return nil
}
