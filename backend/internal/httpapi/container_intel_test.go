package httpapi

import (
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestDeriveRepositoryURLFallsBackToImageReference(t *testing.T) {
	summary := gen.ContainerSummary{
		Image:  "ghcr.io/blakeblackshear/frigate:stable",
		Labels: map[string]string{},
	}
	got := deriveRepositoryURL(summary)
	want := "https://github.com/blakeblackshear/frigate"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDeriveRepositoryURLFromDockerHubImage(t *testing.T) {
	summary := gen.ContainerSummary{
		Image:  "nginx:latest",
		Labels: map[string]string{},
	}
	got := deriveRepositoryURL(summary)
	want := "https://hub.docker.com/r/library/nginx"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDeriveChangelogURLForGitLabRepo(t *testing.T) {
	summary := gen.ContainerSummary{Labels: map[string]string{}}
	got := deriveChangelogURL(summary, "https://gitlab.com/example-org/platform/api")
	want := "https://gitlab.com/example-org/platform/api/-/releases"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
