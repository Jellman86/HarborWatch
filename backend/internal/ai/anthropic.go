package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultAnthropicModel = "claude-sonnet-4-20250514"

type anthropicProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewAnthropicProvider(apiKey, model string) Provider {
	if model == "" {
		model = defaultAnthropicModel
	}
	return &anthropicProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	prompt := `Analyze the following software release notes for a Docker container update.
Identify breaking changes, configuration format updates, migration risk, and critical security fixes.
Return ONLY valid JSON with this exact schema:
{
  "risk_score": (int 0-100),
  "risk_level": ("Low"|"Medium"|"High"|"Critical"),
  "summary": (string),
  "breaking_changes": [string],
  "action_required": (boolean)
}

Release notes:
` + notes

	text, err := p.generate(ctx, prompt)
	if err != nil {
		return AnalysisResult{}, err
	}
	result, err := parseAnalysisResult(text)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse anthropic response: %w", err)
	}
	return result, nil
}

func (p *anthropicProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	prompt := `Audit the following docker-compose.yml for security and resilience.
Focus on privileged mode, unsafe mounts, missing resource limits, network exposure, weak restart policies, and secret handling.
Return a concise remediation report in Markdown.

Compose file:
` + yaml
	return p.generate(ctx, prompt)
}

func (p *anthropicProvider) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	prompt := fmt.Sprintf(`Analyze these performance metrics for Docker container %q.
Identify memory leak patterns, CPU saturation, I/O bottlenecks, and right-sizing recommendations.
Return concise recommendations in Markdown.

Metrics JSON:
%s`, id, string(metricsJSON))
	return p.generate(ctx, prompt)
}

func (p *anthropicProvider) generate(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"model":       p.model,
		"max_tokens":  1600,
		"temperature": 0.1,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}

	var b strings.Builder
	for _, part := range parsed.Content {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("anthropic response was empty")
	}
	return out, nil
}
