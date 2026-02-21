package scheduler

import (
	"context"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/docker/docker/api/types/container"
)

func TestNormalizeTrivySweepMode(t *testing.T) {
	if got := normalizeTrivySweepMode(""); got != TrivySweepModeRunningOnly {
		t.Fatalf("expected default mode running-only, got %q", got)
	}
	if got := normalizeTrivySweepMode("ALL-IMAGES"); got != TrivySweepModeAllImages {
		t.Fatalf("expected all-images normalization, got %q", got)
	}
	if got := normalizeTrivySweepMode("invalid"); got != TrivySweepModeRunningOnly {
		t.Fatalf("expected invalid mode fallback to running-only, got %q", got)
	}
}

func TestTrivyTargetsFromContainers_DedupesAndHonorsPolicy(t *testing.T) {
	containers := []container.Summary{
		{ID: "c1", Image: "nginx:1.27"},
		{ID: "c2", Image: "nginx:1.27"},
		{ID: "c3", Image: "redis:7"},
		{ID: "c4", Image: "<none>:<none>"},
	}
	allow := func(ctx context.Context, containerID string) bool {
		return containerID != "c2"
	}

	got := trivyTargetsFromContainers(containers, allow, context.Background())
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d (%v)", len(got), got)
	}
	if got[0] != "nginx:1.27" || got[1] != "redis:7" {
		t.Fatalf("unexpected targets order/content: %v", got)
	}
}

func TestTrivyTargetsFromImages_PrefersTagsAndSkipsDangling(t *testing.T) {
	images := []gen.ImageSummary{
		{ID: "sha256:aaa", RepoTags: []string{"ghcr.io/acme/api:1.0", "ghcr.io/acme/api:latest"}},
		{ID: "sha256:bbb", RepoTags: []string{"<none>:<none>"}},
		{ID: "sha256:ccc", RepoTags: []string{}},
		{ID: "sha256:aaa", RepoTags: []string{"ghcr.io/acme/api:1.0"}},
	}

	got := trivyTargetsFromImages(images)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d (%v)", len(got), got)
	}
	if got[0] != "ghcr.io/acme/api:1.0" || got[1] != "sha256:ccc" {
		t.Fatalf("unexpected targets order/content: %v", got)
	}
}
