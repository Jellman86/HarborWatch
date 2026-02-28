package metrics

import (
	"context"
	"time"

	"github.com/moby/moby/client"
)

type Service struct {
	store     *Store
	collector *Collector
}

func NewService(store *Store, docker *client.Client, settings settingsStore) *Service {
	return &Service{
		store:     store,
		collector: NewCollector(docker, store, settings),
	}
}

func (s *Service) GetCollectorTask() *Collector {
	return s.collector
}

func (s *Service) GetMetrics(ctx context.Context, containerID string, duration string) ([]Metric, error) {
	// Default to 24 hours
	since := time.Now().Add(-24 * time.Hour).Unix()

	if d, err := time.ParseDuration(duration); err == nil {
		since = time.Now().Add(-d).Unix()
	}

	items, err := s.store.GetMetrics(ctx, containerID, since)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].CPUPercent = NormalizeCPUPercent(items[i].CPUPercent)
	}
	return items, nil
}

// PruneTask cleans up metrics based on retention settings
type PruneTask struct {
	store    *Store
	settings settingsStore
}

func (t *PruneTask) Name() string { return "metrics_prune" }
func (t *PruneTask) Run(ctx context.Context) error {
	days := 14
	if t.settings != nil {
		if st, err := t.settings.Get(ctx); err == nil {
			days = st.RetentionMetricsDays
		}
	}
	if days <= 0 {
		days = 14
	}

	olderThan := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	_, err := t.store.PruneMetrics(ctx, olderThan)
	return err
}

func (s *Service) GetPruneTask() *PruneTask {
	return &PruneTask{store: s.store, settings: s.collector.settings}
}
