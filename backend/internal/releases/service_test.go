package releases

import "testing"

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
	if validRepo("bad") {
		t.Fatal("expected invalid repo")
	}
}
