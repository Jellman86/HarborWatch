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

func (s *Service) fetchReleases(ctx context.Context, repo string) ([]githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases?per_page=10", nil)
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
