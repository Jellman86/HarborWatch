package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/composeexec"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/gitops"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type GitOpsLookup interface {
	ListSources(ctx context.Context) ([]gitops.GitSource, error)
	ListDeploymentsForSource(ctx context.Context, sourceID string) ([]gitops.GitDeployment, error)
}

func resolveComposeExecutionContextForSummary(ctx context.Context, summary gen.ContainerSummary, st settings.Settings, gitOpsLookup GitOpsLookup) (composeexec.Context, error) {
	_, _, workingDir, configFiles, ok := localComposeProjectMetadata(summary)
	if !ok {
		return composeexec.Context{}, nil
	}

	envFiles := make([]string, 0, 3)
	if envPath, _, _ := detectEnvAuthority(workingDir); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			envFiles = append(envFiles, envPath)
		}
	}

	overrideEnvFiles, err := resolveGitOpsOverrideEnvFiles(ctx, workingDir, configFiles, st.GitOpsMasterDirectory, gitOpsLookup)
	if err != nil {
		return composeexec.Context{}, err
	}
	envFiles = append(envFiles, overrideEnvFiles...)

	return composeexec.New(workingDir, configFiles, envFiles), nil
}

func resolveGitOpsOverrideEnvFiles(ctx context.Context, workingDir string, configFiles []string, gitOpsRoot string, gitOpsLookup GitOpsLookup) ([]string, error) {
	root := strings.TrimSpace(gitOpsRoot)
	if root == "" || gitOpsLookup == nil || !isGitOpsBackedComposeSource(workingDir, configFiles, root) {
		return nil, nil
	}

	sources, err := gitOpsLookup.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	for _, src := range sources {
		repoPath, err := gitops.ResolvePathUnder(root, src.TargetDir)
		if err != nil {
			continue
		}
		deps, err := gitOpsLookup.ListDeploymentsForSource(ctx, src.ID)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			composePath, err := gitops.ResolvePathUnder(repoPath, dep.ComposePath)
			if err != nil {
				continue
			}
			if !pathMatchesAny(composePath, configFiles) {
				continue
			}
			return gitops.ResolveDeploymentOverrideEnvFiles(root, repoPath, dep)
		}
	}
	return nil, nil
}

func pathMatchesAny(path string, candidates []string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	for _, raw := range candidates {
		if filepath.Clean(strings.TrimSpace(raw)) == cleanPath {
			return true
		}
	}
	return false
}
