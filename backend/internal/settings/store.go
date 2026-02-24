package settings

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type Settings struct {
	// Notifications
	DiscordWebhookURL string `json:"discordWebhookUrl"`
	DiscordEnabled    bool   `json:"discordEnabled"`

	// API Keys / Integrations
	PortainerURL         string `json:"portainerUrl"`
	PortainerApiKey      string `json:"portainerApiKey"`
	PortainerEnabled     bool   `json:"portainerEnabled"`
	AIEnabled            bool   `json:"aiEnabled"`
	AIProvider           string `json:"aiProvider"`
	AIBlockRiskThreshold int    `json:"aiBlockRiskThreshold"`
	OpenAIKey            string `json:"openaiKey"`
	OpenAIModel          string `json:"openaiModel"`
	AnthropicKey         string `json:"anthropicKey"`
	AnthropicModel       string `json:"anthropicModel"`
	GeminiKey            string `json:"geminiKey"`
	GeminiModel          string `json:"geminiModel"`
	AIPricingJSON        string `json:"aiPricingJson"`

	// System
	InstanceURL                        string `json:"instanceUrl"`
	ValidateURLPattern                 string `json:"validateUrlPattern"`
	UIAnimationsEnabled                bool   `json:"uiAnimationsEnabled"`
	AutomationIgnoredContainers        string `json:"automationIgnoredContainers"`
	MalwareIgnoredMounts               string `json:"malwareIgnoredMounts"`
	AutoUpgradeMaxConcurrency          int    `json:"autoUpgradeMaxConcurrency"`
	AutoUpgradeMinRetryMinutes         int    `json:"autoUpgradeMinRetryMinutes"`
	TrivySweepMode                     string `json:"trivySweepMode"`
	ClamAVSnapshotMaxBytes             int64  `json:"clamavSnapshotMaxBytes"`
	DataRetentionDays                  int    `json:"dataRetentionDays"`
	RetentionLogsDays                  int    `json:"retentionLogsDays"`
	RetentionMetricsDays               int    `json:"retentionMetricsDays"`
	RetentionScanResultsDays           int    `json:"retentionScanResultsDays"`
	RetentionScanJobsDays              int    `json:"retentionScanJobsDays"`
	RetentionUpdateRunsDays            int    `json:"retentionUpdateRunsDays"`
	RetentionComposeAuditDays          int    `json:"retentionComposeAuditDays"`
	RetentionAIUsageDays               int    `json:"retentionAIUsageDays"`
	MetricsNormalized                  bool   `json:"metricsNormalized"`
	GlobalBypassAI                     bool   `json:"globalBypassAi"`
	GlobalSkipHealthCheck              bool   `json:"globalSkipHealthCheck"`
	DefaultValidateMode                string `json:"defaultValidateMode"`
	DefaultValidateTimeoutSec          int    `json:"defaultValidateTimeoutSec"`
	DefaultValidateIntervalSec         int    `json:"defaultValidateIntervalSec"`
	DefaultAIValidateLogs              bool   `json:"defaultAiValidateLogs"`
	DefaultAutoRollback                bool   `json:"defaultAutoRollback"`
	DefaultRestartOnUnhealthy          bool   `json:"defaultRestartOnUnhealthy"`
	UnhealthyAutoRemediationEnabled    bool   `json:"unhealthyAutoRemediationEnabled"`
	UnhealthyRestartCooldownSecDefault int    `json:"unhealthyRestartCooldownSecDefault"`
	MaxRestartsPerWindow               int    `json:"maxRestartsPerWindow"`
	AITestingPassed                    bool   `json:"aiTestingPassed"`
	PortainerTestingPassed             bool   `json:"portainerTestingPassed"`

	// Metadata (read-only info for UI)
	DashboardLastUpdateDetectedCount int             `json:"dashboardLastUpdateDetectedCount"`
	DashboardLastUpdateCheckAt       int64           `json:"dashboardLastUpdateCheckAt"`
	EnvironmentOverrides             map[string]bool `json:"environmentOverrides"`
}

type Store struct {
	db *sql.DB
}

const (
	dashboardUpdateDetectedCountKey = "dashboard_update_detected_count"
	dashboardUpdateCheckedAtKey     = "dashboard_update_checked_at"
)

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) GetDB() *sql.DB { return s.db }

func (s *Store) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='app_settings' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("app_settings table missing; run schema migrations before settings store init")
	}
	if err != nil {
		return fmt.Errorf("verify app_settings table: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context) (Settings, error) {
	st := Settings{
		AIEnabled:                          true,
		AIBlockRiskThreshold:               80,
		DiscordEnabled:                     true,
		PortainerEnabled:                   true,
		UIAnimationsEnabled:                true,
		AutomationIgnoredContainers:        "harborwatch",
		AutoUpgradeMaxConcurrency:          1,
		AutoUpgradeMinRetryMinutes:         60,
		TrivySweepMode:                     "running-only",
		ClamAVSnapshotMaxBytes:             2 << 30,
		DataRetentionDays:                  30,
		RetentionLogsDays:                  30,
		RetentionMetricsDays:               14,
		RetentionScanResultsDays:           30,
		RetentionScanJobsDays:              30,
		RetentionUpdateRunsDays:            90,
		RetentionComposeAuditDays:          90,
		RetentionAIUsageDays:               180,
		UnhealthyAutoRemediationEnabled:    true,
		UnhealthyRestartCooldownSecDefault: 300,
		MaxRestartsPerWindow:               3,
		MetricsNormalized:                  true,
		DefaultValidateMode:                "both",
		DefaultValidateTimeoutSec:          45,
		DefaultValidateIntervalSec:         2,
		DefaultAIValidateLogs:              false,
		DefaultAutoRollback:                true,
		DefaultRestartOnUnhealthy:          false,
		EnvironmentOverrides:               make(map[string]bool),
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
		case "ai_block_risk_threshold":
			st.AIBlockRiskThreshold = parseStoredInt(value, st.AIBlockRiskThreshold, 0, 100)
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
		case "auto_upgrade_max_concurrency":
			st.AutoUpgradeMaxConcurrency = parseStoredInt(value, st.AutoUpgradeMaxConcurrency, 1, 20)
		case "auto_upgrade_min_retry_minutes":
			st.AutoUpgradeMinRetryMinutes = parseStoredInt(value, st.AutoUpgradeMinRetryMinutes, 1, 24*60)
		case "trivy_sweep_mode":
			st.TrivySweepMode = normalizeTrivySweepMode(value)
		case "clamav_snapshot_max_bytes":
			st.ClamAVSnapshotMaxBytes = parseStoredInt64(value, st.ClamAVSnapshotMaxBytes, 1, 32<<30)
		case "data_retention_days":
			st.DataRetentionDays = parseStoredInt(value, st.DataRetentionDays, 1, 3650)
		case "retention_logs_days":
			st.RetentionLogsDays = parseStoredInt(value, st.RetentionLogsDays, 1, 3650)
		case "retention_metrics_days":
			st.RetentionMetricsDays = parseStoredInt(value, st.RetentionMetricsDays, 1, 3650)
		case "retention_scan_results_days":
			st.RetentionScanResultsDays = parseStoredInt(value, st.RetentionScanResultsDays, 1, 3650)
		case "retention_scan_jobs_days":
			st.RetentionScanJobsDays = parseStoredInt(value, st.RetentionScanJobsDays, 1, 3650)
		case "retention_update_runs_days":
			st.RetentionUpdateRunsDays = parseStoredInt(value, st.RetentionUpdateRunsDays, 1, 3650)
		case "retention_compose_audit_days":
			st.RetentionComposeAuditDays = parseStoredInt(value, st.RetentionComposeAuditDays, 1, 3650)
		case "retention_ai_usage_days":
			st.RetentionAIUsageDays = parseStoredInt(value, st.RetentionAIUsageDays, 1, 3650)
		case "metrics_normalized":
			st.MetricsNormalized = parseStoredBool(value, st.MetricsNormalized)
		case "global_bypass_ai":
			st.GlobalBypassAI = parseStoredBool(value, st.GlobalBypassAI)
		case "global_skip_health_check":
			st.GlobalSkipHealthCheck = parseStoredBool(value, st.GlobalSkipHealthCheck)
		case "default_validate_mode":
			st.DefaultValidateMode = normalizeValidateModeValue(value)
		case "default_validate_timeout_sec":
			st.DefaultValidateTimeoutSec = parseStoredInt(value, st.DefaultValidateTimeoutSec, 1, 3600)
		case "default_validate_interval_sec":
			st.DefaultValidateIntervalSec = parseStoredInt(value, st.DefaultValidateIntervalSec, 1, 300)
		case "default_ai_validate_logs":
			st.DefaultAIValidateLogs = parseStoredBool(value, st.DefaultAIValidateLogs)
		case "default_auto_rollback":
			st.DefaultAutoRollback = parseStoredBool(value, st.DefaultAutoRollback)
		case "default_restart_on_unhealthy":
			st.DefaultRestartOnUnhealthy = parseStoredBool(value, st.DefaultRestartOnUnhealthy)
		case "unhealthy_auto_remediation_enabled":
			st.UnhealthyAutoRemediationEnabled = parseStoredBool(value, st.UnhealthyAutoRemediationEnabled)
		case "unhealthy_restart_cooldown_sec_default":
			st.UnhealthyRestartCooldownSecDefault = parseStoredInt(value, st.UnhealthyRestartCooldownSecDefault, 0, 3600*24)
		case "max_restarts_per_window":
			st.MaxRestartsPerWindow = parseStoredInt(value, st.MaxRestartsPerWindow, 0, 100)
		case "ai_testing_passed":
			st.AITestingPassed = parseStoredBool(value, st.AITestingPassed)
		case "portainer_testing_passed":
			st.PortainerTestingPassed = parseStoredBool(value, st.PortainerTestingPassed)
		case dashboardUpdateDetectedCountKey:
			st.DashboardLastUpdateDetectedCount = parseStoredInt(value, 0, 0, 1000000)
		case dashboardUpdateCheckedAtKey:
			st.DashboardLastUpdateCheckAt = parseStoredInt64(value, 0, 0, 1<<62)
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
		"trivySweepMode":              {&st.TrivySweepMode, "HW_TRIVY_SWEEP_MODE"},
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
		"aiEnabled":             {&st.AIEnabled, "HW_AI_ENABLED"},
		"discordEnabled":        {&st.DiscordEnabled, "HW_DISCORD_ENABLED"},
		"portainerEnabled":      {&st.PortainerEnabled, "HW_PORTAINER_ENABLED"},
		"uiAnimationsEnabled":   {&st.UIAnimationsEnabled, "HW_UI_ANIMATIONS_ENABLED"},
		"metricsNormalized":     {&st.MetricsNormalized, "HW_METRICS_NORMALIZED"},
		"globalBypassAi":        {&st.GlobalBypassAI, "HW_GLOBAL_BYPASS_AI"},
		"globalSkipHealthCheck": {&st.GlobalSkipHealthCheck, "HW_GLOBAL_SKIP_HEALTH_CHECK"},
	}
	for jsonKey, mapping := range boolEnvMap {
		if val := strings.TrimSpace(os.Getenv(mapping.envKey)); val != "" {
			*mapping.ptr = parseStoredBool(val, *mapping.ptr)
			st.EnvironmentOverrides[jsonKey] = true
		}
	}
	intEnvMap := map[string]struct {
		ptr      *int
		envKey   string
		minValue int
		maxValue int
	}{
		"aiBlockRiskThreshold":       {&st.AIBlockRiskThreshold, "HW_AI_BLOCK_RISK_THRESHOLD", 0, 100},
		"autoUpgradeMaxConcurrency":  {&st.AutoUpgradeMaxConcurrency, "HW_AUTO_UPGRADE_MAX_CONCURRENCY", 1, 20},
		"autoUpgradeMinRetryMinutes": {&st.AutoUpgradeMinRetryMinutes, "HW_AUTO_UPGRADE_MIN_RETRY_MINUTES", 1, 24 * 60},
	}
	for jsonKey, mapping := range intEnvMap {
		if val := strings.TrimSpace(os.Getenv(mapping.envKey)); val != "" {
			*mapping.ptr = parseStoredInt(val, *mapping.ptr, mapping.minValue, mapping.maxValue)
			st.EnvironmentOverrides[jsonKey] = true
		}
	}
	if val := strings.TrimSpace(os.Getenv("HW_CLAMAV_SNAPSHOT_MAX_BYTES")); val != "" {
		st.ClamAVSnapshotMaxBytes = parseStoredInt64(val, st.ClamAVSnapshotMaxBytes, 1, 32<<30)
		st.EnvironmentOverrides["clamavSnapshotMaxBytes"] = true
	}

	retentionEnvMap := map[string]struct {
		ptr    *int
		envKey string
	}{
		"dataRetentionDays":         {&st.DataRetentionDays, "HW_DATA_RETENTION_DAYS"},
		"retentionLogsDays":         {&st.RetentionLogsDays, "HW_RETENTION_LOG_DAYS"},
		"retentionMetricsDays":      {&st.RetentionMetricsDays, "HW_RETENTION_METRICS_DAYS"},
		"retentionScanResultsDays":  {&st.RetentionScanResultsDays, "HW_RETENTION_SCAN_RESULTS_DAYS"},
		"retentionScanJobsDays":     {&st.RetentionScanJobsDays, "HW_RETENTION_SCAN_JOBS_DAYS"},
		"retentionUpdateRunsDays":   {&st.RetentionUpdateRunsDays, "HW_RETENTION_UPDATE_RUNS_DAYS"},
		"retentionComposeAuditDays": {&st.RetentionComposeAuditDays, "HW_RETENTION_COMPOSE_AUDIT_DAYS"},
		"retentionAIUsageDays":      {&st.RetentionAIUsageDays, "HW_RETENTION_AI_USAGE_DAYS"},
	}
	for jsonKey, mapping := range retentionEnvMap {
		if val := strings.TrimSpace(os.Getenv(mapping.envKey)); val != "" {
			*mapping.ptr = parseStoredInt(val, *mapping.ptr, 1, 3650)
			st.EnvironmentOverrides[jsonKey] = true
		}
	}

	st.TrivySweepMode = normalizeTrivySweepMode(st.TrivySweepMode)
	st.DefaultValidateMode = normalizeValidateModeValue(st.DefaultValidateMode)
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
	st.AIBlockRiskThreshold = parseStoredInt(strconv.Itoa(st.AIBlockRiskThreshold), 80, 0, 100)
	st.AutoUpgradeMaxConcurrency = parseStoredInt(strconv.Itoa(st.AutoUpgradeMaxConcurrency), 1, 1, 20)
	st.AutoUpgradeMinRetryMinutes = parseStoredInt(strconv.Itoa(st.AutoUpgradeMinRetryMinutes), 60, 1, 24*60)
	st.TrivySweepMode = normalizeTrivySweepMode(st.TrivySweepMode)
	st.ClamAVSnapshotMaxBytes = parseStoredInt64(strconv.FormatInt(st.ClamAVSnapshotMaxBytes, 10), 2<<30, 1, 32<<30)
	st.DefaultValidateMode = normalizeValidateModeValue(st.DefaultValidateMode)
	st.DefaultValidateTimeoutSec = parseStoredInt(strconv.Itoa(st.DefaultValidateTimeoutSec), 45, 1, 3600)
	st.DefaultValidateIntervalSec = parseStoredInt(strconv.Itoa(st.DefaultValidateIntervalSec), 2, 1, 300)
	st.DataRetentionDays = parseStoredInt(strconv.Itoa(st.DataRetentionDays), 30, 1, 3650)
	st.RetentionLogsDays = st.DataRetentionDays
	st.RetentionMetricsDays = st.DataRetentionDays
	st.RetentionScanResultsDays = st.DataRetentionDays
	st.RetentionScanJobsDays = st.DataRetentionDays
	st.RetentionUpdateRunsDays = st.DataRetentionDays
	st.RetentionComposeAuditDays = st.DataRetentionDays
	st.RetentionAIUsageDays = st.DataRetentionDays

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keys := map[string]string{
		"discord_webhook_url":                    st.DiscordWebhookURL,
		"discord_enabled":                        boolString(st.DiscordEnabled),
		"portainer_url":                          st.PortainerURL,
		"portainer_api_key":                      st.PortainerApiKey,
		"portainer_enabled":                      boolString(st.PortainerEnabled),
		"ai_enabled":                             boolString(st.AIEnabled),
		"ai_provider":                            st.AIProvider,
		"ai_block_risk_threshold":                intString(st.AIBlockRiskThreshold),
		"openai_key":                             st.OpenAIKey,
		"openai_model":                           st.OpenAIModel,
		"anthropic_key":                          st.AnthropicKey,
		"anthropic_model":                        st.AnthropicModel,
		"gemini_key":                             st.GeminiKey,
		"gemini_model":                           st.GeminiModel,
		"ai_pricing_json":                        st.AIPricingJSON,
		"instance_url":                           st.InstanceURL,
		"validate_url_pattern":                   st.ValidateURLPattern,
		"ui_animations_enabled":                  boolString(st.UIAnimationsEnabled),
		"automation_ignored_containers":          st.AutomationIgnoredContainers,
		"malware_ignored_mounts":                 st.MalwareIgnoredMounts,
		"auto_upgrade_max_concurrency":           intString(st.AutoUpgradeMaxConcurrency),
		"auto_upgrade_min_retry_minutes":         intString(st.AutoUpgradeMinRetryMinutes),
		"trivy_sweep_mode":                       st.TrivySweepMode,
		"clamav_snapshot_max_bytes":              int64String(st.ClamAVSnapshotMaxBytes),
		"data_retention_days":                    intString(st.DataRetentionDays),
		"retention_logs_days":                    intString(st.RetentionLogsDays),
		"retention_metrics_days":                 intString(st.RetentionMetricsDays),
		"retention_scan_results_days":            intString(st.RetentionScanResultsDays),
		"retention_scan_jobs_days":               intString(st.RetentionScanJobsDays),
		"retention_update_runs_days":             intString(st.RetentionUpdateRunsDays),
		"retention_compose_audit_days":           intString(st.RetentionComposeAuditDays),
		"retention_ai_usage_days":                intString(st.RetentionAIUsageDays),
		"metrics_normalized":                     boolString(st.MetricsNormalized),
		"global_bypass_ai":                       boolString(st.GlobalBypassAI),
		"global_skip_health_check":               boolString(st.GlobalSkipHealthCheck),
		"default_validate_mode":                  st.DefaultValidateMode,
		"default_validate_timeout_sec":           intString(st.DefaultValidateTimeoutSec),
		"default_validate_interval_sec":          intString(st.DefaultValidateIntervalSec),
		"default_ai_validate_logs":               boolString(st.DefaultAIValidateLogs),
		"default_auto_rollback":                  boolString(st.DefaultAutoRollback),
		"default_restart_on_unhealthy":           boolString(st.DefaultRestartOnUnhealthy),
		"unhealthy_auto_remediation_enabled":     boolString(st.UnhealthyAutoRemediationEnabled),
		"unhealthy_restart_cooldown_sec_default": intString(st.UnhealthyRestartCooldownSecDefault),
		"max_restarts_per_window":                intString(st.MaxRestartsPerWindow),
		"ai_testing_passed":                      boolString(st.AITestingPassed),
		"portainer_testing_passed":               boolString(st.PortainerTestingPassed),
	}

	jsonToDbKey := map[string]string{
		"discordWebhookUrl":                  "discord_webhook_url",
		"discordEnabled":                     "discord_enabled",
		"portainerUrl":                       "portainer_url",
		"portainerApiKey":                    "portainer_api_key",
		"portainerEnabled":                   "portainer_enabled",
		"aiEnabled":                          "ai_enabled",
		"aiProvider":                         "ai_provider",
		"aiBlockRiskThreshold":               "ai_block_risk_threshold",
		"openaiKey":                          "openai_key",
		"openaiModel":                        "openai_model",
		"anthropicKey":                       "anthropic_key",
		"anthropicModel":                     "anthropic_model",
		"geminiKey":                          "gemini_key",
		"geminiModel":                        "gemini_model",
		"aiPricingJson":                      "ai_pricing_json",
		"instanceUrl":                        "instance_url",
		"validateUrlPattern":                 "validate_url_pattern",
		"uiAnimationsEnabled":                "ui_animations_enabled",
		"automationIgnoredContainers":        "automation_ignored_containers",
		"malwareIgnoredMounts":               "malware_ignored_mounts",
		"autoUpgradeMaxConcurrency":          "auto_upgrade_max_concurrency",
		"autoUpgradeMinRetryMinutes":         "auto_upgrade_min_retry_minutes",
		"trivySweepMode":                     "trivy_sweep_mode",
		"clamavSnapshotMaxBytes":             "clamav_snapshot_max_bytes",
		"dataRetentionDays":                  "data_retention_days",
		"retentionLogsDays":                  "retention_logs_days",
		"retentionMetricsDays":               "retention_metrics_days",
		"retentionScanResultsDays":           "retention_scan_results_days",
		"retentionScanJobsDays":              "retention_scan_jobs_days",
		"retentionUpdateRunsDays":            "retention_update_runs_days",
		"retentionComposeAuditDays":          "retention_compose_audit_days",
		"retentionAIUsageDays":               "retention_ai_usage_days",
		"metricsNormalized":                  "metrics_normalized",
		"globalBypassAi":                     "global_bypass_ai",
		"globalSkipHealthCheck":              "global_skip_health_check",
		"defaultValidateMode":                "default_validate_mode",
		"defaultValidateTimeoutSec":          "default_validate_timeout_sec",
		"defaultValidateIntervalSec":         "default_validate_interval_sec",
		"defaultAiValidateLogs":              "default_ai_validate_logs",
		"defaultAutoRollback":                "default_auto_rollback",
		"defaultRestartOnUnhealthy":          "default_restart_on_unhealthy",
		"unhealthyAutoRemediationEnabled":    "unhealthy_auto_remediation_enabled",
		"unhealthyRestartCooldownSecDefault": "unhealthy_restart_cooldown_sec_default",
		"maxRestartsPerWindow":               "max_restarts_per_window",
		"aiTestingPassed":                    "ai_testing_passed",
		"portainerTestingPassed":             "portainer_testing_passed",
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

func (s *Store) SetDashboardUpdateCheckSnapshot(ctx context.Context, detectedCount int, checkedAt int64) error {
	if detectedCount < 0 {
		detectedCount = 0
	}
	if checkedAt < 0 {
		checkedAt = 0
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, val := range map[string]string{
		dashboardUpdateDetectedCountKey: intString(detectedCount),
		dashboardUpdateCheckedAtKey:     int64String(checkedAt),
	} {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO app_settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value
`, key, val); err != nil {
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

func intString(value int) string {
	return strconv.Itoa(value)
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

func parseStoredInt(value string, defaultValue, minValue, maxValue int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return defaultValue
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}

func parseStoredInt64(value string, defaultValue, minValue, maxValue int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return defaultValue
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
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

func normalizeValidateModeValue(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "http":
		return "http"
	case "docker":
		return "docker"
	case "both":
		return "both"
	default:
		return "both"
	}
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

func normalizeTrivySweepMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "all-images":
		return "all-images"
	default:
		return "running-only"
	}
}
