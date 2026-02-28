package gitops

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type Service struct {
	store         *Store
	settingsStore *settings.Store
}

func NewService(store *Store, settingsStore *settings.Store) *Service {
	return &Service{
		store:         store,
		settingsStore: settingsStore,
	}
}

// SyncSource triggers an immediate git pull/clone for a specific source.
// Returns the new commit hash, a boolean indicating if it changed, and any error.
func (s *Service) SyncSource(ctx context.Context, sourceID string) (string, bool, error) {
	src, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get source: %w", err)
	}

	st, err := s.settingsStore.Get(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to load settings: %w", err)
	}

	if st.GitOpsMasterDirectory == "" {
		return "", false, fmt.Errorf("GitOps master directory is not configured")
	}

	targetPath := filepath.Join(st.GitOpsMasterDirectory, src.TargetDir)

	newHash, syncErr := SyncRepository(ctx, src.URL, src.Branch, targetPath, src.AuthMethod, src.AuthSecret)

	// Persist the result
	errStr := ""
	if syncErr != nil {
		errStr = syncErr.Error()
	}

	// Update DB
	dbErr := s.store.UpdateSourceSyncStatus(ctx, sourceID, newHash, errStr)
	if dbErr != nil {
		// Log the error but return the sync error if it exists
		if syncErr == nil {
			syncErr = fmt.Errorf("failed to save sync status: %w", dbErr)
		}
	}

	if syncErr != nil {
		return "", false, syncErr
	}

	changed := src.LastCommitHash != newHash
	return newHash, changed, nil
}
