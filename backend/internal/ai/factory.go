package ai

import (
	"os"
	"strings"
)

type ProviderConfig struct {
	Preferred      string
	OpenAIKey      string
	OpenAIModel    string
	AnthropicKey   string
	AnthropicModel string
	GeminiKey      string
	GeminiModel    string
}

func NewProviderFromConfig(cfg ProviderConfig) Provider {
	preferred := strings.ToLower(strings.TrimSpace(cfg.Preferred))
	switch preferred {
	case "openai":
		if cfg.OpenAIKey != "" {
			return NewOpenAIProvider(cfg.OpenAIKey, cfg.OpenAIModel)
		}
	case "anthropic", "claude":
		if cfg.AnthropicKey != "" {
			return NewAnthropicProvider(cfg.AnthropicKey, cfg.AnthropicModel)
		}
	case "gemini", "google":
		if cfg.GeminiKey != "" {
			return NewGeminiProvider(cfg.GeminiKey, cfg.GeminiModel)
		}
	}

	// Auto-detect when provider preference is unset or unavailable.
	if cfg.OpenAIKey != "" {
		return NewOpenAIProvider(cfg.OpenAIKey, cfg.OpenAIModel)
	}
	if cfg.AnthropicKey != "" {
		return NewAnthropicProvider(cfg.AnthropicKey, cfg.AnthropicModel)
	}
	if cfg.GeminiKey != "" {
		return NewGeminiProvider(cfg.GeminiKey, cfg.GeminiModel)
	}
	return nil
}

// NewProviderFromEnv creates an AI provider from standard environment variables.
func NewProviderFromEnv() Provider {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		geminiKey = os.Getenv("GOOGLE_API_KEY")
	}
	return NewProviderFromConfig(ProviderConfig{
		Preferred:      os.Getenv("AI_PROVIDER"),
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:    os.Getenv("OPENAI_MODEL"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel: os.Getenv("ANTHROPIC_MODEL"),
		GeminiKey:      geminiKey,
		GeminiModel:    os.Getenv("GEMINI_MODEL"),
	})
}
