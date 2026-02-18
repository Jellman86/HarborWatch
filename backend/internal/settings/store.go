package settings

import (
	"context"
	"database/sql"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Settings struct {
	// Notifications
	DiscordWebhookURL string `json:"discordWebhookUrl"`
	DiscordEnabled    bool   `json:"discordEnabled"`

	// API Keys / Integrations
	PortainerURL     string `json:"portainerUrl"`
	PortainerApiKey  string `json:"portainerApiKey"`
	PortainerEnabled bool   `json:"portainerEnabled"`
	AIEnabled        bool   `json:"aiEnabled"`
	AIProvider       string `json:"aiProvider"`
	OpenAIKey        string `json:"openaiKey"`
	OpenAIModel      string `json:"openaiModel"`
	AnthropicKey     string `json:"anthropicKey"`
	AnthropicModel   string `json:"anthropicModel"`
	GeminiKey        string `json:"geminiKey"`
	GeminiModel      string `json:"geminiModel"`
	AIPricingJSON    string `json:"aiPricingJson"`

	// System
	InstanceURL                 string `json:"instanceUrl"`
	ValidateURLPattern          string `json:"validateUrlPattern"`
	UIAnimationsEnabled         bool   `json:"uiAnimationsEnabled"`
	AutomationIgnoredContainers string `json:"automationIgnoredContainers"`
	MalwareIgnoredMounts        string `json:"malwareIgnoredMounts"`

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
		AIEnabled:                   true,
		DiscordEnabled:              true,
		PortainerEnabled:            true,
		UIAnimationsEnabled:         true,
		AutomationIgnoredContainers: "harborwatch",
		EnvironmentOverrides:        make(map[string]bool),
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
		case "discord_enabled":
			st.DiscordEnabled = parseStoredBool(value, st.DiscordEnabled)
		case "portainer_url":
			st.PortainerURL = value
		case "portainer_api_key":
			st.PortainerApiKey = value
		case "portainer_enabled":
			st.PortainerEnabled = parseStoredBool(value, st.PortainerEnabled)
		case "ai_enabled":
			st.AIEnabled = parseStoredBool(value, st.AIEnabled)
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
		case "ai_pricing_json":
			st.AIPricingJSON = value
		case "instance_url":
			st.InstanceURL = value
		case "validate_url_pattern":
			st.ValidateURLPattern = value
		case "ui_animations_enabled":
			st.UIAnimationsEnabled = parseStoredBool(value, st.UIAnimationsEnabled)
		case "automation_ignored_containers":
			st.AutomationIgnoredContainers = value
		case "malware_ignored_mounts":
			st.MalwareIgnoredMounts = value
		}
	}

	// 2. Override with Environment Variables (Priority)
	envMap := map[string]struct {
		ptr    *string
		envKey string
	}{
		"discordWebhookUrl":           {&st.DiscordWebhookURL, "DISCORD_WEBHOOK_URL"},
		"portainerUrl":                {&st.PortainerURL, "PORTAINER_URL"},
		"portainerApiKey":             {&st.PortainerApiKey, "PORTAINER_API_KEY"},
		"aiProvider":                  {&st.AIProvider, "AI_PROVIDER"},
		"openaiKey":                   {&st.OpenAIKey, "OPENAI_API_KEY"},
		"openaiModel":                 {&st.OpenAIModel, "OPENAI_MODEL"},
		"anthropicKey":                {&st.AnthropicKey, "ANTHROPIC_API_KEY"},
		"anthropicModel":              {&st.AnthropicModel, "ANTHROPIC_MODEL"},
		"geminiKey":                   {&st.GeminiKey, "GEMINI_API_KEY"},
		"geminiModel":                 {&st.GeminiModel, "GEMINI_MODEL"},
		"aiPricingJson":               {&st.AIPricingJSON, "HW_AI_PRICING_JSON"},
		"instanceUrl":                 {&st.InstanceURL, "HW_INSTANCE_URL"},
		"validateUrlPattern":          {&st.ValidateURLPattern, "HW_VALIDATE_PATTERN"},
		"automationIgnoredContainers": {&st.AutomationIgnoredContainers, "HW_AUTOMATION_IGNORE_CONTAINERS"},
		"malwareIgnoredMounts":        {&st.MalwareIgnoredMounts, "HW_MALWARE_IGNORE_MOUNTS"},
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

	boolEnvMap := map[string]struct {
		ptr    *bool
		envKey string
	}{
		"aiEnabled":           {&st.AIEnabled, "HW_AI_ENABLED"},
		"discordEnabled":      {&st.DiscordEnabled, "HW_DISCORD_ENABLED"},
		"portainerEnabled":    {&st.PortainerEnabled, "HW_PORTAINER_ENABLED"},
		"uiAnimationsEnabled": {&st.UIAnimationsEnabled, "HW_UI_ANIMATIONS_ENABLED"},
	}
	for jsonKey, mapping := range boolEnvMap {
		if val := strings.TrimSpace(os.Getenv(mapping.envKey)); val != "" {
			*mapping.ptr = parseStoredBool(val, *mapping.ptr)
			st.EnvironmentOverrides[jsonKey] = true
		}
	}

	st.AutomationIgnoredContainers = normalizeContainerIgnoreList(st.AutomationIgnoredContainers)
	st.MalwareIgnoredMounts = normalizeDelimitedList(st.MalwareIgnoredMounts)

	return st, nil
}

func (s *Store) Save(ctx context.Context, st Settings) error {
	// Only save values that are NOT currently overridden by environment variables
	// Load current state to check overrides
	current, _ := s.Get(ctx)
	st.AutomationIgnoredContainers = normalizeContainerIgnoreList(st.AutomationIgnoredContainers)
	st.MalwareIgnoredMounts = normalizeDelimitedList(st.MalwareIgnoredMounts)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keys := map[string]string{
		"discord_webhook_url":           st.DiscordWebhookURL,
		"discord_enabled":               boolString(st.DiscordEnabled),
		"portainer_url":                 st.PortainerURL,
		"portainer_api_key":             st.PortainerApiKey,
		"portainer_enabled":             boolString(st.PortainerEnabled),
		"ai_enabled":                    boolString(st.AIEnabled),
		"ai_provider":                   st.AIProvider,
		"openai_key":                    st.OpenAIKey,
		"openai_model":                  st.OpenAIModel,
		"anthropic_key":                 st.AnthropicKey,
		"anthropic_model":               st.AnthropicModel,
		"gemini_key":                    st.GeminiKey,
		"gemini_model":                  st.GeminiModel,
		"ai_pricing_json":               st.AIPricingJSON,
		"instance_url":                  st.InstanceURL,
		"validate_url_pattern":          st.ValidateURLPattern,
		"ui_animations_enabled":         boolString(st.UIAnimationsEnabled),
		"automation_ignored_containers": st.AutomationIgnoredContainers,
		"malware_ignored_mounts":        st.MalwareIgnoredMounts,
	}

	jsonToDbKey := map[string]string{
		"discordWebhookUrl":           "discord_webhook_url",
		"discordEnabled":              "discord_enabled",
		"portainerUrl":                "portainer_url",
		"portainerApiKey":             "portainer_api_key",
		"portainerEnabled":            "portainer_enabled",
		"aiEnabled":                   "ai_enabled",
		"aiProvider":                  "ai_provider",
		"openaiKey":                   "openai_key",
		"openaiModel":                 "openai_model",
		"anthropicKey":                "anthropic_key",
		"anthropicModel":              "anthropic_model",
		"geminiKey":                   "gemini_key",
		"geminiModel":                 "gemini_model",
		"aiPricingJson":               "ai_pricing_json",
		"instanceUrl":                 "instance_url",
		"validateUrlPattern":          "validate_url_pattern",
		"uiAnimationsEnabled":         "ui_animations_enabled",
		"automationIgnoredContainers": "automation_ignored_containers",
		"malwareIgnoredMounts":        "malware_ignored_mounts",
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

func parseStoredBool(value string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func normalizeContainerIgnoreList(raw string) string {
	tokens := normalizeDelimitedList(raw)
	for _, token := range splitDelimitedList(tokens) {
		if strings.EqualFold(token, "harborwatch") {
			return tokens
		}
	}
	if tokens == "" {
		return "harborwatch"
	}
	return tokens + ", harborwatch"
}

func normalizeDelimitedList(raw string) string {
	parts := splitDelimitedList(raw)
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, part)
	}
	return strings.Join(out, ", ")
}

func splitDelimitedList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		part := strings.TrimSpace(field)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
