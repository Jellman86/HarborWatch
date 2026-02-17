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
	apiKey     string
	model      string
	httpClient *http.Client
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

	text, err := p.generate(ctx, prompt)
	if err != nil {
		return AnalysisResult{}, err
	}
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
	return p.generate(ctx, prompt)
}

func (p *geminiProvider) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	prompt := fmt.Sprintf(`Analyze performance metrics for Docker container %q.
Identify memory leaks, CPU pressure, and right-sizing actions.
Return concise markdown recommendations.

Metrics JSON:
%s`, id, string(metricsJSON))
	return p.generate(ctx, prompt)
}

func (p *geminiProvider) generate(ctx context.Context, prompt string) (string, error) {
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
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}

	var b strings.Builder
	for _, c := range parsed.Candidates {
		for _, p := range c.Content.Parts {
			b.WriteString(p.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("gemini response was empty")
	}
	return out, nil
}
