package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseAnalysisResult(raw string) (AnalysisResult, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return AnalysisResult{}, fmt.Errorf("empty AI response")
	}

	if first := strings.Index(candidate, "{"); first >= 0 {
		if last := strings.LastIndex(candidate, "}"); last > first {
			candidate = candidate[first : last+1]
		}
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(candidate), &result); err != nil {
		return AnalysisResult{}, err
	}
	if result.RiskLevel == "" {
		result.RiskLevel = RiskMedium
	}
	if strings.TrimSpace(result.Summary) == "" {
		result.Summary = "No summary provided by AI provider"
	}
	return result, nil
}

func parseHealthAssessment(raw string) (HealthAssessment, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return HealthAssessment{}, fmt.Errorf("empty AI response")
	}

	if first := strings.Index(candidate, "{"); first >= 0 {
		if last := strings.LastIndex(candidate, "}"); last > first {
			candidate = candidate[first : last+1]
		}
	}

	var result HealthAssessment
	if err := json.Unmarshal([]byte(candidate), &result); err != nil {
		return HealthAssessment{}, err
	}
	if strings.TrimSpace(result.Summary) == "" {
		result.Summary = "No summary provided by AI provider"
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}
	return result, nil
}
