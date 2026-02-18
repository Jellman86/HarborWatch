package httpapi

import (
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/containerintel"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type containerIntelResponse struct {
	ContainerID            string `json:"containerId"`
	ContainerName          string `json:"containerName,omitempty"`
	Image                  string `json:"image,omitempty"`
	OverrideRepositoryURL  string `json:"overrideRepositoryUrl,omitempty"`
	OverrideChangelogURL   string `json:"overrideChangelogUrl,omitempty"`
	DerivedRepositoryURL   string `json:"derivedRepositoryUrl,omitempty"`
	DerivedChangelogURL    string `json:"derivedChangelogUrl,omitempty"`
	EffectiveRepositoryURL string `json:"effectiveRepositoryUrl,omitempty"`
	EffectiveChangelogURL  string `json:"effectiveChangelogUrl,omitempty"`
	UpdatedAt              int64  `json:"updatedAt,omitempty"`
}

func deriveRepositoryURL(summary gen.ContainerSummary) string {
	return firstNonEmpty(
		summary.Labels["harborwatch.intel.url"],
		summary.Labels["org.opencontainers.image.source"],
		summary.Labels["org.label-schema.vcs-url"],
		deriveRepositoryURLFromImage(summary.Image),
	)
}

func deriveChangelogURL(summary gen.ContainerSummary, repoURL string) string {
	if direct := firstNonEmpty(
		summary.Labels["harborwatch.changelog.url"],
		summary.Labels["io.harborwatch.changelog_url"],
		summary.Labels["org.opencontainers.image.documentation"],
		summary.Labels["org.opencontainers.image.url"],
	); direct != "" {
		return direct
	}
	return deriveDefaultChangelogURL(repoURL)
}

func effectiveContainerIntel(summary gen.ContainerSummary, ov containerintel.Override) containerIntelResponse {
	derivedRepo := deriveRepositoryURL(summary)
	derivedChangelog := deriveChangelogURL(summary, derivedRepo)
	overrideRepo := strings.TrimSpace(ov.RepositoryURL)
	overrideChangelog := strings.TrimSpace(ov.ChangelogURL)

	return containerIntelResponse{
		ContainerID:            summary.ID,
		ContainerName:          trimContainerName(summary.Names),
		Image:                  strings.TrimSpace(summary.Image),
		OverrideRepositoryURL:  overrideRepo,
		OverrideChangelogURL:   overrideChangelog,
		DerivedRepositoryURL:   derivedRepo,
		DerivedChangelogURL:    derivedChangelog,
		EffectiveRepositoryURL: firstNonEmpty(overrideRepo, derivedRepo),
		EffectiveChangelogURL:  firstNonEmpty(overrideChangelog, derivedChangelog),
		UpdatedAt:              ov.UpdatedAt,
	}
}

func deriveRepositoryURLFromImage(imageRef string) string {
	ref := strings.ToLower(strings.TrimSpace(taglessImageRef(imageRef)))
	if ref == "" {
		return ""
	}
	registry, path, ok := splitImageRegistryAndPath(ref)
	if !ok {
		return ""
	}
	parts := splitPathParts(path)
	if len(parts) == 0 {
		return ""
	}
	switch registry {
	case "ghcr.io":
		if len(parts) < 2 {
			return ""
		}
		return "https://github.com/" + parts[0] + "/" + parts[1]
	case "registry.gitlab.com":
		if len(parts) < 2 {
			return ""
		}
		repoParts := append([]string(nil), parts...)
		if len(repoParts) > 2 {
			repoParts = repoParts[:len(repoParts)-1]
		}
		if len(repoParts) < 2 {
			return ""
		}
		return "https://gitlab.com/" + strings.Join(repoParts, "/")
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		if len(parts) == 1 {
			return "https://hub.docker.com/r/library/" + parts[0]
		}
		return "https://hub.docker.com/r/" + parts[0] + "/" + parts[1]
	case "quay.io":
		if len(parts) < 2 {
			return ""
		}
		return "https://quay.io/repository/" + parts[0] + "/" + parts[1]
	default:
		return ""
	}
}

func splitImageRegistryAndPath(ref string) (string, string, bool) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", "", false
	}
	if !hasExplicitRegistry(trimmed) {
		if strings.Contains(trimmed, "/") {
			return "docker.io", trimmed, true
		}
		return "docker.io", "library/" + trimmed, true
	}
	idx := strings.Index(trimmed, "/")
	if idx <= 0 || idx+1 >= len(trimmed) {
		return "", "", false
	}
	registry := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	path := strings.TrimSpace(trimmed[idx+1:])
	if registry == "" || path == "" {
		return "", "", false
	}
	return registry, path, true
}
