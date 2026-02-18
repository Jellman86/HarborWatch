package httpapi

import (
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/ai"
)

func TestBuildAIUsageResponse_ComputesDailyAndCumulativeSpend(t *testing.T) {
	summary := ai.UsageSummary{
		From:         100,
		To:           200,
		Calls:        3,
		InputTokens:  2500,
		OutputTokens: 1000,
		TotalTokens:  3500,
		Breakdown: []ai.UsageBreakdown{
			{
				Provider:     "openai",
				Model:        "gpt-5.2",
				Feature:      "compose_audit",
				Calls:        3,
				InputTokens:  2500,
				OutputTokens: 1000,
				TotalTokens:  3500,
			},
		},
		Daily: []ai.UsageDaily{
			{Day: "2026-02-17", Calls: 1, InputTokens: 1000, OutputTokens: 200, TotalTokens: 1200},
			{Day: "2026-02-18", Calls: 2, InputTokens: 1500, OutputTokens: 800, TotalTokens: 2300},
		},
		DailyBreakdown: []ai.UsageDailyBreakdown{
			{Day: "2026-02-17", Provider: "openai", Model: "gpt-5.2", InputTokens: 1000, OutputTokens: 200, TotalTokens: 1200},
			{Day: "2026-02-18", Provider: "openai", Model: "gpt-5.2", InputTokens: 1500, OutputTokens: 800, TotalTokens: 2300},
		},
	}
	pricing := `[{"provider":"openai","model":"gpt-5.2","inputPer1M":2,"outputPer1M":4}]`

	out := buildAIUsageResponse(summary, "7d", pricing)
	if !out.PricingConfigured {
		t.Fatalf("expected pricing configured")
	}
	if len(out.Daily) != 2 {
		t.Fatalf("expected 2 daily rows, got %d", len(out.Daily))
	}
	if out.Daily[0].EstimatedCostUSD <= 0 {
		t.Fatalf("expected positive day-one estimated cost, got %f", out.Daily[0].EstimatedCostUSD)
	}
	if out.Daily[1].CumulativeCostUSD <= out.Daily[0].CumulativeCostUSD {
		t.Fatalf("expected cumulative spend to increase, got day1=%f day2=%f", out.Daily[0].CumulativeCostUSD, out.Daily[1].CumulativeCostUSD)
	}
	if out.EstimatedCostUSD <= 0 {
		t.Fatalf("expected positive total estimated cost, got %f", out.EstimatedCostUSD)
	}
}
