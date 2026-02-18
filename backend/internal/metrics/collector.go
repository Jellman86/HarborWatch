package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"runtime"
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

		// Use a sub-context with a tight timeout for each individual container
		// to prevent one slow container from stalling the entire sweep.
		subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := c.collectOne(subCtx, cont.ID); err != nil {
			log.Printf("Failed to collect stats for %s: %v", cont.ID, err)
		}
		cancel()
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

	m.CPUPercent = computeCPUPercent(stats)

	return c.store.SaveMetric(ctx, m)
}

func computeCPUPercent(stats container.StatsResponse) float64 {
	cpuCount := cpuCoreCount(stats)
	if cpuCount <= 0 {
		cpuCount = 1
	}

	// Primary path: delta-based CPU% (matches Docker CLI behavior).
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta >= 0 {
		return normalizeCPUPercent((cpuDelta / systemDelta) * cpuCount * 100)
	}

	// Fallback for engines that do not provide valid pre-CPU snapshots.
	if stats.CPUStats.SystemUsage > 0 && stats.CPUStats.CPUUsage.TotalUsage > 0 {
		return normalizeCPUPercent((float64(stats.CPUStats.CPUUsage.TotalUsage) / float64(stats.CPUStats.SystemUsage)) * cpuCount * 100)
	}

	return 0
}

func normalizeCPUPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func cpuCoreCount(stats container.StatsResponse) float64 {
	if stats.CPUStats.OnlineCPUs > 0 {
		return float64(stats.CPUStats.OnlineCPUs)
	}
	if l := len(stats.CPUStats.CPUUsage.PercpuUsage); l > 0 {
		return float64(l)
	}
	if l := len(stats.PreCPUStats.CPUUsage.PercpuUsage); l > 0 {
		return float64(l)
	}
	return float64(runtime.NumCPU())
}
