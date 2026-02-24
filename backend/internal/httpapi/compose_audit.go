package httpapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
	"gopkg.in/yaml.v3"
)

type composeAuditHistoryStore interface {
	SaveComposeAudit(ctx context.Context, rec ai.ComposeAuditRecord) (ai.ComposeAuditRecord, error)
	ListComposeAudits(ctx context.Context, containerID string, limit, offset int) ([]ai.ComposeAuditRecordSummary, error)
	GetComposeAudit(ctx context.Context, id string) (ai.ComposeAuditRecord, error)
}

type composeAuditResponse struct {
	RecordID             string `json:"recordId,omitempty"`
	Provider             string `json:"provider,omitempty"`
	Model                string `json:"model,omitempty"`
	Config               string `json:"config"`
	ConfigRequestedScope string `json:"configRequestedScope,omitempty"`
	ConfigAppliedScope   string `json:"configAppliedScope,omitempty"`
	ConfigMode           string `json:"configMode,omitempty"`
	ComposeProject       string `json:"composeProject,omitempty"`
	ComposeService       string `json:"composeService,omitempty"`
	ConfigNote           string `json:"configNote,omitempty"`
	Analysis             string `json:"analysis"`
	AnalysisMarkdown     string `json:"analysisMarkdown"`
	AnalysisHTML         string `json:"analysisHtml"`
	Persisted            bool   `json:"persisted"`
	PersistError         string `json:"persistError,omitempty"`
	CreatedAt            int64  `json:"createdAt,omitempty"`
}

type composeConfigPreviewResponse struct {
	Config               string `json:"config"`
	ConfigRequestedScope string `json:"configRequestedScope"`
	ConfigAppliedScope   string `json:"configAppliedScope"`
	ConfigMode           string `json:"configMode"`
	ComposeProject       string `json:"composeProject,omitempty"`
	ComposeService       string `json:"composeService,omitempty"`
	ConfigNote           string `json:"configNote,omitempty"`
}

type composeConfigView struct {
	Config         string
	RequestedScope string
	AppliedScope   string
	Mode           string
	ComposeProject string
	ComposeService string
	Note           string
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

func normalizeComposeConfigScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "full":
		return "full"
	default:
		return "service"
	}
}

func buildComposeConfigView(fullConfig string, summary gen.ContainerSummary, requestedScope string) composeConfigView {
	view := composeConfigView{
		Config:         fullConfig,
		RequestedScope: normalizeComposeConfigScope(requestedScope),
		AppliedScope:   "full",
		Mode:           "classic-docker",
	}
	if strings.TrimSpace(view.Config) == "" {
		view.Config = "# No configuration data available"
		view.Note = "No compose or reconstructed configuration data could be retrieved."
		return view
	}

	if summary.Labels == nil {
		view.Note = "Classic Docker container detected; showing reconstructed effective config."
		return view
	}

	view.ComposeProject = strings.TrimSpace(summary.Labels["com.docker.compose.project"])
	view.ComposeService = strings.TrimSpace(summary.Labels["com.docker.compose.service"])
	if view.ComposeProject == "" || view.ComposeService == "" {
		view.Note = "Classic Docker container detected; showing reconstructed effective config."
		return view
	}

	view.Mode = "compose"
	if view.RequestedScope == "full" {
		view.Note = "Full compose configuration selected for display and AI audit."
		return view
	}

	scoped, err := deriveComposeServiceScopedYAML(fullConfig, view.ComposeService)
	if err != nil {
		view.Note = fmt.Sprintf("Could not derive service-only compose section; falling back to full compose (%v).", err)
		return view
	}
	view.Config = scoped
	view.AppliedScope = "service"
	view.Note = "Showing the compose section derived for this container's service."
	return view
}

func deriveComposeServiceScopedYAML(fullConfig, serviceName string) (string, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "", fmt.Errorf("missing compose service name")
	}

	var root map[string]any
	if err := yaml.Unmarshal([]byte(fullConfig), &root); err != nil {
		return "", fmt.Errorf("parse yaml: %w", err)
	}

	servicesRaw, ok := root["services"].(map[string]any)
	if !ok || len(servicesRaw) == 0 {
		return "", fmt.Errorf("services map not found")
	}
	serviceDef, ok := servicesRaw[serviceName]
	if !ok {
		return "", fmt.Errorf("service %q not found", serviceName)
	}

	out := map[string]any{}
	if v, ok := root["version"]; ok {
		out["version"] = v
	}
	if v, ok := root["name"]; ok {
		out["name"] = v
	}
	out["services"] = map[string]any{serviceName: serviceDef}

	referencedNetworks := extractServiceNetworkRefs(serviceDef)
	if len(referencedNetworks) > 0 {
		if top, ok := root["networks"].(map[string]any); ok {
			selected := map[string]any{}
			for _, name := range referencedNetworks {
				if def, ok := top[name]; ok {
					selected[name] = def
				}
			}
			if len(selected) > 0 {
				out["networks"] = selected
			}
		}
	}

	referencedVolumes := extractServiceNamedVolumeRefs(serviceDef)
	if len(referencedVolumes) > 0 {
		if top, ok := root["volumes"].(map[string]any); ok {
			selected := map[string]any{}
			for _, name := range referencedVolumes {
				if def, ok := top[name]; ok {
					selected[name] = def
				}
			}
			if len(selected) > 0 {
				out["volumes"] = selected
			}
		}
	}

	buf, err := yaml.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal scoped yaml: %w", err)
	}
	return string(buf), nil
}

func extractServiceNetworkRefs(serviceDef any) []string {
	m, ok := serviceDef.(map[string]any)
	if !ok {
		return nil
	}
	networks, ok := m["networks"]
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]struct{}{}
	switch v := networks.(type) {
	case []any:
		for _, item := range v {
			name := strings.TrimSpace(fmt.Sprint(item))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	case map[string]any:
		for name := range v {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

func extractServiceNamedVolumeRefs(serviceDef any) []string {
	m, ok := serviceDef.(map[string]any)
	if !ok {
		return nil
	}
	volumes, ok := m["volumes"].([]any)
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]struct{}{}
	for _, item := range volumes {
		name := ""
		switch v := item.(type) {
		case string:
			parts := strings.SplitN(v, ":", 2)
			if len(parts) == 0 {
				continue
			}
			src := strings.TrimSpace(parts[0])
			if src == "" || strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
				continue
			}
			name = src
		case map[string]any:
			if typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(v["type"]))); typ != "" && typ != "volume" {
				continue
			}
			src := strings.TrimSpace(fmt.Sprint(v["source"]))
			if src == "" || strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") {
				continue
			}
			name = src
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
