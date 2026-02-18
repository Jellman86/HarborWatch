package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGeminiModel = "gemini-2.5-flash"

type geminiProvider struct {
	apiKey        string
	model         string
	httpClient    *http.Client
	usageRecorder func(UsageRecord)
}

func NewGeminiProvider(apiKey, model string) Provider {
	if model == "" {
		model = defaultGeminiModel
	}
	return &geminiProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *geminiProvider) Name() string { return "gemini" }

func (p *geminiProvider) SetUsageRecorder(recorder func(UsageRecord)) {
	p.usageRecorder = recorder
}

func (p *geminiProvider) emitUsage(feature string, inputTokens, outputTokens, totalTokens int64) {
	if p.usageRecorder == nil {
		return
	}
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens
	}
	p.usageRecorder(UsageRecord{
		Provider:     p.Name(),
		Model:        p.model,
		Feature:      feature,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	})
}

func (p *geminiProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	prompt := `Analyze these Docker release notes for upgrade risk.
Return ONLY valid JSON using this exact schema:
{
  "risk_score": (int 0-100),
  "risk_level": ("Low"|"Medium"|"High"|"Critical"),
  "summary": (string),
  "breaking_changes": [string],
  "action_required": (boolean)
}

Release notes:
` + notes

	text, inputTokens, outputTokens, totalTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return AnalysisResult{}, err
	}
	p.emitUsage("release_analysis", inputTokens, outputTokens, totalTokens)
	result, err := parseAnalysisResult(text)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse gemini response: %w", err)
	}
	return result, nil
}

func (p *geminiProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	prompt := `Audit this docker-compose.yml for security and reliability.
Flag privileged mode, host networking, weak volume mounts, missing limits, and poor restart policies.
Return a concise markdown remediation report.

Compose file:
` + yaml
	text, inputTokens, outputTokens, totalTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	p.emitUsage("compose_audit", inputTokens, outputTokens, totalTokens)
	return text, nil
}

func (p *geminiProvider) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	prompt := fmt.Sprintf(`Analyze performance metrics for Docker container %q.
Identify memory leaks, CPU pressure, and right-sizing actions.
Return concise markdown recommendations.

Metrics JSON:
%s`, id, string(metricsJSON))
	text, inputTokens, outputTokens, totalTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	p.emitUsage("metrics_analysis", inputTokens, outputTokens, totalTokens)
	return text, nil
}

func (p *geminiProvider) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (HealthAssessment, error) {
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
	text, inputTokens, outputTokens, totalTokens, err := p.generate(ctx, prompt)
	if err != nil {
		return HealthAssessment{}, err
	}
	p.emitUsage("health_logs", inputTokens, outputTokens, totalTokens)
	result, err := parseHealthAssessment(text)
	if err != nil {
		return HealthAssessment{}, fmt.Errorf("failed to parse gemini health response: %w", err)
	}
	return result, nil
}

func (p *geminiProvider) generate(ctx context.Context, prompt string) (string, int64, int64, int64, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", url.PathEscape(p.model), url.QueryEscape(p.apiKey))
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":      0.1,
			"maxOutputTokens":  1600,
			"responseMimeType": "text/plain",
		},
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, 0, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, 0, 0, fmt.Errorf("gemini API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", 0, 0, 0, fmt.Errorf("decode gemini response: %w", err)
	}

	var b strings.Builder
	for _, c := range parsed.Candidates {
		for _, p := range c.Content.Parts {
			b.WriteString(p.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", 0, 0, 0, fmt.Errorf("gemini response was empty")
	}
	return out, parsed.UsageMetadata.PromptTokenCount, parsed.UsageMetadata.CandidatesTokenCount, parsed.UsageMetadata.TotalTokenCount, nil
}
