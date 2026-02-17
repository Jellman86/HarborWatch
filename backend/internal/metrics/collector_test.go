package metrics

import (
	"math"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestComputeCPUPercent_UsesOnlineCPUsWhenPercpuMissing(t *testing.T) {
	stats := container.StatsResponse{}
	stats.CPUStats.CPUUsage.TotalUsage = 2_000_000_000
	stats.PreCPUStats.CPUUsage.TotalUsage = 1_000_000_000
	stats.CPUStats.SystemUsage = 10_000_000_000
	stats.PreCPUStats.SystemUsage = 5_000_000_000
	stats.CPUStats.OnlineCPUs = 4
	// No percpu usage data from engine.
	stats.CPUStats.CPUUsage.PercpuUsage = nil
	stats.PreCPUStats.CPUUsage.PercpuUsage = nil

	got := computeCPUPercent(stats)
	want := 80.0 // (1e9/5e9) * 4 * 100
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("unexpected cpu percent: got %.4f want %.4f", got, want)
	}
}

func TestComputeCPUPercent_UsesPercpuLengthFallback(t *testing.T) {
	stats := container.StatsResponse{}
	stats.CPUStats.CPUUsage.TotalUsage = 3_000_000_000
	stats.PreCPUStats.CPUUsage.TotalUsage = 2_000_000_000
	stats.CPUStats.SystemUsage = 12_000_000_000
	stats.PreCPUStats.SystemUsage = 10_000_000_000
	stats.CPUStats.OnlineCPUs = 0
	stats.CPUStats.CPUUsage.PercpuUsage = []uint64{1, 2}

	got := computeCPUPercent(stats)
	want := 100.0 // (1e9/2e9) * 2 * 100
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("unexpected cpu percent: got %.4f want %.4f", got, want)
	}
}
