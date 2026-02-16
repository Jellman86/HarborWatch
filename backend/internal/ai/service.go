package ai

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// RiskLevel represents the AI's assessment of an update.
type RiskLevel string

const (
	RiskLow      RiskLevel = "Low"
	RiskMedium   RiskLevel = "Medium"
	RiskHigh     RiskLevel = "High"
	RiskCritical RiskLevel = "Critical"
)

// AnalysisResult is the structured output from an AI analysis.
type AnalysisResult struct {
	RiskScore       int      `json:"risk_score"`
	RiskLevel       RiskLevel `json:"risk_level"`
	Summary         string   `json:"summary"`
	BreakingChanges []string `json:"breaking_changes"`
	ActionRequired  bool     `json:"action_required"`
}

// Provider defines the interface for different AI models (OpenAI, Anthropic, etc).
type Provider interface {
	Name() string
	AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error)
	AuditCompose(ctx context.Context, yaml string) (string, error)
	AnalyzeMetrics(ctx context.Context, containerID string, metrics []any) (string, error)
}

// Service coordinates AI operations.
type Service struct {
	provider Provider
}

func NewService(p Provider) *Service {
	return &Service{provider: p}
}

func (s *Service) HasProvider() bool {
	return s.provider != nil
}

func (s *Service) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	if s.provider == nil {
		return AnalysisResult{}, errors.New("no AI provider configured")
	}
	return s.provider.AnalyzeReleaseNotes(ctx, notes)
}

func (s *Service) AuditCompose(ctx context.Context, yamlStr string) (string, error) {
	if s.provider == nil {
		return "", errors.New("no AI provider configured")
	}

	// Pre-validate YAML structure using established Go module
	var body any
	if err := yaml.Unmarshal([]byte(yamlStr), &body); err != nil {
		return "", fmt.Errorf("invalid YAML syntax: %w", err)
	}

	return s.provider.AuditCompose(ctx, yamlStr)
}

func (s *Service) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	if s.provider == nil {
		return "", errors.New("no AI provider configured")
	}
	return s.provider.AnalyzeMetrics(ctx, id, metrics)
}
