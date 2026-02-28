package gitops

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type SettingsProvider interface {
	Get(ctx context.Context) (settings.Settings, error)
}

type Service struct {
	store         *Store
	settingsStore SettingsProvider
}

func NewService(store *Store, settingsStore SettingsProvider) *Service {
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
	if err := NormalizeSource(&src); err != nil {
		return "", false, fmt.Errorf("invalid git source configuration: %w", err)
	}

	st, err := s.settingsStore.Get(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to load settings: %w", err)
	}

	if st.GitOpsMasterDirectory == "" {
		return "", false, fmt.Errorf("GitOps master directory is not configured")
	}

	targetPath, err := ResolvePathUnder(st.GitOpsMasterDirectory, src.TargetDir)
	if err != nil {
		return "", false, fmt.Errorf("invalid target path: %w", err)
	}

	newHash, syncErr := SyncRepository(ctx, src.URL, src.Branch, targetPath, src.AuthMethod, src.AuthSecret)

	// Persist the result
	errStr := ""
	if syncErr != nil {
		errStr = syncErr.Error()
	}

	// Update DB
	dbErr := s.store.UpdateSourceSyncStatus(ctx, sourceID, newHash, errStr)
	if dbErr != nil {
		if syncErr != nil {
			syncErr = errors.Join(syncErr, fmt.Errorf("failed to save sync status: %w", dbErr))
		} else {
			syncErr = fmt.Errorf("failed to save sync status: %w", dbErr)
		}
	}

	if syncErr != nil {
		return "", false, syncErr
	}

	changed := src.LastCommitHash != newHash
	return newHash, changed, nil
}
