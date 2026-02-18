package releases

import (
	"strings"
	"testing"
)

func TestAnalyzeReleaseTextRisk(t *testing.T) {
	r := githubRelease{
		TagName: "v1.2.0",
		Body:    `Breaking change: API endpoint /foo removed\nMigration required for database change\nSecurity fix for CVE-2026-0001`,
	}
	got := analyzeReleaseText(r)
	if got.score <= 0 {
		t.Fatalf("expected positive score, got %d", got.score)
	}
	if len(got.excerpts) == 0 {
		t.Fatal("expected excerpts")
	}
}

func TestValidRepo(t *testing.T) {
	if !validRepo("owner/repo") {
		t.Fatal("expected valid repo")
	}
	if !validRepo("https://gitlab.com/group/subgroup/project/-/releases") {
		t.Fatal("expected gitlab URL to be valid")
	}
	if validRepo("bad") {
		t.Fatal("expected invalid repo")
	}
}

func TestParseRepoSpecGitLabReleasesURL(t *testing.T) {
	spec, err := parseRepoSpec("https://gitlab.com/group/subgroup/project/-/releases")
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if spec.provider != "gitlab" {
		t.Fatalf("expected provider gitlab, got %q", spec.provider)
	}
	if spec.path != "group/subgroup/project" {
		t.Fatalf("expected gitlab path extraction, got %q", spec.path)
	}
}

func TestParseRepoSpecGiteaURL(t *testing.T) {
	spec, err := parseRepoSpec("https://code.example.com/acme/api/releases")
	if err != nil {
		t.Fatalf("expected parse success, got %v", err)
	}
	if spec.path != "acme/api" {
		t.Fatalf("expected owner/repo extraction, got %q", spec.path)
	}
}

func TestSelectUpgradeReleases_SemverRange(t *testing.T) {
	rels := []githubRelease{
		{TagName: "v1.4.0"},
		{TagName: "v1.3.0"},
		{TagName: "v1.2.0"},
		{TagName: "v1.1.0"},
	}
	selected, mode := selectUpgradeReleases(rels, "v1.1.0", "v1.3.0")
	if mode != "semver_range" {
		t.Fatalf("expected semver_range mode, got %q", mode)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 releases in range, got %d", len(selected))
	}
	if selected[0].TagName != "v1.3.0" || selected[1].TagName != "v1.2.0" {
		t.Fatalf("unexpected range selection: %#v", selected)
	}
}

func TestSelectUpgradeReleases_FallbackLatest(t *testing.T) {
	rels := []githubRelease{
		{TagName: "release-2026-02"},
		{TagName: "release-2026-01"},
	}
	selected, mode := selectUpgradeReleases(rels, "old", "new")
	if mode != "latest_fallback" {
		t.Fatalf("expected latest_fallback mode, got %q", mode)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 fallback releases, got %d", len(selected))
	}
}

func TestFormatUpgradeContextIncludesNotes(t *testing.T) {
	ctx := formatUpgradeContext("owner/repo", "v1.0.0", "v1.1.0", []githubRelease{
		{
			TagName: "v1.1.0",
			Name:    "Release 1.1",
			Body:    "Breaking change: config path moved\nSecurity fixes",
		},
	}, "semver_range")

	if !strings.Contains(ctx, "Selection mode: semver_range") {
		t.Fatalf("expected selection mode in context, got %q", ctx)
	}
	if !strings.Contains(ctx, "Breaking change: config path moved") {
		t.Fatalf("expected note excerpt in context, got %q", ctx)
	}
}
