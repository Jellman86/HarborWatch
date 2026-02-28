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

const defaultAnthropicModel = "claude-sonnet-4-5"

type anthropicProvider struct {
	apiKey        string
	model         string
	httpClient    *http.Client
	usageRecorder func(UsageRecord)
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

func (p *anthropicProvider) SetUsageRecorder(recorder func(UsageRecord)) {
	p.usageRecorder = recorder
}

func (p *anthropicProvider) emitUsage(feature string, inputTokens, outputTokens int64) {
	if p.usageRecorder == nil {
		return
	}
	total := inputTokens + outputTokens
	p.usageRecorder(UsageRecord{
		Provider:     p.Name(),
		Model:        p.model,
		Feature:      feature,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  total,
	})
}

func (p *anthropicProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	prompt := `Analyze the following software release notes for a Docker container update.
Identify breaking changes, configuration format updates, migration risk, and critical security fixes.
Return ONLY valid JSON with this exact schema:
{
  "risk_score": (int 0-100),
  "risk_level": ("Low"|"Medium"|"High"|"Critical"),
  "summary": (string),
  "breaking_changes": [string],
  "action_required": (boolean, set to true ONLY if manual user intervention or config changes are required before updating)
}

Release notes:
` + notes

	text, inputTokens, outputTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return AnalysisResult{}, err
	}
	p.emitUsage("release_analysis", inputTokens, outputTokens)
	result, err := parseAnalysisResult(text)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse anthropic response: %w", err)
	}
	return result, nil
}

func (p *anthropicProvider) AnalyzeFleet(ctx context.Context, inventory string) (string, error) {
	prompt := `Analyze the following Docker fleet inventory and provide proactive maintenance, security, and optimization advice.
Identify containers that may need updates, those that are stopped and might be orphaned, and suggest general best practices based on the deployment mix.
Return a concise, technical report in Markdown.

Fleet inventory:
` + inventory
	text, inputTokens, outputTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	p.emitUsage("fleet_advice", inputTokens, outputTokens)
	return text, nil
}

func (p *anthropicProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	prompt := `Audit the following docker-compose.yml for security and resilience.
Focus on privileged mode, unsafe mounts, missing resource limits, network exposure, weak restart policies, and secret handling.
Return a concise remediation report in Markdown.

Compose file:
` + yaml
	text, inputTokens, outputTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	p.emitUsage("compose_audit", inputTokens, outputTokens)
	return text, nil
}

func (p *anthropicProvider) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	prompt := fmt.Sprintf(`Analyze these performance metrics for Docker container %q.
Identify memory leak patterns, CPU saturation, I/O bottlenecks, and right-sizing recommendations.
Return concise recommendations in Markdown.

Metrics JSON:
%s`, id, string(metricsJSON))
	text, inputTokens, outputTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	p.emitUsage("metrics_analysis", inputTokens, outputTokens)
	return text, nil
}

func (p *anthropicProvider) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (HealthAssessment, error) {
	prompt := `Assess runtime health from container logs.
Return ONLY valid JSON:
{
  "healthy": (boolean),
  "confidence": (int 0-100),
  "summary": (string),
  "concerns": [string],
  "recommendation": (string)
}

Container ID: ` + containerID + `

Recent logs:
` + logs
	text, inputTokens, outputTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return HealthAssessment{}, err
	}
	p.emitUsage("health_logs", inputTokens, outputTokens)
	result, err := parseHealthAssessment(text)
	if err != nil {
		return HealthAssessment{}, fmt.Errorf("failed to parse anthropic health response: %w", err)
	}
	return result, nil
}

func (p *anthropicProvider) generate(ctx context.Context, prompt string) (string, int64, int64, error) {
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
		return "", 0, 0, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, 0, fmt.Errorf("anthropic API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", 0, 0, fmt.Errorf("decode anthropic response: %w", err)
	}

	var b strings.Builder
	for _, part := range parsed.Content {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", 0, 0, fmt.Errorf("anthropic response was empty")
	}
	inputTokens := parsed.Usage.InputTokens + parsed.Usage.CacheCreationInputTokens + parsed.Usage.CacheReadInputTokens
	return out, inputTokens, parsed.Usage.OutputTokens, nil
}
