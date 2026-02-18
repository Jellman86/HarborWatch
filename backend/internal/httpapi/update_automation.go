package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type upgradeContextBuilder interface {
	BuildUpgradeContext(ctx context.Context, repo, currentTag, targetTag string) (string, error)
}

func deriveGithubRepo(repoURL string) (string, bool) {
	raw := strings.TrimSpace(repoURL)
	if raw == "" {
		return "", false
	}

	// Support owner/name shorthand directly.
	parts := strings.Split(raw, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(raw, "://") {
		return raw, true
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return "", false
	}

	pathParts := strings.Split(strings.Trim(strings.TrimSpace(u.Path), "/"), "/")
	if len(pathParts) < 2 {
		return "", false
	}
	owner := strings.TrimSpace(pathParts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(pathParts[1]), ".git")
	if owner == "" || repo == "" {
		return "", false
	}
	return owner + "/" + repo, true
}

func summarizeReleaseIntel(intel gen.ReleaseRiskSummary) string {
	var b strings.Builder
	if intel.Repo != "" {
		b.WriteString(fmt.Sprintf("Repository: %s\n", intel.Repo))
	}
	if intel.LatestTag != "" {
		b.WriteString(fmt.Sprintf("Latest release tag: %s\n", intel.LatestTag))
	}
	b.WriteString(fmt.Sprintf("Aggregate risk score: %d/100\n", intel.TotalRisk))
	b.WriteString(fmt.Sprintf("Breaking change likely: %t\n", intel.BreakingChangeLikely))
	if len(intel.HighlightedExcerpts) > 0 {
		b.WriteString("Risk excerpts:\n")
		max := len(intel.HighlightedExcerpts)
		if max > 5 {
			max = 5
		}
		for i := 0; i < max; i++ {
			ex := intel.HighlightedExcerpts[i]
			b.WriteString(fmt.Sprintf("- [%s] %s\n", ex.Tag, ex.Text))
		}
	}
	return strings.TrimSpace(b.String())
}

func buildReleaseContext(ctx context.Context, releaseService ReleaseService, repo, currentTag, targetTag string) string {
	trimmedRepo := strings.TrimSpace(repo)
	if trimmedRepo == "" {
		return ""
	}
	if releaseService != nil {
		if advanced, ok := releaseService.(upgradeContextBuilder); ok {
			if text, err := advanced.BuildUpgradeContext(ctx, trimmedRepo, currentTag, targetTag); err == nil {
				if strings.TrimSpace(text) != "" {
					return text
				}
			}
		}
		if intel, err := releaseService.Analyze(ctx, trimmedRepo); err == nil {
			if text := summarizeReleaseIntel(intel); strings.TrimSpace(text) != "" {
				return text
			}
		}
	}

	// Graceful fallback for unsupported providers or unavailable release APIs.
	if trimmedRepo != "" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Repository reference: %s\n", trimmedRepo))
		if strings.TrimSpace(currentTag) != "" {
			b.WriteString(fmt.Sprintf("Current tag: %s\n", strings.TrimSpace(currentTag)))
		}
		if strings.TrimSpace(targetTag) != "" {
			b.WriteString(fmt.Sprintf("Target tag: %s\n", strings.TrimSpace(targetTag)))
		}
		b.WriteString("Structured release notes unavailable; rely on changelog URL and image metadata.")
		return strings.TrimSpace(b.String())
	}
	return ""
}

func imageTagFromRef(image string) string {
	raw := strings.TrimSpace(image)
	if raw == "" {
		return "unknown"
	}
	withoutDigest := strings.SplitN(raw, "@", 2)[0]
	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		tag := strings.TrimSpace(withoutDigest[lastColon+1:])
		if tag != "" {
			return tag
		}
	}
	return "latest"
}

func deriveDefaultChangelogURL(repoURL string) string {
	host, repoPath, ok := parseRepositoryRef(repoURL)
	if !ok {
		return ""
	}
	provider := providerFromHost(host)
	switch provider {
	case "github":
		parts := strings.Split(repoPath, "/")
		if len(parts) < 2 {
			return ""
		}
		return fmt.Sprintf("https://github.com/%s/%s/releases", parts[0], parts[1])
	case "gitlab":
		return fmt.Sprintf("https://%s/%s/-/releases", host, repoPath)
	case "bitbucket":
		parts := strings.Split(repoPath, "/")
		if len(parts) < 2 {
			return ""
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/downloads/?tab=tags", parts[0], parts[1])
	default:
		parts := strings.Split(repoPath, "/")
		if len(parts) < 2 {
			return ""
		}
		return fmt.Sprintf("https://%s/%s/%s/releases", host, parts[0], parts[1])
	}
}

func parseRepositoryRef(raw string) (string, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}
	if ownerRepo, ok := deriveGithubRepo(trimmed); ok {
		return "github.com", ownerRepo, true
	}
	if !strings.Contains(trimmed, "://") {
		first := strings.SplitN(trimmed, "/", 2)[0]
		if strings.Contains(first, ".") {
			trimmed = "https://" + trimmed
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return "", "", false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	parts := trimRepositoryPathParts(host, splitPathParts(parsed.Path))
	if len(parts) < 2 {
		return "", "", false
	}
	provider := providerFromHost(host)
	switch provider {
	case "github", "bitbucket":
		parts = parts[:2]
	case "gitlab":
		// Keep group/subgroup/project path.
	default:
		parts = parts[:2]
	}
	return host, strings.Join(parts, "/"), true
}

func providerFromHost(host string) string {
	switch {
	case host == "github.com":
		return "github"
	case host == "bitbucket.org":
		return "bitbucket"
	case strings.Contains(host, "gitlab"):
		return "gitlab"
	case strings.Contains(host, "gitea"), strings.Contains(host, "forgejo"):
		return "gitea"
	default:
		return "generic"
	}
}

func splitPathParts(rawPath string) []string {
	normalized := path.Clean("/" + strings.TrimSpace(rawPath))
	parts := strings.Split(strings.Trim(normalized, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || item == "." {
			continue
		}
		out = append(out, item)
	}
	return out
}

func trimRepositoryPathParts(host string, parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	stopTokens := map[string]struct{}{
		"-":              {},
		"releases":       {},
		"release":        {},
		"tags":           {},
		"tag":            {},
		"tree":           {},
		"blob":           {},
		"src":            {},
		"commits":        {},
		"commit":         {},
		"compare":        {},
		"pull":           {},
		"pulls":          {},
		"issues":         {},
		"wiki":           {},
		"merge_requests": {},
		"merge-requests": {},
	}
	cut := len(parts)
	for i, part := range parts {
		lower := strings.ToLower(strings.TrimSpace(part))
		if _, stop := stopTokens[lower]; stop {
			cut = i
			break
		}
	}
	parts = append([]string(nil), parts[:cut]...)
	if len(parts) == 0 {
		return nil
	}
	parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".git")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	if strings.EqualFold(host, "registry.gitlab.com") && len(parts) >= 3 {
		// registry.gitlab.com/group/project/image -> group/project
		if len(parts) > 2 {
			parts = parts[:len(parts)-1]
		}
	}
	if strings.EqualFold(host, "registry.gitlab.com") && len(parts) >= 3 && strings.EqualFold(parts[0], "v2") {
		parts = parts[1:]
		if len(parts) > 2 {
			parts = parts[:len(parts)-1]
		}
	}
	return parts
}
