package ai

import (
	"os"
)

// NewProviderFromEnv creates an AI provider based on environment variables.
func NewProviderFromEnv() Provider {
	// For now, we support OpenAI as the primary target.
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey != "" {
		return NewOpenAIProvider(apiKey, os.Getenv("OPENAI_MODEL"))
	}

	// Future: Support Anthropic, Gemini, or Ollama
	return nil
}
