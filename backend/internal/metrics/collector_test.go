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

	got := computeCPUPercent(stats, true)
	want := 20.0 // (1e9/5e9) * 100
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

	got := computeCPUPercent(stats, true)
	want := 50.0 // (1e9/2e9) * 100
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("unexpected cpu percent: got %.4f want %.4f", got, want)
	}
}

func TestComputeCPUPercent_ClampsHighValues(t *testing.T) {
	stats := container.StatsResponse{}
	stats.CPUStats.CPUUsage.TotalUsage = 1_000_000_000_000
	stats.PreCPUStats.CPUUsage.TotalUsage = 0
	stats.CPUStats.SystemUsage = 1
	stats.PreCPUStats.SystemUsage = 0
	stats.CPUStats.OnlineCPUs = 128

	got := computeCPUPercent(stats, false)
	if got != 10000 {
		t.Fatalf("expected clamped cpu percent at 10000, got %.4f", got)
	}
}

func TestComputeCPUPercent_MultipliesByCoresWhenNotNormalized(t *testing.T) {
	stats := container.StatsResponse{}
	stats.CPUStats.CPUUsage.TotalUsage = 2_000_000_000
	stats.PreCPUStats.CPUUsage.TotalUsage = 1_000_000_000
	stats.CPUStats.SystemUsage = 10_000_000_000
	stats.PreCPUStats.SystemUsage = 5_000_000_000
	stats.CPUStats.OnlineCPUs = 4

	// Normalized (system total)
	gotN := computeCPUPercent(stats, true)
	wantN := 20.0
	if math.Abs(gotN-wantN) > 0.0001 {
		t.Fatalf("normalized: expected %.4f, got %.4f", wantN, gotN)
	}

	// Raw (per-core)
	gotR := computeCPUPercent(stats, false)
	wantR := 80.0 // 20.0 * 4 cores
	if math.Abs(gotR-wantR) > 0.0001 {
		t.Fatalf("raw: expected %.4f, got %.4f", wantR, gotR)
	}
}
