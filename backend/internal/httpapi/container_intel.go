package httpapi

import (
	"fmt"
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
	if repo, ok := deriveGithubRepo(repoURL); ok {
		return fmt.Sprintf("https://github.com/%s/releases", repo)
	}
	return ""
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
