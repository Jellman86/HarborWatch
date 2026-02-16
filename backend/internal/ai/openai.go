package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type openAIProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIProvider(apiKey, model string) Provider {
	if model == "" {
		model = "gpt-4o-mini" // Economical default for analysis
	}
	return &openAIProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *openAIProvider) Name() string { return "openai" }

func (p *openAIProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	prompt := `Analyze the following software release notes for a Docker container update. 
Identify if there are any breaking changes, configuration format updates, or critical security fixes.
Respond ONLY in JSON format with the following structure:
{
  "risk_score": (int 0-100),
  "risk_level": ("Low", "Medium", "High", "Critical"),
  "summary": (brief string),
  "breaking_changes": [string array],
  "action_required": (boolean)
}

Release Notes:
` + notes

	return p.callChat(ctx, prompt)
}

func (p *openAIProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	prompt := `Audit the following docker-compose.yml file for security risks, missing resource limits, or insecure practices. 
Provide a concise, professional summary of findings and recommended fixes.

Compose File:
` + yaml

	return p.callRaw(ctx, prompt)
}

// Internal helpers for OpenAI API
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

func (p *openAIProvider) callChat(ctx context.Context, prompt string) (AnalysisResult, error) {
	reqBody, _ := json.Marshal(openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return AnalysisResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AnalysisResult{}, fmt.Errorf("openai error: status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AnalysisResult{}, err
	}

	if len(result.Choices) == 0 {
		return AnalysisResult{}, errors.New("openai returned no choices")
	}

	var analysis AnalysisResult
	// Clean markdown backticks if present
	content := result.Choices[0].Message.Content
	content = cleanJSON(content)
	
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return analysis, nil
}

func (p *openAIProvider) callRaw(ctx context.Context, prompt string) (string, error) {
	reqBody, _ := json.Marshal(openAIRequest{
		Model: p.model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Choices) == 0 {
		return "", errors.New("no response from AI")
	}

	return result.Choices[0].Message.Content, nil
}

func cleanJSON(s string) string {
	s = bytes.NewBufferString(s).String()
	if len(s) > 7 && s[:7] == "```json" {
		s = s[7:]
		if len(s) > 3 && s[len(s)-3:] == "```" {
			s = s[:len(s)-3]
		}
	}
	return s
}
