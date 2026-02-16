#!/usr/bin/env bash
set -euo pipefail

cat > backend/internal/gen/types_gen.go <<'GEN_GO'
// Code generated from api/openapi.yaml; DO NOT EDIT.

package gen

type HealthResponse struct { Status string `json:"status"`; Service string `json:"service"`; Version string `json:"version"` }
type AuditJobSummary struct { ID string `json:"id"`; Type string `json:"type"`; Target string `json:"target"`; Status string `json:"status"`; Error string `json:"error,omitempty"`; StartedAt int64 `json:"startedAt"`; CompletedAt int64 `json:"completedAt,omitempty"` }
type ContainerSummary struct { ID string `json:"id"`; Names []string `json:"names"`; Image string `json:"image"`; State string `json:"state"`; Status string `json:"status"`; Labels map[string]string `json:"labels"` }
type ImageSummary struct { ID string `json:"id"`; RepoTags []string `json:"repoTags"`; Size int64 `json:"size"` }
type DockerEvent struct { Type string `json:"type"`; Action string `json:"action"`; ID string `json:"id"`; From string `json:"from"`; Attributes map[string]string `json:"attributes,omitempty"`; Time int64 `json:"time"` }
type ScanRunRequest struct { Target string `json:"target"` }
type MalwareScanRequest struct { Target string `json:"target"` }
type ScanStartResponse struct { JobID string `json:"jobId"`; Status string `json:"status"` }
type ScanJobStatus struct { JobID string `json:"jobId"`; Target string `json:"target"`; Status string `json:"status"`; Source string `json:"source"`; Error string `json:"error,omitempty"`; StartedAt int64 `json:"startedAt"`; CompletedAt int64 `json:"completedAt,omitempty"` }
type ScanSummary struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Critical int `json:"critical"`; High int `json:"high"`; Medium int `json:"medium"`; Low int `json:"low"`; Unknown int `json:"unknown"`; Total int `json:"total"`; RiskScore int `json:"riskScore"` }
type MalwareScanSummary struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Infected bool `json:"infected"`; ThreatsFound []string `json:"threatsFound"` }
type ReleaseExcerpt struct { Tag string `json:"tag"`; Text string `json:"text"`; Weight int `json:"weight"` }
type ReleaseRiskSummary struct { Repo string `json:"repo"`; LatestTag string `json:"latestTag"`; LatestPublishedAt int64 `json:"latestPublishedAt"`; ReleasesAnalyzed int `json:"releasesAnalyzed"`; TotalRisk int `json:"totalRisk"`; BreakingChangeLikely bool `json:"breakingChangeLikely"`; HighlightedExcerpts []ReleaseExcerpt `json:"highlightedExcerpts"`; GeneratedAt int64 `json:"generatedAt"` }
type UpdateStartRequest struct { ContainerID string `json:"containerId"`; TargetImage string `json:"targetImage"`; ValidateURL string `json:"validateUrl"` }
type UpdateStartResponse struct { JobID string `json:"jobId"`; Status string `json:"status"` }
type UpdateStepEvent struct { JobID string `json:"jobId"`; Step string `json:"step"`; Status string `json:"status"`; Message string `json:"message"`; Timestamp int64 `json:"timestamp"` }
type AIAnalysisSummary struct { RiskScore int `json:"riskScore"`; RiskLevel string `json:"riskLevel"`; Summary string `json:"summary"`; BreakingChanges []string `json:"breakingChanges"` }
type UpdateJobStatus struct { JobID string `json:"jobId"`; ContainerID string `json:"containerId"`; TargetImage string `json:"targetImage"`; ValidateURL string `json:"validateUrl"`; Status string `json:"status"`; CreatedAt int64 `json:"createdAt"`; UpdatedAt int64 `json:"updatedAt"`; Error string `json:"error"`; AIAnalysis *AIAnalysisSummary `json:"aiAnalysis,omitempty"`; Steps []UpdateStepEvent `json:"steps"` }
GEN_GO

cat > web/src/lib/api-types.ts <<'GEN_TS'
// Code generated from api/openapi.yaml; DO NOT EDIT.

export type HealthResponse = { status: string; service: string; version: string };
export type AuditJobSummary = { id: string; type: string; target: string; status: string; error?: string; startedAt: number; completedAt?: number };
export type ContainerSummary = { id: string; names: string[]; image: string; state: string; status: string; labels: Record<string, string> };
export type ImageSummary = { id: string; repoTags: string[]; size: number };
export type DockerEvent = { type: string; action: string; id: string; from: string; attributes?: Record<string, string>; time: number };
export type ScanRunRequest = { target: string };
export type MalwareScanRequest = { target: string };
export type ScanStartResponse = { jobId: string; status: string };
export type ScanJobStatus = { jobId: string; target: string; status: string; source: string; error?: string; startedAt: number; completedAt?: number };
export type ScanSummary = { target: string; source: string; scannedAt: number; critical: number; high: number; medium: number; low: number; unknown: number; total: number; riskScore: number };
export type MalwareScanSummary = { target: string; source: string; scannedAt: number; infected: boolean; threatsFound: string[] };
export type ReleaseExcerpt = { tag: string; text: string; weight: number };
export type ReleaseRiskSummary = { repo: string; latestTag: string; latestPublishedAt: number; releasesAnalyzed: number; totalRisk: number; breakingChangeLikely: boolean; highlightedExcerpts: ReleaseExcerpt[]; generatedAt: number };
export type UpdateStartRequest = { containerId: string; targetImage: string; validateUrl: string };
export type UpdateStartResponse = { jobId: string; status: string };
export type UpdateStepEvent = { jobId: string; step: string; status: string; message: string; timestamp: number };
export type UpdateJobStatus = { jobId: string; containerId: string; targetImage: string; validateUrl: string; status: string; createdAt: number; updatedAt: number; error: string; aiAnalysis?: { riskScore: number; riskLevel: string; summary: string; breakingChanges: string[] }; steps: UpdateStepEvent[] };
GEN_TS

echo "Generated Go + TypeScript API types from api/openapi.yaml"
