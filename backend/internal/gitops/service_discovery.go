package gitops

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// ListSourceFiles returns compose and env files from the synced repository directory.
func (s *Service) ListSourceFiles(ctx context.Context, sourceID string) (RepositoryFiles, error) {
	src, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return RepositoryFiles{}, fmt.Errorf("failed to get source: %w", err)
	}
	if err := NormalizeSource(&src); err != nil {
		return RepositoryFiles{}, fmt.Errorf("invalid git source configuration: %w", err)
	}

	st, err := s.settingsStore.Get(ctx)
	if err != nil {
		return RepositoryFiles{}, fmt.Errorf("failed to load settings: %w", err)
	}
	if st.GitOpsMasterDirectory == "" {
		return RepositoryFiles{}, fmt.Errorf("GitOps master directory is not configured")
	}

	repoPath, err := ResolvePathUnder(st.GitOpsMasterDirectory, src.TargetDir)
	if err != nil {
		return RepositoryFiles{}, fmt.Errorf("invalid target path: %w", err)
	}

	files, err := DiscoverRepositoryFiles(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return RepositoryFiles{}, nil
		}
		return RepositoryFiles{}, fmt.Errorf("failed to discover repository files: %w", err)
	}
	return files, nil
}

// AutoCreateDeploymentRules discovers compose files and creates missing deployment rules as disabled.
func (s *Service) AutoCreateDeploymentRules(ctx context.Context, sourceID string) ([]GitDeployment, error) {
	files, err := s.ListSourceFiles(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if len(files.ComposeFiles) == 0 {
		return nil, nil
	}

	created := make([]GitDeployment, 0, len(files.ComposeFiles))
	for _, composePath := range files.ComposeFiles {
		dep := GitDeployment{
			ID:          uuid.NewString(),
			GitSourceID: sourceID,
			ComposePath: composePath,
			AutoCreated: true,
			Enabled:     false,
		}
		if err := NormalizeDeployment(&dep); err != nil {
			return nil, fmt.Errorf("invalid auto-generated deployment for %q: %w", composePath, err)
		}
		ok, err := s.store.CreateDeploymentIfMissing(ctx, dep)
		if err != nil {
			return nil, fmt.Errorf("failed to create deployment for %q: %w", composePath, err)
		}
		if ok {
			created = append(created, dep)
		}
	}

	return created, nil
}
