package httpapi

import (
	"context"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type fakeImageScanLookup struct {
	summaryByTarget map[string]*gen.ScanSummary
	malwareByTarget map[string][]gen.MalwareScanSummary
}

func (f fakeImageScanLookup) LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error) {
	if f.summaryByTarget == nil {
		return nil, nil
	}
	return f.summaryByTarget[target], nil
}

func (f fakeImageScanLookup) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	if f.malwareByTarget == nil {
		return nil, nil
	}
	return f.malwareByTarget[target], nil
}

func TestBuildImageIntelligence_NoTagContainerMatchesLatestImage(t *testing.T) {
	docker := fakeDockerClient{
		images: []gen.ImageSummary{
			{ID: "img1", RepoTags: []string{"cloudflare/cloudflared:latest"}, Size: 100},
		},
		containers: []gen.ContainerSummary{
			{ID: "c1", Image: "cloudflare/cloudflared", UpdateAvailable: true},
		},
	}

	rows, err := buildImageIntelligence(context.Background(), docker, fakeImageScanLookup{})
	if err != nil {
		t.Fatalf("buildImageIntelligence error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if !row.InUse {
		t.Fatalf("expected inUse=true for repo:latest image matched by no-tag container image")
	}
	if row.PruneCandidate {
		t.Fatalf("expected pruneCandidate=false for running image")
	}
	if !row.Outdated {
		t.Fatalf("expected outdated=true when matched container has updateAvailable=true")
	}
}

func TestBuildImageIntelligence_ScanFallbackToTaglessTarget(t *testing.T) {
	docker := fakeDockerClient{
		images: []gen.ImageSummary{
			{ID: "img1", RepoTags: []string{"cloudflare/cloudflared:latest"}, Size: 100},
		},
		containers: []gen.ContainerSummary{
			{ID: "c1", Image: "cloudflare/cloudflared"},
		},
	}
	scans := fakeImageScanLookup{
		summaryByTarget: map[string]*gen.ScanSummary{
			"cloudflare/cloudflared": {
				Target:    "cloudflare/cloudflared",
				ScannedAt: 1771414793,
				Total:     12,
				Critical:  1,
				High:      2,
				Medium:    3,
				Low:       6,
				RiskScore: 30,
				Source:    "trivy",
				Unknown:   0,
			},
		},
	}

	rows, err := buildImageIntelligence(context.Background(), docker, scans)
	if err != nil {
		t.Fatalf("buildImageIntelligence error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.VulnerabilityTotal != 12 {
		t.Fatalf("expected vulnerabilityTotal=12, got %d", row.VulnerabilityTotal)
	}
	if row.VulnerabilityCritical != 1 {
		t.Fatalf("expected vulnerabilityCritical=1, got %d", row.VulnerabilityCritical)
	}
	if row.SecurityScannedAt == 0 {
		t.Fatalf("expected securityScannedAt to be set from matched tagless scan target")
	}
}
