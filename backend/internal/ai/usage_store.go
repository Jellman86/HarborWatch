package ai

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type UsageSQLiteStore struct {
	db *sql.DB
}

func NewUsageSQLiteStore(db *sql.DB) *UsageSQLiteStore {
	return &UsageSQLiteStore{db: db}
}

func (s *UsageSQLiteStore) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
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
CREATE INDEX IF NOT EXISTS idx_ai_usage_ts ON ai_usage_events(ts);
CREATE INDEX IF NOT EXISTS idx_ai_usage_provider_model ON ai_usage_events(provider, model);
CREATE INDEX IF NOT EXISTS idx_ai_conv_ts ON ai_conversations(ts);
`)
	return err
}

func (s *UsageSQLiteStore) RecordUsage(ctx context.Context, rec UsageRecord) error {
	ts := rec.Timestamp
	if ts <= 0 {
		ts = time.Now().UTC().Unix()
	}
	if rec.TotalTokens <= 0 {
		rec.TotalTokens = rec.InputTokens + rec.OutputTokens
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO ai_usage_events(ts, provider, model, feature, input_tokens, output_tokens, total_tokens)
VALUES(?, ?, ?, ?, ?, ?, ?)
`, ts, rec.Provider, rec.Model, rec.Feature, rec.InputTokens, rec.OutputTokens, rec.TotalTokens)
	return err
}

func (s *UsageSQLiteStore) RecordConversation(ctx context.Context, rec ConversationRecord) error {
	ts := rec.Timestamp
	if ts <= 0 {
		ts = time.Now().UTC().Unix()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO ai_conversations(ts, provider, model, feature, prompt, response)
VALUES(?, ?, ?, ?, ?, ?)
`, ts, rec.Provider, rec.Model, rec.Feature, rec.Prompt, rec.Response)
	return err
}

func (s *UsageSQLiteStore) ListConversations(ctx context.Context, limit, offset int) ([]ConversationRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ts, provider, model, feature, prompt, response
FROM ai_conversations
ORDER BY ts DESC, id DESC
LIMIT ? OFFSET ?
`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConversationRecord
	for rows.Next() {
		var rec ConversationRecord
		if err := rows.Scan(&rec.Timestamp, &rec.Provider, &rec.Model, &rec.Feature, &rec.Prompt, &rec.Response); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *UsageSQLiteStore) SummaryUsage(ctx context.Context, from, to int64) (UsageSummary, error) {
	if to <= 0 {
		to = time.Now().UTC().Unix()
	}
	if from <= 0 || from > to {
		from = to - 30*24*3600
	}
	out := UsageSummary{
		From:           from,
		To:             to,
		Breakdown:      []UsageBreakdown{},
		Daily:          []UsageDaily{},
		DailyBreakdown: []UsageDailyBreakdown{},
	}

	if err := s.db.QueryRowContext(ctx, `
SELECT
	COALESCE(COUNT(*), 0),
	COALESCE(SUM(input_tokens), 0),
	COALESCE(SUM(output_tokens), 0),
	COALESCE(SUM(total_tokens), 0)
FROM ai_usage_events
WHERE ts >= ? AND ts <= ?
`, from, to).Scan(&out.Calls, &out.InputTokens, &out.OutputTokens, &out.TotalTokens); err != nil {
		return out, fmt.Errorf("query ai usage totals: %w", err)
	}

	breakdownRows, err := s.db.QueryContext(ctx, `
SELECT provider, model, feature,
	COALESCE(COUNT(*), 0) AS calls,
	COALESCE(SUM(input_tokens), 0) AS input_tokens,
	COALESCE(SUM(output_tokens), 0) AS output_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM ai_usage_events
WHERE ts >= ? AND ts <= ?
GROUP BY provider, model, feature
ORDER BY total_tokens DESC, calls DESC
`, from, to)
	if err != nil {
		return out, fmt.Errorf("query ai usage breakdown: %w", err)
	}
	defer breakdownRows.Close()
	for breakdownRows.Next() {
		var item UsageBreakdown
		if err := breakdownRows.Scan(
			&item.Provider,
			&item.Model,
			&item.Feature,
			&item.Calls,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
		); err != nil {
			return out, fmt.Errorf("scan ai usage breakdown: %w", err)
		}
		out.Breakdown = append(out.Breakdown, item)
	}
	if err := breakdownRows.Err(); err != nil {
		return out, fmt.Errorf("iterate ai usage breakdown: %w", err)
	}

	dailyRows, err := s.db.QueryContext(ctx, `
SELECT date(ts, 'unixepoch') AS day,
	COALESCE(COUNT(*), 0) AS calls,
	COALESCE(SUM(input_tokens), 0) AS input_tokens,
	COALESCE(SUM(output_tokens), 0) AS output_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM ai_usage_events
WHERE ts >= ? AND ts <= ?
GROUP BY day
ORDER BY day ASC
`, from, to)
	if err != nil {
		return out, fmt.Errorf("query ai usage daily: %w", err)
	}
	defer dailyRows.Close()
	for dailyRows.Next() {
		var item UsageDaily
		if err := dailyRows.Scan(
			&item.Day,
			&item.Calls,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
		); err != nil {
			return out, fmt.Errorf("scan ai usage daily: %w", err)
		}
		out.Daily = append(out.Daily, item)
	}
	if err := dailyRows.Err(); err != nil {
		return out, fmt.Errorf("iterate ai usage daily: %w", err)
	}

	dailyBreakdownRows, err := s.db.QueryContext(ctx, `
SELECT date(ts, 'unixepoch') AS day, provider, model,
	COALESCE(SUM(input_tokens), 0) AS input_tokens,
	COALESCE(SUM(output_tokens), 0) AS output_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens
FROM ai_usage_events
WHERE ts >= ? AND ts <= ?
GROUP BY day, provider, model
ORDER BY day ASC
`, from, to)
	if err != nil {
		return out, fmt.Errorf("query ai usage daily breakdown: %w", err)
	}
	defer dailyBreakdownRows.Close()
	for dailyBreakdownRows.Next() {
		var item UsageDailyBreakdown
		if err := dailyBreakdownRows.Scan(
			&item.Day,
			&item.Provider,
			&item.Model,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
		); err != nil {
			return out, fmt.Errorf("scan ai usage daily breakdown: %w", err)
		}
		out.DailyBreakdown = append(out.DailyBreakdown, item)
	}
	if err := dailyBreakdownRows.Err(); err != nil {
		return out, fmt.Errorf("iterate ai usage daily breakdown: %w", err)
	}

	return out, nil
}

func (s *UsageSQLiteStore) PruneUsage(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM ai_usage_events WHERE ts < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune ai usage events: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ai usage rows affected: %w", err)
	}
	return rows, nil
}
