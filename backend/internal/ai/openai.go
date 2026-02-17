package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type openAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey, model string) Provider {
	if model == "" {
		model = openai.GPT4oMini
	}
	return &openAIProvider{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (p *openAIProvider) Name() string { return "openai" }

func (p *openAIProvider) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	prompt := `Analyze the following software release notes for a Docker container update. 
Identify if there are any breaking changes, configuration format updates, or critical security fixes.
Respond ONLY in valid JSON format with the following structure:
{
  "risk_score": (int 0-100),
  "risk_level": ("Low", "Medium", "High", "Critical"),
  "summary": (brief string),
  "breaking_changes": [string array],
  "action_required": (boolean)
}

Release Notes:
` + notes

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("openai completion failed: %w", err)
	}

	result, err := parseAnalysisResult(resp.Choices[0].Message.Content)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return result, nil
}

func (p *openAIProvider) AuditCompose(ctx context.Context, yaml string) (string, error) {
	prompt := `Audit the following docker-compose.yml file for security risks, missing resource limits, or insecure practices. 
Provide a concise, professional summary of findings and recommended fixes in Markdown format.

Compose File:
` + yaml

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai completion failed: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func (p *openAIProvider) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	metricsJSON, _ := json.Marshal(metrics)
	prompt := fmt.Sprintf(`Analyze the following performance metrics for Docker container "%s".
Identify potential issues such as memory leaks (steady linear growth), inefficient CPU usage, or inappropriate resource limits.
Provide a concise, technical summary and optimization recommendations in Markdown format.

Metrics Data (JSON):
%s`, id, string(metricsJSON))

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai completion failed: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}
