package httpapi

import (
	"context"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type composeAuditHistoryStore interface {
	SaveComposeAudit(ctx context.Context, rec ai.ComposeAuditRecord) (ai.ComposeAuditRecord, error)
	ListComposeAudits(ctx context.Context, containerID string, limit, offset int) ([]ai.ComposeAuditRecordSummary, error)
	GetComposeAudit(ctx context.Context, id string) (ai.ComposeAuditRecord, error)
}

type composeAuditResponse struct {
	RecordID         string `json:"recordId,omitempty"`
	Provider         string `json:"provider,omitempty"`
	Model            string `json:"model,omitempty"`
	Config           string `json:"config"`
	Analysis         string `json:"analysis"`
	AnalysisMarkdown string `json:"analysisMarkdown"`
	AnalysisHTML     string `json:"analysisHtml"`
	Persisted        bool   `json:"persisted"`
	PersistError     string `json:"persistError,omitempty"`
	CreatedAt        int64  `json:"createdAt,omitempty"`
}

const (
	defaultOpenAIModelForAudit    = "gpt-5.2"
	defaultAnthropicModelForAudit = "claude-sonnet-4-5"
	defaultGeminiModelForAudit    = "gemini-2.5-flash"
)

func resolveAuditProviderModel(st settings.Settings) (string, string) {
	type candidate struct {
		provider string
		key      string
		model    string
		defModel string
	}

	normalize := func(provider, model, def string) (string, string) {
		p := strings.ToLower(strings.TrimSpace(provider))
		m := strings.TrimSpace(model)
		if m == "" {
			m = def
		}
		return p, m
	}

	openai := candidate{provider: "openai", key: st.OpenAIKey, model: st.OpenAIModel, defModel: defaultOpenAIModelForAudit}
	anthropic := candidate{provider: "anthropic", key: st.AnthropicKey, model: st.AnthropicModel, defModel: defaultAnthropicModelForAudit}
	gemini := candidate{provider: "gemini", key: st.GeminiKey, model: st.GeminiModel, defModel: defaultGeminiModelForAudit}

	pref := strings.ToLower(strings.TrimSpace(st.AIProvider))
	switch pref {
	case "openai":
		if strings.TrimSpace(openai.key) != "" {
			return normalize(openai.provider, openai.model, openai.defModel)
		}
	case "anthropic", "claude":
		if strings.TrimSpace(anthropic.key) != "" {
			return normalize("anthropic", anthropic.model, anthropic.defModel)
		}
	case "gemini", "google":
		if strings.TrimSpace(gemini.key) != "" {
			return normalize("gemini", gemini.model, gemini.defModel)
		}
	}

	if strings.TrimSpace(openai.key) != "" {
		return normalize(openai.provider, openai.model, openai.defModel)
	}
	if strings.TrimSpace(anthropic.key) != "" {
		return normalize("anthropic", anthropic.model, anthropic.defModel)
	}
	if strings.TrimSpace(gemini.key) != "" {
		return normalize("gemini", gemini.model, gemini.defModel)
	}

	return "unknown", "unknown"
}

func containerDisplayName(c gen.ContainerSummary) string {
	for _, n := range c.Names {
		name := strings.TrimSpace(strings.TrimPrefix(n, "/"))
		if name != "" {
			return name
		}
	}
	return strings.TrimSpace(c.ID)
}
