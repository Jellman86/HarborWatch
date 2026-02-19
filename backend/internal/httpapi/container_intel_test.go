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

func TestDeriveRepositoryURLFromLSCRLinuxServerImage(t *testing.T) {
	summary := gen.ContainerSummary{
		Image:  "lscr.io/linuxserver/radarr:latest",
		Labels: map[string]string{},
	}
	got := deriveRepositoryURL(summary)
	want := "https://github.com/linuxserver/docker-radarr"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEvaluateContainerIntelReadinessFlagsMissingRepo(t *testing.T) {
	got := evaluateContainerIntelReadiness("", "")
	if got.ReleaseIntelReady {
		t.Fatalf("expected release intel to be false when repo is missing")
	}
	if got.FullAutomationReady {
		t.Fatalf("expected full automation readiness to be false when metadata is missing")
	}
	if len(got.Issues) == 0 {
		t.Fatalf("expected issues for missing metadata")
	}
}
