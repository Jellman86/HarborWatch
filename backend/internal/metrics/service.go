package metrics

import (
	"context"
	"os"
	"time"

	"github.com/moby/moby/client"
)

type Service struct {
	store     *Store
	collector *Collector
}

func NewService(store *Store, docker *client.Client) *Service {
	return &Service{
		store:     store,
		collector: NewCollector(docker, store),
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
	
	return s.store.GetMetrics(ctx, containerID, since)
}

// PruneTask cleans up metrics older than 7 days
type PruneTask struct {
	store *Store
}

func (t *PruneTask) Name() string { return "metrics_prune" }
func (t *PruneTask) Run(ctx context.Context) error {
	// Keep 7 days of data
	olderThan := time.Now().Add(-7 * 24 * time.Hour).Unix()
	_, err := t.store.PruneMetrics(ctx, olderThan)
	return err
}

func (s *Service) GetPruneTask() *PruneTask {
	return &PruneTask{store: s.store}
}
