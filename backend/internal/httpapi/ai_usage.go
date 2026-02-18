package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
)

type aiUsageResponse struct {
	Span              string                     `json:"span"`
	From              int64                      `json:"from"`
	To                int64                      `json:"to"`
	Calls             int64                      `json:"calls"`
	InputTokens       int64                      `json:"inputTokens"`
	OutputTokens      int64                      `json:"outputTokens"`
	TotalTokens       int64                      `json:"totalTokens"`
	PricingConfigured bool                       `json:"pricingConfigured"`
	EstimatedCostUSD  float64                    `json:"estimatedCostUsd,omitempty"`
	PricingError      string                     `json:"pricingError,omitempty"`
	Breakdown         []aiUsageBreakdownResponse `json:"breakdown"`
	Daily             []aiUsageDailyResponse     `json:"daily"`
}

type aiUsageBreakdownResponse struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Feature          string  `json:"feature"`
	Calls            int64   `json:"calls"`
	InputTokens      int64   `json:"inputTokens"`
	OutputTokens     int64   `json:"outputTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	EstimatedCostUSD float64 `json:"estimatedCostUsd,omitempty"`
}

type aiUsageDailyResponse struct {
	Day               string  `json:"day"`
	Calls             int64   `json:"calls"`
	InputTokens       int64   `json:"inputTokens"`
	OutputTokens      int64   `json:"outputTokens"`
	TotalTokens       int64   `json:"totalTokens"`
	EstimatedCostUSD  float64 `json:"estimatedCostUsd,omitempty"`
	CumulativeCostUSD float64 `json:"cumulativeCostUsd,omitempty"`
}

type aiPricingEntry struct {
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	InputPer1M        float64 `json:"inputPer1M"`
	OutputPer1M       float64 `json:"outputPer1M"`
	InputPer1MTokens  float64 `json:"inputPer1MTokens"`
	OutputPer1MTokens float64 `json:"outputPer1MTokens"`
}

type aiPricingRate struct {
	InputPer1M  float64
	OutputPer1M float64
}

func parseAIUsageSpan(raw string) (string, time.Duration) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "24h":
		return "24h", 24 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	case "90d":
		return "90d", 90 * 24 * time.Hour
	case "30d":
		fallthrough
	default:
		return "30d", 30 * 24 * time.Hour
	}
}

func parseAIPricing(raw string) (map[string]aiPricingRate, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]aiPricingRate{}, nil
	}
	var entries []aiPricingEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil, err
	}
	rates := map[string]aiPricingRate{}
	for _, entry := range entries {
		provider := strings.ToLower(strings.TrimSpace(entry.Provider))
		model := strings.ToLower(strings.TrimSpace(entry.Model))
		if provider == "" || model == "" {
			continue
		}
		input := entry.InputPer1M
		if input == 0 {
			input = entry.InputPer1MTokens
		}
		output := entry.OutputPer1M
		if output == 0 {
			output = entry.OutputPer1MTokens
		}
		rates[provider+"|"+model] = aiPricingRate{
			InputPer1M:  input,
			OutputPer1M: output,
		}
	}
	return rates, nil
}

func lookupPricingRate(rates map[string]aiPricingRate, provider, model string) (aiPricingRate, bool) {
	if len(rates) == 0 {
		return aiPricingRate{}, false
	}
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.ToLower(strings.TrimSpace(model))
	if rate, ok := rates[p+"|"+m]; ok {
		return rate, true
	}
	if rate, ok := rates[p+"|*"]; ok {
		return rate, true
	}
	if rate, ok := rates["*|*"]; ok {
		return rate, true
	}
	return aiPricingRate{}, false
}

func estimateTokenCostUSD(inputTokens, outputTokens int64, rate aiPricingRate) float64 {
	return (float64(inputTokens)/1_000_000.0)*rate.InputPer1M + (float64(outputTokens)/1_000_000.0)*rate.OutputPer1M
}

func buildAIUsageResponse(summary ai.UsageSummary, span string, pricingJSON string) aiUsageResponse {
	resp := aiUsageResponse{
		Span:         span,
		From:         summary.From,
		To:           summary.To,
		Calls:        summary.Calls,
		InputTokens:  summary.InputTokens,
		OutputTokens: summary.OutputTokens,
		TotalTokens:  summary.TotalTokens,
		Breakdown:    []aiUsageBreakdownResponse{},
		Daily:        []aiUsageDailyResponse{},
	}
	rates, err := parseAIPricing(pricingJSON)
	if err != nil {
		resp.PricingError = err.Error()
		rates = map[string]aiPricingRate{}
	}
	resp.PricingConfigured = len(rates) > 0
	dailyCostByDay := map[string]float64{}

	for _, item := range summary.Breakdown {
		row := aiUsageBreakdownResponse{
			Provider:     item.Provider,
			Model:        item.Model,
			Feature:      item.Feature,
			Calls:        item.Calls,
			InputTokens:  item.InputTokens,
			OutputTokens: item.OutputTokens,
			TotalTokens:  item.TotalTokens,
		}
		if rate, ok := lookupPricingRate(rates, item.Provider, item.Model); ok {
			row.EstimatedCostUSD = estimateTokenCostUSD(item.InputTokens, item.OutputTokens, rate)
			resp.EstimatedCostUSD += row.EstimatedCostUSD
		}
		resp.Breakdown = append(resp.Breakdown, row)
	}

	if resp.PricingConfigured {
		for _, item := range summary.DailyBreakdown {
			rate, ok := lookupPricingRate(rates, item.Provider, item.Model)
			if !ok {
				continue
			}
			dailyCostByDay[item.Day] += estimateTokenCostUSD(item.InputTokens, item.OutputTokens, rate)
		}
	}

	cumulative := 0.0
	for _, item := range summary.Daily {
		row := aiUsageDailyResponse{
			Day:          item.Day,
			Calls:        item.Calls,
			InputTokens:  item.InputTokens,
			OutputTokens: item.OutputTokens,
			TotalTokens:  item.TotalTokens,
		}
		if resp.PricingConfigured {
			row.EstimatedCostUSD = dailyCostByDay[item.Day]
			cumulative += row.EstimatedCostUSD
			row.CumulativeCostUSD = cumulative
		}
		resp.Daily = append(resp.Daily, row)
	}
	return resp
}

type aiUsageSummarizer interface {
	UsageSummary(ctx context.Context, from, to int64) (ai.UsageSummary, error)
}
