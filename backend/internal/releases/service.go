package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type Service struct {
	client *http.Client
}

func NewService() *Service {
	return &Service{client: &http.Client{Timeout: 20 * time.Second}}
}

func (s *Service) Analyze(ctx context.Context, repo string) (gen.ReleaseRiskSummary, error) {
	if strings.TrimSpace(repo) == "" {
		repo = defaultRepo()
	}
	if !validRepo(repo) {
		return gen.ReleaseRiskSummary{}, fmt.Errorf("invalid repo format: expected owner/name")
	}

	releases, err := s.fetchReleases(ctx, repo)
	if err != nil {
		return gen.ReleaseRiskSummary{}, err
	}

	summary := gen.ReleaseRiskSummary{
		Repo:                repo,
		ReleasesAnalyzed:    len(releases),
		GeneratedAt:         time.Now().UTC().Unix(),
		HighlightedExcerpts: []gen.ReleaseExcerpt{},
	}
	if len(releases) == 0 {
		return summary, nil
	}

	latest := releases[0]
	summary.LatestTag = latest.TagName
	if latest.PublishedAt != "" {
		t, _ := time.Parse(time.RFC3339, latest.PublishedAt)
		summary.LatestPublishedAt = t.Unix()
	}

	for _, rel := range releases {
		analysis := analyzeReleaseText(rel)
		summary.TotalRisk += analysis.score
		summary.HighlightedExcerpts = append(summary.HighlightedExcerpts, analysis.excerpts...)
	}

	if summary.TotalRisk > 100 {
		summary.TotalRisk = 100
	}
	if len(summary.HighlightedExcerpts) > 12 {
		summary.HighlightedExcerpts = summary.HighlightedExcerpts[:12]
	}
	summary.BreakingChangeLikely = summary.TotalRisk >= 45

	sort.Slice(summary.HighlightedExcerpts, func(i, j int) bool {
		return summary.HighlightedExcerpts[i].Weight > summary.HighlightedExcerpts[j].Weight
	})

	return summary, nil
}

// BuildUpgradeContext returns release-note context scoped to the current->target tag range.
func (s *Service) BuildUpgradeContext(ctx context.Context, repo, currentTag, targetTag string) (string, error) {
	if strings.TrimSpace(repo) == "" {
		repo = defaultRepo()
	}
	if !validRepo(repo) {
		return "", fmt.Errorf("invalid repo format: expected owner/name")
	}

	releases, err := s.fetchReleases(ctx, repo)
	if err != nil {
		return "", err
	}

	selected, reason := selectUpgradeReleases(releases, currentTag, targetTag)
	return formatUpgradeContext(repo, currentTag, targetTag, selected, reason), nil
}

func (s *Service) fetchReleases(ctx context.Context, repo string) ([]githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases?per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "harborwatch-release-intelligence")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("github releases failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode github releases: %w", err)
	}
	return releases, nil
}

func analyzeReleaseText(r githubRelease) releaseAnalysis {
	text := strings.ToLower(strings.Join([]string{r.Name, r.TagName, r.Body}, "\n"))

	rules := []struct {
		name    string
		pattern *regexp.Regexp
		weight  int
	}{
		{"breaking", regexp.MustCompile(`\bbreaking\b|\bbreaking change\b|\bincompatible\b`), 25},
		{"migration", regexp.MustCompile(`\bmigration\b|\bupgrade step\b|\bdatabase change\b`), 16},
		{"security", regexp.MustCompile(`\bcve\b|\bsecurity\b|\bvulnerability\b|\bauth\b`), 14},
		{"deprecation", regexp.MustCompile(`\bdeprecated\b|\bremoved\b|\bdropped\b`), 12},
		{"api", regexp.MustCompile(`\bapi\b|\bendpoint\b|\bschema\b`), 8},
	}

	out := releaseAnalysis{}
	lines := strings.Split(r.Body, "\n")

	for _, rule := range rules {
		if !rule.pattern.MatchString(text) {
			continue
		}
		out.score += rule.weight
		for _, line := range lines {
			if rule.pattern.MatchString(strings.ToLower(line)) {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				out.excerpts = append(out.excerpts, gen.ReleaseExcerpt{
					Tag:    r.TagName,
					Text:   truncate(trimmed, 220),
					Weight: rule.weight,
				})
				break
			}
		}
	}

	if out.score == 0 {
		out.excerpts = append(out.excerpts, gen.ReleaseExcerpt{Tag: r.TagName, Text: "No high-risk keywords detected", Weight: 1})
	}
	if out.score > 40 {
		out.score = 40
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func defaultRepo() string {
	if v := os.Getenv("HARBORWATCH_RELEASES_REPO"); v != "" {
		return v
	}
	return "Jellman86/HarborWatch"
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
}

type releaseAnalysis struct {
	score    int
	excerpts []gen.ReleaseExcerpt
}

type semVersion struct {
	major int
	minor int
	patch int
	pre   string
}

func parseLooseSemverTag(tag string) (semVersion, bool) {
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(strings.TrimPrefix(t, "v"), "V")
	if t == "" {
		return semVersion{}, false
	}

	core := t
	pre := ""
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		pre = core[i+1:]
		core = core[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semVersion{}, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semVersion{}, false
	}
	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return semVersion{}, false
		}
	}
	return semVersion{major: major, minor: minor, patch: patch, pre: pre}, true
}

func compareSemver(a, b semVersion) int {
	switch {
	case a.major != b.major:
		if a.major < b.major {
			return -1
		}
		return 1
	case a.minor != b.minor:
		if a.minor < b.minor {
			return -1
		}
		return 1
	case a.patch != b.patch:
		if a.patch < b.patch {
			return -1
		}
		return 1
	}

	// stable releases sort after pre-releases
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	if a.pre < b.pre {
		return -1
	}
	return 1
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(tag), "v"), "V"))
}

func findTagIndex(releases []githubRelease, tag string) int {
	want := normalizeTag(tag)
	if want == "" {
		return -1
	}
	for i, rel := range releases {
		if normalizeTag(rel.TagName) == want {
			return i
		}
	}
	return -1
}

func selectUpgradeReleases(releases []githubRelease, currentTag, targetTag string) ([]githubRelease, string) {
	curVer, curOK := parseLooseSemverTag(currentTag)
	tgtVer, tgtOK := parseLooseSemverTag(targetTag)
	if curOK && tgtOK {
		upgrade := compareSemver(tgtVer, curVer) >= 0
		var out []githubRelease
		for _, rel := range releases {
			relVer, ok := parseLooseSemverTag(rel.TagName)
			if !ok {
				continue
			}
			if upgrade {
				if compareSemver(relVer, curVer) > 0 && compareSemver(relVer, tgtVer) <= 0 {
					out = append(out, rel)
				}
			} else {
				if compareSemver(relVer, curVer) < 0 && compareSemver(relVer, tgtVer) >= 0 {
					out = append(out, rel)
				}
			}
		}
		if len(out) > 0 {
			return out, "semver_range"
		}
	}

	curIdx := findTagIndex(releases, currentTag)
	tgtIdx := findTagIndex(releases, targetTag)
	if curIdx >= 0 && tgtIdx >= 0 && curIdx != tgtIdx {
		start, end := curIdx, tgtIdx
		if start > end {
			start, end = end, start
		}
		out := append([]githubRelease(nil), releases[start:end+1]...)
		return out, "tag_order_fallback"
	}

	if len(releases) > 0 {
		max := len(releases)
		if max > 5 {
			max = 5
		}
		return append([]githubRelease(nil), releases[:max]...), "latest_fallback"
	}
	return nil, "empty"
}

func formatUpgradeContext(repo, currentTag, targetTag string, releases []githubRelease, reason string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Repository: %s\n", strings.TrimSpace(repo)))
	if strings.TrimSpace(currentTag) != "" {
		b.WriteString(fmt.Sprintf("Current tag: %s\n", strings.TrimSpace(currentTag)))
	}
	if strings.TrimSpace(targetTag) != "" {
		b.WriteString(fmt.Sprintf("Target tag: %s\n", strings.TrimSpace(targetTag)))
	}
	b.WriteString(fmt.Sprintf("Selection mode: %s\n", reason))

	if len(releases) == 0 {
		b.WriteString("No release notes available for selected range.\n")
		return strings.TrimSpace(b.String())
	}

	maxReleases := len(releases)
	if maxReleases > 8 {
		maxReleases = 8
	}
	b.WriteString(fmt.Sprintf("Release notes in scope (%d):\n", maxReleases))
	for i := 0; i < maxReleases; i++ {
		rel := releases[i]
		title := strings.TrimSpace(rel.Name)
		if title == "" {
			title = rel.TagName
		}
		published := strings.TrimSpace(rel.PublishedAt)
		if published != "" {
			if ts, err := time.Parse(time.RFC3339, published); err == nil {
				published = ts.Format("2006-01-02")
			}
		}
		if published != "" {
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", rel.TagName, published, title))
		} else {
			b.WriteString(fmt.Sprintf("- %s: %s\n", rel.TagName, title))
		}

		excerpt := releaseBodyExcerpt(rel.Body, 3, 420)
		if excerpt != "" {
			b.WriteString("  Notes:\n")
			for _, line := range strings.Split(excerpt, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				b.WriteString("  - " + trimmed + "\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func releaseBodyExcerpt(body string, maxLines int, maxChars int) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, 0, maxLines)
	used := 0
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimLeft(raw, "-*# "))
		if line == "" {
			continue
		}
		if len(line) > 180 {
			line = truncate(line, 180)
		}
		if used+len(line) > maxChars {
			break
		}
		out = append(out, line)
		used += len(line)
		if len(out) >= maxLines {
			break
		}
	}
	return strings.Join(out, "\n")
}
