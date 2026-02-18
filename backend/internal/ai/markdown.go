package ai

import (
	"bytes"
	stdhtml "html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	mdhtml "github.com/yuin/goldmark/renderer/html"
)

var markdownSanitizer = bluemonday.UGCPolicy()

// NormalizeAndRenderMarkdown normalizes raw model output and returns safe HTML for UI rendering.
func NormalizeAndRenderMarkdown(raw string) (string, string, error) {
	normalized := NormalizeMarkdown(raw)
	if normalized == "" {
		return "", "", nil
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(mdhtml.WithHardWraps()),
	)
	var out bytes.Buffer
	if err := md.Convert([]byte(normalized), &out); err != nil {
		return normalized, "", err
	}

	safeHTML := strings.TrimSpace(markdownSanitizer.Sanitize(out.String()))
	if safeHTML == "" {
		safeHTML = "<p>" + stdhtml.EscapeString(normalized) + "</p>"
	}
	return normalized, safeHTML, nil
}

// NormalizeMarkdown applies lightweight cleanup to model markdown output.
func NormalizeMarkdown(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}

	// Models sometimes wrap full responses in a markdown code fence.
	for i := 0; i < 2; i++ {
		unwrapped := unwrapOuterFence(normalized)
		if unwrapped == normalized {
			break
		}
		normalized = unwrapped
	}

	return strings.TrimSpace(normalized)
}

func unwrapOuterFence(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return text
	}

	first := strings.TrimSpace(lines[0])
	marker := ""
	switch {
	case strings.HasPrefix(first, "```"):
		marker = "```"
	case strings.HasPrefix(first, "~~~"):
		marker = "~~~"
	default:
		return text
	}

	last := len(lines) - 1
	for last > 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	if strings.TrimSpace(lines[last]) != marker {
		return text
	}

	return strings.TrimSpace(strings.Join(lines[1:last], "\n"))
}
