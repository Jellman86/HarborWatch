#!/usr/bin/env bash
set -euo pipefail

cat > backend/internal/gen/types_gen.go <<'GEN_GO'
// Code generated from api/openapi.yaml; DO NOT EDIT.

package gen

type HealthResponse struct { Status string `json:"status"`; Service string `json:"service"`; Version string `json:"version"` }
type Metric struct { ContainerID string `json:"containerId"`; Timestamp int64 `json:"timestamp"`; CPUPercent float64 `json:"cpuPercent"`; MemoryUsage int64 `json:"memoryUsage"`; MemoryLimit int64 `json:"memoryLimit"`; Pids int `json:"pids"` }
type AuditJobSummary struct { ID string `json:"id"`; Type string `json:"type"`; Target string `json:"target"`; ContainerID string `json:"containerId,omitempty"`; Status string `json:"status"`; Error string `json:"error,omitempty"`; StartedAt int64 `json:"startedAt"`; CompletedAt int64 `json:"completedAt,omitempty"` }
type ContainerSummary struct { ID string `json:"id"`; Names []string `json:"names"`; Image string `json:"image"`; State string `json:"state"`; Status string `json:"status"`; Labels map[string]string `json:"labels"`; UpdateAvailable bool `json:"updateAvailable"` }
type ContainerDetail struct { Summary ContainerSummary `json:"summary"`; DiskUsage *ContainerDiskUsage `json:"diskUsage,omitempty"`; VulnerabilitySummary *ScanSummary `json:"vulnerabilitySummary,omitempty"`; MalwareSummary []MalwareScanSummary `json:"malwareSummary,omitempty"`; RecentMetrics []Metric `json:"recentMetrics,omitempty"`; ActionHistory []AuditJobSummary `json:"actionHistory,omitempty"`; Rules *ContainerRules `json:"rules,omitempty"` }
type ContainerDiskUsage struct { WritableBytes int64 `json:"writableBytes,omitempty"`; RootFsBytes int64 `json:"rootFsBytes,omitempty"`; MountCount int `json:"mountCount,omitempty"`; HostTotalBytes int64 `json:"hostTotalBytes,omitempty"`; HostAvailableBytes int64 `json:"hostAvailableBytes,omitempty"`; HostUsedBytes int64 `json:"hostUsedBytes,omitempty"` }
type ContainerRules struct { ContainerID string `json:"containerId"`; UpdatePolicy string `json:"updatePolicy"`; ValidateURL string `json:"validateUrl"`; ValidateMode string `json:"validateMode"`; ValidateTimeoutSec int `json:"validateTimeoutSec"`; ValidateIntervalSec int `json:"validateIntervalSec"`; AIValidateLogs bool `json:"aiValidateLogs"`; AutoRollback bool `json:"autoRollback"`; InheritAutomation bool `json:"inheritAutomation"`; UpgradesAutomation bool `json:"upgradesAutomation"`; MaintenanceAutomation bool `json:"maintenanceAutomation"`; SecurityAutomation bool `json:"securityAutomation"` }
type ImageSummary struct { ID string `json:"id"`; RepoTags []string `json:"repoTags"`; Size int64 `json:"size"` }
type DockerEvent struct { Type string `json:"type"`; Action string `json:"action"`; ID string `json:"id"`; From string `json:"from"`; Attributes map[string]string `json:"attributes,omitempty"`; Time int64 `json:"time"` }
type ScanRunRequest struct { Target string `json:"target"` }
type MalwareScanRequest struct { Target string `json:"target"` }
type ScanStartResponse struct { JobID string `json:"jobId"`; Status string `json:"status"` }
type ScanJobStatus struct { JobID string `json:"jobId"`; Target string `json:"target"`; Status string `json:"status"`; Source string `json:"source"`; Error string `json:"error,omitempty"`; StartedAt int64 `json:"startedAt"`; CompletedAt int64 `json:"completedAt,omitempty"` }
type ScanSummary struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Critical int `json:"critical"`; High int `json:"high"`; Medium int `json:"medium"`; Low int `json:"low"`; Unknown int `json:"unknown"`; Total int `json:"total"`; RiskScore int `json:"riskScore"` }
type TrivyVulnerability struct { ID string `json:"id"`; PkgName string `json:"pkgName"`; InstalledVersion string `json:"installedVersion,omitempty"`; FixedVersion string `json:"fixedVersion,omitempty"`; Severity string `json:"severity"`; Title string `json:"title,omitempty"`; Description string `json:"description,omitempty"`; PrimaryURL string `json:"primaryUrl,omitempty"`; CVSSScore float64 `json:"cvssScore,omitempty"`; CVSSSource string `json:"cvssSource,omitempty"`; PublishedDate string `json:"publishedDate,omitempty"`; LastModifiedDate string `json:"lastModifiedDate,omitempty"`; References []string `json:"references,omitempty"` }
type TrivyResultGroup struct { Type string `json:"type,omitempty"`; Target string `json:"target,omitempty"`; Class string `json:"class,omitempty"`; Vulnerabilities []TrivyVulnerability `json:"vulnerabilities,omitempty"` }
type TrivyScanDetails struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Summary ScanSummary `json:"summary"`; Results []TrivyResultGroup `json:"results,omitempty"`; RawJSON string `json:"rawJson,omitempty"`; ParseError string `json:"parseError,omitempty"` }
type MalwareThreatDetail struct { Path string `json:"path,omitempty"`; Signature string `json:"signature,omitempty"` }
type MalwareScanDetail struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Infected bool `json:"infected"`; ThreatsFound []string `json:"threatsFound"`; ThreatDetails []MalwareThreatDetail `json:"threatDetails,omitempty"`; RawOutput string `json:"rawOutput,omitempty"`; ParseError string `json:"parseError,omitempty"` }
type ClamAVSignatureStatus struct { EngineVersion string `json:"engineVersion"`; DatabaseVersion string `json:"databaseVersion,omitempty"`; DatabaseTimestamp string `json:"databaseTimestamp,omitempty"`; DatabasePublished int64 `json:"databasePublished,omitempty"`; DatabaseDir string `json:"databaseDir,omitempty"`; DatabaseFiles []string `json:"databaseFiles,omitempty"`; LastLocalUpdate int64 `json:"lastLocalUpdate,omitempty"` }
type MalwareScanSummary struct { Target string `json:"target"`; Source string `json:"source"`; ScannedAt int64 `json:"scannedAt"`; Infected bool `json:"infected"`; ThreatsFound []string `json:"threatsFound"` }
type ReleaseExcerpt struct { Tag string `json:"tag"`; Text string `json:"text"`; Weight int `json:"weight"` }
type ReleaseRiskSummary struct { Repo string `json:"repo"`; LatestTag string `json:"latestTag"`; LatestPublishedAt int64 `json:"latestPublishedAt"`; ReleasesAnalyzed int `json:"releasesAnalyzed"`; TotalRisk int `json:"totalRisk"`; BreakingChangeLikely bool `json:"breakingChangeLikely"`; HighlightedExcerpts []ReleaseExcerpt `json:"highlightedExcerpts"`; GeneratedAt int64 `json:"generatedAt"` }
type UpdateStartRequest struct { ContainerID string `json:"containerId"`; TargetImage string `json:"targetImage"`; ValidateURL string `json:"validateUrl"` }
type UpdateStartResponse struct { JobID string `json:"jobId"`; Status string `json:"status"` }
type UpdateStepEvent struct { JobID string `json:"jobId"`; Step string `json:"step"`; Status string `json:"status"`; Message string `json:"message"`; Timestamp int64 `json:"timestamp"` }
type AIAnalysisSummary struct { RiskScore int `json:"riskScore"`; RiskLevel string `json:"riskLevel"`; Summary string `json:"summary"`; BreakingChanges []string `json:"breakingChanges"` }
type UpdateJobStatus struct { JobID string `json:"jobId"`; ContainerID string `json:"containerId"`; TargetImage string `json:"targetImage"`; ValidateURL string `json:"validateUrl"`; Status string `json:"status"`; CreatedAt int64 `json:"createdAt"`; UpdatedAt int64 `json:"updatedAt"`; Error string `json:"error"`; AIAnalysis *AIAnalysisSummary `json:"aiAnalysis,omitempty"`; Steps []UpdateStepEvent `json:"steps"` }
type Settings struct { DiscordWebhookURL string `json:"discordWebhookUrl"`; DiscordEnabled bool `json:"discordEnabled"`; PortainerURL string `json:"portainerUrl"`; PortainerApiKey string `json:"portainerApiKey"`; PortainerEnabled bool `json:"portainerEnabled"`; AIEnabled bool `json:"aiEnabled"`; AIProvider string `json:"aiProvider"`; AIBlockRiskThreshold int `json:"aiBlockRiskThreshold"`; OpenAIKey string `json:"openaiKey"`; OpenAIModel string `json:"openaiModel"`; AnthropicKey string `json:"anthropicKey"`; AnthropicModel string `json:"anthropicModel"`; GeminiKey string `json:"geminiKey"`; GeminiModel string `json:"geminiModel"`; AIPricingJSON string `json:"aiPricingJson"`; InstanceURL string `json:"instanceUrl"`; ValidateURLPattern string `json:"validateUrlPattern"`; UIAnimationsEnabled bool `json:"uiAnimationsEnabled"`; AutomationIgnoredContainers string `json:"automationIgnoredContainers"`; MalwareIgnoredMounts string `json:"malwareIgnoredMounts"`; AutoUpgradeMaxConcurrency int `json:"autoUpgradeMaxConcurrency"`; AutoUpgradeMinRetryMinutes int `json:"autoUpgradeMinRetryMinutes"`; ClamAVSnapshotMaxBytes int64 `json:"clamavSnapshotMaxBytes"`; RetentionLogsDays int `json:"retentionLogsDays"`; RetentionMetricsDays int `json:"retentionMetricsDays"`; RetentionScanResultsDays int `json:"retentionScanResultsDays"`; RetentionScanJobsDays int `json:"retentionScanJobsDays"`; RetentionUpdateRunsDays int `json:"retentionUpdateRunsDays"`; RetentionComposeAuditDays int `json:"retentionComposeAuditDays"`; RetentionAIUsageDays int `json:"retentionAIUsageDays"`; EnvironmentOverrides map[string]bool `json:"environmentOverrides"` }
GEN_GO

cat > web/src/lib/api-types.ts <<'GEN_TS'
// Code generated from api/openapi.yaml; DO NOT EDIT.

export type HealthResponse = { status: string; service: string; version: string };
export type Metric = { containerId: string; timestamp: number; cpuPercent: number; memoryUsage: number; memoryLimit: number; pids: number };
export type AuditJobSummary = { id: string; type: string; target: string; containerId?: string; status: string; error?: string; startedAt: number; completedAt?: number };
export type ContainerSummary = { id: string; names: string[]; image: string; state: string; status: string; labels: Record<string, string>; updateAvailable: boolean };
export type ContainerDetail = { summary: ContainerSummary; diskUsage?: ContainerDiskUsage; vulnerabilitySummary?: ScanSummary; malwareSummary?: MalwareScanSummary[]; recentMetrics?: Metric[]; actionHistory?: AuditJobSummary[]; rules?: ContainerRules };
export type ContainerDiskUsage = { writableBytes?: number; rootFsBytes?: number; mountCount?: number; hostTotalBytes?: number; hostAvailableBytes?: number; hostUsedBytes?: number };
export type ContainerRules = { containerId: string; updatePolicy: 'auto' | 'manual' | 'locked'; validateUrl?: string; validateMode?: 'http' | 'docker' | 'both'; validateTimeoutSec?: number; validateIntervalSec?: number; aiValidateLogs?: boolean; autoRollback: boolean; inheritAutomation?: boolean; upgradesAutomation?: boolean; maintenanceAutomation?: boolean; securityAutomation?: boolean };
export type ImageSummary = { id: string; repoTags: string[]; size: number };
export type DockerEvent = { type: string; action: string; id: string; from: string; attributes?: Record<string, string>; time: number };
export type ScanRunRequest = { target: string };
export type MalwareScanRequest = { target: string };
export type ScanStartResponse = { jobId: string; status: string };
export type ScanJobStatus = { jobId: string; target: string; status: string; source: string; error?: string; startedAt: number; completedAt?: number };
export type ScanSummary = { target: string; source: string; scannedAt: number; critical: number; high: number; medium: number; low: number; unknown: number; total: number; riskScore: number };
export type TrivyVulnerability = { id: string; pkgName: string; installedVersion?: string; fixedVersion?: string; severity: string; title?: string; description?: string; primaryUrl?: string; cvssScore?: number; cvssSource?: string; publishedDate?: string; lastModifiedDate?: string; references?: string[] };
export type TrivyResultGroup = { type?: string; target?: string; class?: string; vulnerabilities?: TrivyVulnerability[] };
export type TrivyScanDetails = { target: string; source: string; scannedAt: number; summary: ScanSummary; results?: TrivyResultGroup[]; rawJson?: string; parseError?: string };
export type MalwareThreatDetail = { path?: string; signature?: string };
export type MalwareScanDetail = { target: string; source: string; scannedAt: number; infected: boolean; threatsFound: string[]; threatDetails?: MalwareThreatDetail[]; rawOutput?: string; parseError?: string };
export type ClamAVSignatureStatus = { engineVersion: string; databaseVersion?: string; databaseTimestamp?: string; databasePublished?: number; databaseDir?: string; databaseFiles?: string[]; lastLocalUpdate?: number };
export type MalwareScanSummary = { target: string; source: string; scannedAt: number; infected: boolean; threatsFound: string[] };
export type ReleaseExcerpt = { tag: string; text: string; weight: number };
export type ReleaseRiskSummary = { repo: string; latestTag: string; latestPublishedAt: number; releasesAnalyzed: number; totalRisk: number; breakingChangeLikely: boolean; highlightedExcerpts: ReleaseExcerpt[]; generatedAt: number };
export type UpdateStartRequest = { containerId: string; targetImage: string; validateUrl: string };
export type UpdateStartResponse = { jobId: string; status: string };
export type UpdateStepEvent = { jobId: string; step: string; status: string; message: string; timestamp: number };
export type UpdateJobStatus = { jobId: string; containerId: string; targetImage: string; validateUrl: string; status: string; createdAt: number; updatedAt: number; error: string; aiAnalysis?: { riskScore: number; riskLevel: string; summary: string; breakingChanges: string[] }; steps: UpdateStepEvent[] };
export type Settings = { discordWebhookUrl?: string; discordEnabled?: boolean; portainerUrl?: string; portainerApiKey?: string; portainerEnabled?: boolean; aiEnabled?: boolean; aiProvider?: string; aiBlockRiskThreshold?: number; openaiKey?: string; openaiModel?: string; anthropicKey?: string; anthropicModel?: string; geminiKey?: string; geminiModel?: string; aiPricingJson?: string; instanceUrl?: string; validateUrlPattern?: string; uiAnimationsEnabled?: boolean; automationIgnoredContainers?: string; malwareIgnoredMounts?: string; autoUpgradeMaxConcurrency?: number; autoUpgradeMinRetryMinutes?: number; clamavSnapshotMaxBytes?: number; retentionLogsDays?: number; retentionMetricsDays?: number; retentionScanResultsDays?: number; retentionScanJobsDays?: number; retentionUpdateRunsDays?: number; retentionComposeAuditDays?: number; retentionAIUsageDays?: number; environmentOverrides?: Record<string, boolean> };
GEN_TS

echo "Generated Go + TypeScript API types from api/openapi.yaml"
