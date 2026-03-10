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

type composeExecutionContext struct {
	composeexec.Context
	ManagedEnvContent string
}

func resolveComposeExecutionContextForSummary(ctx context.Context, summary gen.ContainerSummary, st settings.Settings, gitOpsLookup GitOpsLookup) (composeExecutionContext, error) {
	_, _, workingDir, configFiles, ok := localComposeProjectMetadata(summary)
	if !ok {
		return composeExecutionContext{}, nil
	}

	envFiles := make([]string, 0, 3)
	if envPath, _, _ := detectEnvAuthority(workingDir); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			envFiles = append(envFiles, envPath)
		}
	}

	managedEnvContent, overrideEnvFiles, err := resolveGitOpsOverrideEnvFiles(ctx, workingDir, configFiles, st.GitOpsMasterDirectory, gitOpsLookup)
	if err != nil {
		return composeExecutionContext{}, err
	}
	envFiles = append(envFiles, overrideEnvFiles...)

	return composeExecutionContext{
		Context:           composeexec.New(workingDir, configFiles, envFiles),
		ManagedEnvContent: managedEnvContent,
	}, nil
}

func resolveGitOpsOverrideEnvFiles(ctx context.Context, workingDir string, configFiles []string, gitOpsRoot string, gitOpsLookup GitOpsLookup) (string, []string, error) {
	root := strings.TrimSpace(gitOpsRoot)
	if root == "" || gitOpsLookup == nil || !isGitOpsBackedComposeSource(workingDir, configFiles, root) {
		return "", nil, nil
	}

	sources, err := gitOpsLookup.ListSources(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, src := range sources {
		repoPath, err := gitops.ResolvePathUnder(root, src.TargetDir)
		if err != nil {
			continue
		}
		deps, err := gitOpsLookup.ListDeploymentsForSource(ctx, src.ID)
		if err != nil {
			return "", nil, err
		}
		for _, dep := range deps {
			if !dep.Enabled {
				continue
			}
			composePath, err := gitops.ResolvePathUnder(repoPath, dep.ComposePath)
			if err != nil {
				continue
			}
			if !pathMatchesAny(composePath, configFiles) {
				continue
			}
			managedEnvContent, envFiles, err := gitops.ResolveDeploymentOverrideEnvContent(repoPath, dep)
			if err != nil {
				return "", nil, err
			}
			return managedEnvContent, envFiles, nil
		}
	}
	return "", nil, nil
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
