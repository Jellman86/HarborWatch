package httpapi

import (
	"context"
	"fmt"
	"net/url"
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
	if releaseService == nil {
		return ""
	}
	if advanced, ok := releaseService.(upgradeContextBuilder); ok {
		if text, err := advanced.BuildUpgradeContext(ctx, repo, currentTag, targetTag); err == nil {
			if strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	if intel, err := releaseService.Analyze(ctx, repo); err == nil {
		return summarizeReleaseIntel(intel)
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
