package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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
	RiskScore       int       `json:"risk_score"`
	RiskLevel       RiskLevel `json:"risk_level"`
	Summary         string    `json:"summary"`
	BreakingChanges []string  `json:"breaking_changes"`
	ActionRequired  bool      `json:"action_required"`
}

type HealthAssessment struct {
	Healthy        bool     `json:"healthy"`
	Confidence     int      `json:"confidence"`
	Summary        string   `json:"summary"`
	Concerns       []string `json:"concerns"`
	Recommendation string   `json:"recommendation"`
}

type UsageRecord struct {
	Timestamp    int64  `json:"timestamp"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Feature      string `json:"feature"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
}

type UsageBreakdown struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Feature      string `json:"feature"`
	Calls        int64  `json:"calls"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
}

type UsageDaily struct {
	Day          string `json:"day"`
	Calls        int64  `json:"calls"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
}

type UsageSummary struct {
	From         int64            `json:"from"`
	To           int64            `json:"to"`
	Calls        int64            `json:"calls"`
	InputTokens  int64            `json:"inputTokens"`
	OutputTokens int64            `json:"outputTokens"`
	TotalTokens  int64            `json:"totalTokens"`
	Breakdown    []UsageBreakdown `json:"breakdown"`
	Daily        []UsageDaily     `json:"daily"`
}

type UsageStore interface {
	RecordUsage(ctx context.Context, rec UsageRecord) error
	SummaryUsage(ctx context.Context, from, to int64) (UsageSummary, error)
}

type usageRecorderProvider interface {
	SetUsageRecorder(recorder func(UsageRecord))
}

// Provider defines the interface for different AI models (OpenAI, Anthropic, etc).
type Provider interface {
	Name() string
	AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error)
	AuditCompose(ctx context.Context, yaml string) (string, error)
	AnalyzeMetrics(ctx context.Context, containerID string, metrics []any) (string, error)
	AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (HealthAssessment, error)
}

// Service coordinates AI operations.
type Service struct {
	mu         sync.RWMutex
	provider   Provider
	usageStore UsageStore
}

func NewService(p Provider) *Service {
	return &Service{provider: p}
}

func (s *Service) HasProvider() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider != nil
}

func (s *Service) SetProvider(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = p
	s.bindUsageRecorderLocked(p)
}

func (s *Service) SetUsageStore(store UsageStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageStore = store
	s.bindUsageRecorderLocked(s.provider)
}

func (s *Service) currentProvider() Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider
}

func (s *Service) currentUsageStore() UsageStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usageStore
}

func (s *Service) AnalyzeReleaseNotes(ctx context.Context, notes string) (AnalysisResult, error) {
	provider := s.currentProvider()
	if provider == nil {
		return AnalysisResult{}, errors.New("no AI provider configured")
	}
	return provider.AnalyzeReleaseNotes(ctx, notes)
}

func (s *Service) AuditCompose(ctx context.Context, yamlStr string) (string, error) {
	provider := s.currentProvider()
	if provider == nil {
		return "", errors.New("no AI provider configured")
	}

	// Pre-validate YAML structure using established Go module
	var body any
	if err := yaml.Unmarshal([]byte(yamlStr), &body); err != nil {
		return "", fmt.Errorf("invalid YAML syntax: %w", err)
	}

	return provider.AuditCompose(ctx, yamlStr)
}

func (s *Service) AnalyzeMetrics(ctx context.Context, id string, metrics []any) (string, error) {
	provider := s.currentProvider()
	if provider == nil {
		return "", errors.New("no AI provider configured")
	}
	return provider.AnalyzeMetrics(ctx, id, metrics)
}

func (s *Service) AnalyzeHealthLogs(ctx context.Context, containerID string, logs string) (HealthAssessment, error) {
	provider := s.currentProvider()
	if provider == nil {
		return HealthAssessment{}, errors.New("no AI provider configured")
	}
	return provider.AnalyzeHealthLogs(ctx, containerID, logs)
}

func (s *Service) UsageSummary(ctx context.Context, from, to int64) (UsageSummary, error) {
	store := s.currentUsageStore()
	if store == nil {
		return UsageSummary{
			From:      from,
			To:        to,
			Breakdown: []UsageBreakdown{},
			Daily:     []UsageDaily{},
		}, nil
	}
	return store.SummaryUsage(ctx, from, to)
}

func (s *Service) bindUsageRecorderLocked(provider Provider) {
	if provider == nil {
		return
	}
	hookable, ok := provider.(usageRecorderProvider)
	if !ok {
		return
	}
	if s.usageStore == nil {
		hookable.SetUsageRecorder(nil)
		return
	}
	hookable.SetUsageRecorder(func(rec UsageRecord) {
		s.recordUsage(rec)
	})
}

func (s *Service) recordUsage(rec UsageRecord) {
	store := s.currentUsageStore()
	if store == nil {
		return
	}
	rec.Provider = strings.ToLower(strings.TrimSpace(rec.Provider))
	rec.Model = strings.TrimSpace(rec.Model)
	rec.Feature = strings.TrimSpace(rec.Feature)
	if rec.Provider == "" {
		rec.Provider = "unknown"
	}
	if rec.Model == "" {
		rec.Model = "unknown"
	}
	if rec.Feature == "" {
		rec.Feature = "unknown"
	}
	if rec.TotalTokens <= 0 {
		rec.TotalTokens = rec.InputTokens + rec.OutputTokens
	}
	if rec.TotalTokens <= 0 {
		return
	}
	if rec.Timestamp <= 0 {
		rec.Timestamp = time.Now().UTC().Unix()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = store.RecordUsage(ctx, rec)
}
