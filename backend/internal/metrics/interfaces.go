package metrics

import (
	"context"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type settingsStore interface {
	Get(ctx context.Context) (settings.Settings, error)
}
