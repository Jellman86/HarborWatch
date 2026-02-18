package ai

import (
	"strings"
	"testing"
)

func TestNormalizeAndRenderMarkdown_UnwrapFenceAndSanitize(t *testing.T) {
	raw := "```markdown\n# Report\n\n<script>alert('x')</script>\n\n- item one\n```\n"
	md, html, err := NormalizeAndRenderMarkdown(raw)
	if err != nil {
		t.Fatalf("NormalizeAndRenderMarkdown failed: %v", err)
	}
	if strings.Contains(md, "```") {
		t.Fatalf("expected outer markdown fence to be removed, got: %q", md)
	}
	if strings.Contains(strings.ToLower(html), "<script") {
		t.Fatalf("expected script tag to be sanitized, got: %q", html)
	}
	if !strings.Contains(strings.ToLower(html), "<h1") {
		t.Fatalf("expected rendered heading HTML, got: %q", html)
	}
}

func TestNormalizeMarkdown_LeavesPlainTextStable(t *testing.T) {
	raw := "Line one\r\nLine two"
	out := NormalizeMarkdown(raw)
	if out != "Line one\nLine two" {
		t.Fatalf("unexpected normalized output: %q", out)
	}
}
