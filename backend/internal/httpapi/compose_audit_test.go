package httpapi

import (
	"strings"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestBuildComposeConfigView_DerivesServiceScopeFromComposeLabels(t *testing.T) {
	full := `
name: media
services:
  gluetun:
    image: qmcgaw/gluetun:latest
    networks:
      - vpn
    volumes:
      - gluetun-state:/gluetun
  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    network_mode: "service:gluetun"
networks:
  vpn: {}
volumes:
  gluetun-state: {}
`
	view := buildComposeConfigView(full, gen.ContainerSummary{
		ID: "c1",
		Labels: map[string]string{
			"com.docker.compose.project": "media",
			"com.docker.compose.service": "gluetun",
		},
	}, "service")

	if view.Mode != "compose" {
		t.Fatalf("expected compose mode, got %q", view.Mode)
	}
	if view.AppliedScope != "service" {
		t.Fatalf("expected applied scope service, got %q", view.AppliedScope)
	}
	if !strings.Contains(view.Config, "services:") || !strings.Contains(view.Config, "gluetun:") {
		t.Fatalf("expected scoped config to contain target service, got:\n%s", view.Config)
	}
	if strings.Contains(view.Config, "qbittorrent:") {
		t.Fatalf("expected scoped config to omit other services, got:\n%s", view.Config)
	}
	if !strings.Contains(view.Config, "networks:") || !strings.Contains(view.Config, "vpn:") {
		t.Fatalf("expected scoped config to retain referenced network, got:\n%s", view.Config)
	}
	if !strings.Contains(view.Config, "volumes:") || !strings.Contains(view.Config, "gluetun-state:") {
		t.Fatalf("expected scoped config to retain referenced volume, got:\n%s", view.Config)
	}
}

func TestBuildComposeConfigView_FallsBackForClassicDocker(t *testing.T) {
	full := "services:\n  app:\n    image: nginx:latest\n"
	view := buildComposeConfigView(full, gen.ContainerSummary{ID: "classic", Labels: map[string]string{}}, "service")
	if view.Mode != "classic-docker" {
		t.Fatalf("expected classic-docker mode, got %q", view.Mode)
	}
	if view.AppliedScope != "full" {
		t.Fatalf("expected full fallback scope, got %q", view.AppliedScope)
	}
	if view.Config != full {
		t.Fatalf("expected original config passthrough, got %q", view.Config)
	}
}

func TestBuildComposeConfigView_RespectsFullScopeRequest(t *testing.T) {
	full := "services:\n  gluetun:\n    image: qmcgaw/gluetun:latest\n  qbittorrent:\n    image: x\n"
	view := buildComposeConfigView(full, gen.ContainerSummary{
		Labels: map[string]string{
			"com.docker.compose.project": "media",
			"com.docker.compose.service": "gluetun",
		},
	}, "full")
	if view.AppliedScope != "full" {
		t.Fatalf("expected full scope, got %q", view.AppliedScope)
	}
	if !strings.Contains(view.Config, "qbittorrent:") {
		t.Fatalf("expected full config to be preserved, got:\n%s", view.Config)
	}
}
