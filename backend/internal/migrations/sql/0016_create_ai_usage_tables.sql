CREATE TABLE IF NOT EXISTS ai_usage_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts INTEGER NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	feature TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ai_conversations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts INTEGER NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	feature TEXT NOT NULL,
	prompt TEXT NOT NULL,
	response TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_fleet_advice (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ts INTEGER NOT NULL,
	inventory TEXT NOT NULL,
	advice TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_ts ON ai_usage_events(ts);
CREATE INDEX IF NOT EXISTS idx_ai_usage_provider_model ON ai_usage_events(provider, model);
CREATE INDEX IF NOT EXISTS idx_ai_conv_ts ON ai_conversations(ts);
