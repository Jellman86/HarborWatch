// Code generated from api/openapi.yaml; DO NOT EDIT.

export type HealthResponse = { status: string; service: string; version: string };
export type Metric = { containerId: string; timestamp: number; cpuPercent: number; memoryUsage: number; memoryLimit: number; pids: number };
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
