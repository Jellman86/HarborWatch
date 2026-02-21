package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

type openAIProvider struct {
	client        *openai.Client
	model         string
	usageRecorder func(UsageRecord)
}

const defaultOpenAIModel = "gpt-5.2"

func NewOpenAIProvider(apiKey, model string) Provider {
	if model == "" {
		model = defaultOpenAIModel
	}
	return &openAIProvider{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (p *openAIProvider) Name() string { return "openai" }

func (p *openAIProvider) SetUsageRecorder(recorder func(UsageRecord)) {
	p.usageRecorder = recorder
}

func (p *openAIProvider) emitUsage(feature string, usage openai.Usage) {
	if p.usageRecorder == nil {
		return
	}
	p.usageRecorder(UsageRecord{
		Provider:     p.Name(),
		Model:        p.model,
		Feature:      feature,
		InputTokens:  int64(usage.PromptTokens),
		OutputTokens: int64(usage.CompletionTokens),
		TotalTokens:  int64(usage.TotalTokens),
	})
}

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
	p.emitUsage("release_analysis", resp.Usage)

	result, err := parseAnalysisResult(resp.Choices[0].Message.Content)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return result, nil
}

func (p *openAIProvider) AnalyzeFleet(ctx context.Context, inventory string) (string, error) {
	prompt := `Analyze the following Docker fleet inventory and provide proactive maintenance, security, and optimization advice. 
Identify containers that may need updates, those that are stopped and might be orphaned, and suggest general best practices based on the deployment mix.
Provide a technical, concise summary in Markdown format.

Fleet Inventory:
` + inventory

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai completion failed: %w", err)
	}
	p.emitUsage("fleet_advice", resp.Usage)

	return resp.Choices[0].Message.Content, nil
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
	p.emitUsage("compose_audit", resp.Usage)

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
	p.emitUsage("metrics_analysis", resp.Usage)

	return resp.Choices[0].Message.Content, nil
}

func (p *openAIProvider) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (HealthAssessment, error) {
	prompt := `Assess runtime health from container logs.
Return ONLY valid JSON with this schema:
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
		return HealthAssessment{}, fmt.Errorf("openai completion failed: %w", err)
	}
	p.emitUsage("health_logs", resp.Usage)

	result, err := parseHealthAssessment(resp.Choices[0].Message.Content)
	if err != nil {
		return HealthAssessment{}, fmt.Errorf("failed to parse AI health response: %w", err)
	}
	return result, nil
}
