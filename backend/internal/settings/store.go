package settings

import (
	"context"
	"database/sql"
	"os"

	_ "modernc.org/sqlite"
)

type Settings struct {
	// Notifications
	DiscordWebhookURL string `json:"discordWebhookUrl"`
	GotifyURL         string `json:"gotifyUrl"`
	GotifyToken       string `json:"gotifyToken"`

	// API Keys / Integrations
	PortainerURL    string `json:"portainerUrl"`
	PortainerApiKey string `json:"portainerApiKey"`
	AIProvider      string `json:"aiProvider"`
	OpenAIKey       string `json:"openaiKey"`
	OpenAIModel     string `json:"openaiModel"`
	AnthropicKey    string `json:"anthropicKey"`
	AnthropicModel  string `json:"anthropicModel"`
	GeminiKey       string `json:"geminiKey"`
	GeminiModel     string `json:"geminiModel"`

	// System
	InstanceURL        string `json:"instanceUrl"`
	ValidateURLPattern string `json:"validateUrlPattern"`

	// Metadata (read-only info for UI)
	EnvironmentOverrides map[string]bool `json:"environmentOverrides"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) GetDB() *sql.DB { return s.db }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

func (s *Store) Get(ctx context.Context) (Settings, error) {
	st := Settings{
		EnvironmentOverrides: make(map[string]bool),
	}

	// 1. Load from Database
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM app_settings")
	if err != nil {
		return st, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return st, err
		}
		switch key {
		case "discord_webhook_url":
			st.DiscordWebhookURL = value
		case "gotify_url":
			st.GotifyURL = value
		case "gotify_token":
			st.GotifyToken = value
		case "portainer_url":
			st.PortainerURL = value
		case "portainer_api_key":
			st.PortainerApiKey = value
		case "openai_key":
			st.OpenAIKey = value
		case "openai_model":
			st.OpenAIModel = value
		case "ai_provider":
			st.AIProvider = value
		case "anthropic_key":
			st.AnthropicKey = value
		case "anthropic_model":
			st.AnthropicModel = value
		case "gemini_key":
			st.GeminiKey = value
		case "gemini_model":
			st.GeminiModel = value
		case "instance_url":
			st.InstanceURL = value
		case "validate_url_pattern":
			st.ValidateURLPattern = value
		}
	}

	// 2. Override with Environment Variables (Priority)
	envMap := map[string]struct {
		ptr    *string
		envKey string
	}{
		"discordWebhookUrl":  {&st.DiscordWebhookURL, "DISCORD_WEBHOOK_URL"},
		"gotifyUrl":          {&st.GotifyURL, "GOTIFY_URL"},
		"gotifyToken":        {&st.GotifyToken, "GOTIFY_TOKEN"},
		"portainerUrl":       {&st.PortainerURL, "PORTAINER_URL"},
		"portainerApiKey":    {&st.PortainerApiKey, "PORTAINER_API_KEY"},
		"aiProvider":         {&st.AIProvider, "AI_PROVIDER"},
		"openaiKey":          {&st.OpenAIKey, "OPENAI_API_KEY"},
		"openaiModel":        {&st.OpenAIModel, "OPENAI_MODEL"},
		"anthropicKey":       {&st.AnthropicKey, "ANTHROPIC_API_KEY"},
		"anthropicModel":     {&st.AnthropicModel, "ANTHROPIC_MODEL"},
		"geminiKey":          {&st.GeminiKey, "GEMINI_API_KEY"},
		"geminiModel":        {&st.GeminiModel, "GEMINI_MODEL"},
		"instanceUrl":        {&st.InstanceURL, "HW_INSTANCE_URL"},
		"validateUrlPattern": {&st.ValidateURLPattern, "HW_VALIDATE_PATTERN"},
	}

	for jsonKey, mapping := range envMap {
		if val := os.Getenv(mapping.envKey); val != "" {
			*mapping.ptr = val
			st.EnvironmentOverrides[jsonKey] = true
		}
	}
	if !st.EnvironmentOverrides["geminiKey"] {
		if val := os.Getenv("GOOGLE_API_KEY"); val != "" {
			st.GeminiKey = val
			st.EnvironmentOverrides["geminiKey"] = true
		}
	}

	return st, nil
}

func (s *Store) Save(ctx context.Context, st Settings) error {
	// Only save values that are NOT currently overridden by environment variables
	// Load current state to check overrides
	current, _ := s.Get(ctx)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keys := map[string]string{
		"discord_webhook_url":  st.DiscordWebhookURL,
		"gotify_url":           st.GotifyURL,
		"gotify_token":         st.GotifyToken,
		"portainer_url":        st.PortainerURL,
		"portainer_api_key":    st.PortainerApiKey,
		"ai_provider":          st.AIProvider,
		"openai_key":           st.OpenAIKey,
		"openai_model":         st.OpenAIModel,
		"anthropic_key":        st.AnthropicKey,
		"anthropic_model":      st.AnthropicModel,
		"gemini_key":           st.GeminiKey,
		"gemini_model":         st.GeminiModel,
		"instance_url":         st.InstanceURL,
		"validate_url_pattern": st.ValidateURLPattern,
	}

	jsonToDbKey := map[string]string{
		"discordWebhookUrl":  "discord_webhook_url",
		"gotifyUrl":          "gotify_url",
		"gotifyToken":        "gotify_token",
		"portainerUrl":       "portainer_url",
		"portainerApiKey":    "portainer_api_key",
		"aiProvider":         "ai_provider",
		"openaiKey":          "openai_key",
		"openaiModel":        "openai_model",
		"anthropicKey":       "anthropic_key",
		"anthropicModel":     "anthropic_model",
		"geminiKey":          "gemini_key",
		"geminiModel":        "gemini_model",
		"instanceUrl":        "instance_url",
		"validateUrlPattern": "validate_url_pattern",
	}

	for jsonKey, dbKey := range jsonToDbKey {
		// If it's overridden by ENV, we don't allow saving to DB for that key
		// (or we can save it but ENV will still win on next Get)
		// For clarity, we'll only save if NOT overridden.
		if current.EnvironmentOverrides[jsonKey] {
			continue
		}

		val := keys[dbKey]
		_, err = tx.ExecContext(ctx, `
INSERT INTO app_settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value
`, dbKey, val)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
