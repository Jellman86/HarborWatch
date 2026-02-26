package httpapi

import (
	"net/url"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/containerintel"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type containerIntelIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Action   string `json:"action,omitempty"`
}

type containerIntelResponse struct {
	ContainerID            string                `json:"containerId"`
	ContainerName          string                `json:"containerName,omitempty"`
	Image                  string                `json:"image,omitempty"`
	OverrideRepositoryURL  string                `json:"overrideRepositoryUrl,omitempty"`
	OverrideChangelogURL   string                `json:"overrideChangelogUrl,omitempty"`
	DerivedRepositoryURL   string                `json:"derivedRepositoryUrl,omitempty"`
	DerivedChangelogURL    string                `json:"derivedChangelogUrl,omitempty"`
	EffectiveRepositoryURL string                `json:"effectiveRepositoryUrl,omitempty"`
	EffectiveChangelogURL  string                `json:"effectiveChangelogUrl,omitempty"`
	RepositoryProvider     string                `json:"repositoryProvider,omitempty"`
	HasRepository          bool                  `json:"hasRepository"`
	HasChangelog           bool                  `json:"hasChangelog"`
	ReleaseIntelReady      bool                  `json:"releaseIntelReady"`
	FullAutomationReady    bool                  `json:"fullAutomationReady"`
	Issues                 []containerIntelIssue `json:"issues,omitempty"`
	PortainerManaged       bool                  `json:"portainerManaged"`
	PortainerConfigured    bool                  `json:"portainerConfigured"`
	OrchestrationMode      string                `json:"orchestrationMode,omitempty"`
	ComposeProject         string                `json:"composeProject,omitempty"`
	ComposeService         string                `json:"composeService,omitempty"`
	ComposeWorkingDir      string                `json:"composeWorkingDir,omitempty"`
	ComposeConfigFiles     []string              `json:"composeConfigFiles,omitempty"`
	ComposeSourceStatus    string                `json:"composeSourceStatus,omitempty"`
	ComposeSourceVerified  bool                  `json:"composeSourceVerified"`
	ComposeSourceWritable  bool                  `json:"composeSourceWritable"`
	UpdatedAt              int64                 `json:"updatedAt,omitempty"`
}

func deriveRepositoryURL(summary gen.ContainerSummary) string {
	return firstNonEmpty(
		summary.Labels["harborwatch.intel.url"],
		summary.Labels["org.opencontainers.image.source"],
		summary.Labels["org.label-schema.vcs-url"],
		repositoryURLFromGenericLabels(summary),
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

func effectiveContainerIntel(summary gen.ContainerSummary, ov containerintel.Override, portainerService PortainerClient) containerIntelResponse {
	derivedRepo := deriveRepositoryURL(summary)
	derivedChangelog := deriveChangelogURL(summary, derivedRepo)
	overrideRepo := strings.TrimSpace(ov.RepositoryURL)
	overrideChangelog := strings.TrimSpace(ov.ChangelogURL)
	effectiveRepo := firstNonEmpty(overrideRepo, derivedRepo)
	effectiveChangelog := firstNonEmpty(overrideChangelog, derivedChangelog)
	readiness := evaluateContainerIntelReadiness(effectiveRepo, effectiveChangelog)

	mode := detectContainerOrchestrationMode(summary)
	portainerManaged := mode == orchestrationModePortainerStack
	portainerConfigured := portainerService != nil
	composeProject := strings.TrimSpace(summary.Labels["com.docker.compose.project"])
	composeService := strings.TrimSpace(summary.Labels["com.docker.compose.service"])
	composeWorkingDir := strings.TrimSpace(summary.Labels["com.docker.compose.project.working_dir"])
	composeConfigFiles := splitComposePathList(summary.Labels["com.docker.compose.project.config_files"])
	composeStatus := composeSourceStatus("")
	if mode == orchestrationModeDockerCompose {
		composeStatus = detectComposeSourceStatus(composeConfigFiles)
	}

	if portainerManaged && !portainerConfigured {
		readiness.Issues = append(readiness.Issues, containerIntelIssue{
			Code:     "portainer_integration_missing",
			Severity: "error",
			Message:  "Portainer integration is required for this container's lifecycle management.",
			Action:   "Configure Portainer API in Settings > Integrations to enable safe updates.",
		})
		readiness.FullAutomationReady = false
	}

	return containerIntelResponse{
		ContainerID:            summary.ID,
		ContainerName:          trimContainerName(summary.Names),
		Image:                  strings.TrimSpace(summary.Image),
		OverrideRepositoryURL:  overrideRepo,
		OverrideChangelogURL:   overrideChangelog,
		DerivedRepositoryURL:   derivedRepo,
		DerivedChangelogURL:    derivedChangelog,
		EffectiveRepositoryURL: effectiveRepo,
		EffectiveChangelogURL:  effectiveChangelog,
		RepositoryProvider:     readiness.RepositoryProvider,
		HasRepository:          readiness.HasRepository,
		HasChangelog:           readiness.HasChangelog,
		ReleaseIntelReady:      readiness.ReleaseIntelReady,
		FullAutomationReady:    readiness.FullAutomationReady,
		Issues:                 readiness.Issues,
		PortainerManaged:       portainerManaged,
		PortainerConfigured:    portainerConfigured,
		OrchestrationMode:      string(mode),
		ComposeProject:         composeProject,
		ComposeService:         composeService,
		ComposeWorkingDir:      composeWorkingDir,
		ComposeConfigFiles:     composeConfigFiles,
		ComposeSourceStatus:    string(composeStatus),
		ComposeSourceVerified:  composeStatus != "" && composeStatus != composeSourceStatusUnverified,
		ComposeSourceWritable:  composeStatus == composeSourceStatusVerifiedWritable,
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
	if linuxServerRepo := deriveLinuxServerRepoURL(parts); linuxServerRepo != "" {
		return linuxServerRepo
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
	case "lscr.io":
		if len(parts) < 2 {
			return ""
		}
		return "https://github.com/" + parts[0] + "/" + parts[1]
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

func repositoryURLFromGenericLabels(summary gen.ContainerSummary) string {
	candidates := []string{
		summary.Labels["org.opencontainers.image.url"],
		summary.Labels["org.opencontainers.image.documentation"],
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, _, ok := parseRepositoryRef(candidate); ok {
			return candidate
		}
	}
	return ""
}

func deriveLinuxServerRepoURL(parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(parts[0]), "linuxserver") {
		return ""
	}
	repo := strings.TrimSpace(parts[1])
	if repo == "" {
		return ""
	}
	if !strings.HasPrefix(repo, "docker-") {
		repo = "docker-" + repo
	}
	return "https://github.com/linuxserver/" + repo
}

type intelReadiness struct {
	RepositoryProvider  string
	HasRepository       bool
	HasChangelog        bool
	ReleaseIntelReady   bool
	FullAutomationReady bool
	Issues              []containerIntelIssue
}

func evaluateContainerIntelReadiness(repoURL, changelogURL string) intelReadiness {
	out := intelReadiness{
		HasRepository: strings.TrimSpace(repoURL) != "",
		HasChangelog:  strings.TrimSpace(changelogURL) != "",
		Issues:        []containerIntelIssue{},
	}

	if !out.HasRepository {
		out.Issues = append(out.Issues, containerIntelIssue{
			Code:     "repository_missing",
			Severity: "warning",
			Message:  "Repository source could not be derived.",
			Action:   "Set Repository URL Override or add an OCI source label (org.opencontainers.image.source).",
		})
	} else {
		host, repoPath, ok := parseRepositoryRef(repoURL)
		if !ok {
			out.Issues = append(out.Issues, containerIntelIssue{
				Code:     "repository_invalid",
				Severity: "warning",
				Message:  "Repository reference is not parseable for release analysis.",
				Action:   "Use a repository URL such as https://github.com/owner/repo.",
			})
		} else {
			provider := providerFromHost(host)
			out.RepositoryProvider = provider
			switch provider {
			case "github", "gitlab", "gitea", "bitbucket":
				out.ReleaseIntelReady = true
			default:
				out.ReleaseIntelReady = false
			}

			if provider == "generic" {
				out.Issues = append(out.Issues, containerIntelIssue{
					Code:     "release_provider_generic",
					Severity: "warning",
					Message:  "Repository host is not a first-class release provider.",
					Action:   "Set repository override to upstream source (GitHub/GitLab/Gitea/Bitbucket) for richer release intelligence.",
				})
			}

			repoPathLower := strings.ToLower(strings.TrimSpace(repoPath))
			if strings.Contains(host, "docker.com") || strings.Contains(host, "docker.io") || strings.Contains(host, "quay.io") || strings.HasPrefix(repoPathLower, "r/") {
				out.Issues = append(out.Issues, containerIntelIssue{
					Code:     "repository_points_to_registry",
					Severity: "info",
					Message:  "Repository URL appears to be a registry page, not an upstream source repo.",
					Action:   "Prefer an upstream source repository URL for higher quality AI release-note analysis.",
				})
			}
		}
	}

	if !out.HasChangelog {
		out.Issues = append(out.Issues, containerIntelIssue{
			Code:     "changelog_missing",
			Severity: "info",
			Message:  "Changelog/release URL is not available.",
			Action:   "Set Changelog URL Override to improve release note traceability.",
		})
	} else if !looksLikeHTTPURL(changelogURL) {
		out.Issues = append(out.Issues, containerIntelIssue{
			Code:     "changelog_invalid",
			Severity: "warning",
			Message:  "Changelog URL is not a valid HTTP(S) URL.",
			Action:   "Provide a valid changelog or releases URL.",
		})
	}

	out.FullAutomationReady = out.ReleaseIntelReady && out.HasChangelog
	return out
}

func looksLikeHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed == nil {
		return false
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	return (scheme == "http" || scheme == "https") && strings.TrimSpace(parsed.Hostname()) != ""
}

func mapContainerIntelOverrides(items []containerintel.Override) map[string]containerintel.Override {
	out := make(map[string]containerintel.Override, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.ContainerID)
		if key == "" {
			continue
		}
		out[key] = item
	}
	return out
}

func overrideForContainerID(containerID string, overrides map[string]containerintel.Override) containerintel.Override {
	id := strings.TrimSpace(containerID)
	if id == "" || len(overrides) == 0 {
		return containerintel.Override{}
	}
	if item, ok := overrides[id]; ok {
		return item
	}
	for key, item := range overrides {
		if key == "" {
			continue
		}
		if strings.HasPrefix(id, key) || strings.HasPrefix(key, id) {
			return item
		}
	}
	return containerintel.Override{}
}
