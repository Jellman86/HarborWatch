package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/moby/client"
)

type Collector struct {
	docker *client.Client
	store  *Store
}

func NewCollector(docker *client.Client, store *Store) *Collector {
	return &Collector{docker: docker, store: store}
}

func (c *Collector) Name() string { return "metrics_collector" }

// Run implements the scheduler.Task interface.
func (c *Collector) Run(ctx context.Context) error {
	log.Println("Starting metrics collection sweep...")
	containers, err := c.docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, cont := range containers {
		if cont.State != "running" {
			continue
		}
		if err := c.collectOne(ctx, cont.ID); err != nil {
			log.Printf("Failed to collect stats for %s: %v", cont.ID, err)
		}
	}
	return nil
}

func (c *Collector) collectOne(ctx context.Context, id string) error {
	statsResp, err := c.docker.ContainerStats(ctx, id, false) // Stream: false for snapshot
	if err != nil {
		return err
	}
	defer statsResp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		// Sometimes stats are empty or invalid if container is restarting
		if err == io.EOF {
			return nil
		}
		return err
	}

	m := Metric{
		ContainerID: id,
		Timestamp:   time.Now().UTC().Unix(),
		MemoryUsage: int64(stats.MemoryStats.Usage),
		MemoryLimit: int64(stats.MemoryStats.Limit),
		Pids:        int(stats.PidsStats.Current),
	}

	// Calculate CPU Percent (Docker CLI Logic)
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		m.CPUPercent = (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	return c.store.SaveMetric(ctx, m)
}
