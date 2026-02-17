package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type aiModelList struct {
	Models []string `json:"models"`
	Error  string   `json:"error,omitempty"`
}

type aiModelCatalogResponse struct {
	Providers map[string]aiModelList `json:"providers"`
}

func loadProviderModelCatalog(ctx context.Context, st settings.Settings, providerFilter string) aiModelCatalogResponse {
	out := aiModelCatalogResponse{
		Providers: map[string]aiModelList{},
	}
	providerFilter = strings.ToLower(strings.TrimSpace(providerFilter))
	if providerFilter == "" || providerFilter == "openai" {
		out.Providers["openai"] = fetchOpenAIModels(ctx, st.OpenAIKey)
	}
	if providerFilter == "" || providerFilter == "anthropic" {
		out.Providers["anthropic"] = fetchAnthropicModels(ctx, st.AnthropicKey)
	}
	if providerFilter == "" || providerFilter == "gemini" {
		key := st.GeminiKey
		out.Providers["gemini"] = fetchGeminiModels(ctx, key)
	}
	return out
}

func fetchOpenAIModels(ctx context.Context, apiKey string) aiModelList {
	if strings.TrimSpace(apiKey) == "" {
		return aiModelList{Error: "OpenAI API key is not configured"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return aiModelList{Error: fmt.Sprintf("OpenAI models API status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return aiModelList{Error: fmt.Sprintf("decode OpenAI models response: %v", err)}
	}

	models := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		// Favor text-generation models suitable for HarborWatch prompts.
		if strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "o") || strings.Contains(id, "chatgpt") {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	models = reverseStrings(models)
	return aiModelList{Models: dedupeStrings(models)}
}

func fetchAnthropicModels(ctx context.Context, apiKey string) aiModelList {
	if strings.TrimSpace(apiKey) == "" {
		return aiModelList{Error: "Anthropic API key is not configured"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return aiModelList{Error: fmt.Sprintf("Anthropic models API status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return aiModelList{Error: fmt.Sprintf("decode Anthropic models response: %v", err)}
	}

	models := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	models = reverseStrings(models)
	return aiModelList{Models: dedupeStrings(models)}
}

func fetchGeminiModels(ctx context.Context, apiKey string) aiModelList {
	if strings.TrimSpace(apiKey) == "" {
		return aiModelList{Error: "Gemini API key is not configured"}
	}
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models?key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return aiModelList{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return aiModelList{Error: fmt.Sprintf("Gemini models API status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))}
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return aiModelList{Error: fmt.Sprintf("decode Gemini models response: %v", err)}
	}

	models := make([]string, 0, len(payload.Models))
	for _, m := range payload.Models {
		id := strings.TrimSpace(strings.TrimPrefix(m.Name, "models/"))
		if strings.Contains(id, "gemini") {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	models = reverseStrings(models)
	return aiModelList{Models: dedupeStrings(models)}
}

func reverseStrings(in []string) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[len(in)-1-i]
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func testProviderModel(ctx context.Context, provider, model string, st settings.Settings) (string, string, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)

	var p ai.Provider
	switch provider {
	case "openai":
		key := strings.TrimSpace(st.OpenAIKey)
		if key == "" {
			return provider, model, "", fmt.Errorf("OpenAI key is not configured")
		}
		if model == "" {
			model = strings.TrimSpace(st.OpenAIModel)
		}
		p = ai.NewOpenAIProvider(key, model)
	case "anthropic":
		key := strings.TrimSpace(st.AnthropicKey)
		if key == "" {
			return provider, model, "", fmt.Errorf("Anthropic key is not configured")
		}
		if model == "" {
			model = strings.TrimSpace(st.AnthropicModel)
		}
		p = ai.NewAnthropicProvider(key, model)
	case "gemini":
		key := strings.TrimSpace(st.GeminiKey)
		if key == "" {
			return provider, model, "", fmt.Errorf("Gemini key is not configured")
		}
		if model == "" {
			model = strings.TrimSpace(st.GeminiModel)
		}
		p = ai.NewGeminiProvider(key, model)
	default:
		return provider, model, "", fmt.Errorf("unsupported provider: %s", provider)
	}
	if p == nil {
		return provider, model, "", fmt.Errorf("provider is not available")
	}
	svc := ai.NewService(p)
	ctxTest, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	result, err := svc.AnalyzeReleaseNotes(ctxTest, "Validation release notes: bugfixes only.")
	if err != nil {
		return provider, model, "", err
	}
	return provider, model, nilIfEmpty(result.Summary), nil
}

func nilIfEmpty(s string) string {
	out := strings.TrimSpace(s)
	if out == "" {
		return "Provider responded successfully."
	}
	return out
}
