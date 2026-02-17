// Code generated from api/openapi.yaml; DO NOT EDIT.

package gen

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}
type Metric struct {
	ContainerID string  `json:"containerId"`
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsage int64   `json:"memoryUsage"`
	MemoryLimit int64   `json:"memoryLimit"`
	Pids        int     `json:"pids"`
}
type AuditJobSummary struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Target      string `json:"target"`
	ContainerID string `json:"containerId,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt,omitempty"`
}
type ContainerSummary struct {
	ID              string            `json:"id"`
	Names           []string          `json:"names"`
	Image           string            `json:"image"`
	State           string            `json:"state"`
	Status          string            `json:"status"`
	Labels          map[string]string `json:"labels"`
	UpdateAvailable bool              `json:"updateAvailable"`
}
type ContainerDetail struct {
	Summary              ContainerSummary     `json:"summary"`
	DiskUsage            *ContainerDiskUsage  `json:"diskUsage,omitempty"`
	VulnerabilitySummary *ScanSummary         `json:"vulnerabilitySummary,omitempty"`
	MalwareSummary       []MalwareScanSummary `json:"malwareSummary,omitempty"`
	RecentMetrics        []Metric             `json:"recentMetrics,omitempty"`
	ActionHistory        []AuditJobSummary    `json:"actionHistory,omitempty"`
	Rules                *ContainerRules      `json:"rules,omitempty"`
}
type ContainerDiskUsage struct {
	WritableBytes int64 `json:"writableBytes,omitempty"`
	RootFsBytes   int64 `json:"rootFsBytes,omitempty"`
	MountCount    int   `json:"mountCount,omitempty"`
}
type ContainerRules struct {
	ContainerID         string `json:"containerId"`
	UpdatePolicy        string `json:"updatePolicy"`
	ValidateURL         string `json:"validateUrl"`
	ValidateMode        string `json:"validateMode"`
	ValidateTimeoutSec  int    `json:"validateTimeoutSec"`
	ValidateIntervalSec int    `json:"validateIntervalSec"`
	AIValidateLogs      bool   `json:"aiValidateLogs"`
	AutoRollback        bool   `json:"autoRollback"`
}
type ImageSummary struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repoTags"`
	Size     int64    `json:"size"`
}
type DockerEvent struct {
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ID         string            `json:"id"`
	From       string            `json:"from"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Time       int64             `json:"time"`
}
type ScanRunRequest struct {
	Target string `json:"target"`
}
type MalwareScanRequest struct {
	Target string `json:"target"`
}
type ScanStartResponse struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
}
type ScanJobStatus struct {
	JobID       string `json:"jobId"`
	Target      string `json:"target"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Error       string `json:"error,omitempty"`
	StartedAt   int64  `json:"startedAt"`
	CompletedAt int64  `json:"completedAt,omitempty"`
}
type ScanSummary struct {
	Target    string `json:"target"`
	Source    string `json:"source"`
	ScannedAt int64  `json:"scannedAt"`
	Critical  int    `json:"critical"`
	High      int    `json:"high"`
	Medium    int    `json:"medium"`
	Low       int    `json:"low"`
	Unknown   int    `json:"unknown"`
	Total     int    `json:"total"`
	RiskScore int    `json:"riskScore"`
}
type TrivyVulnerability struct {
	ID               string   `json:"id"`
	PkgName          string   `json:"pkgName"`
	InstalledVersion string   `json:"installedVersion,omitempty"`
	FixedVersion     string   `json:"fixedVersion,omitempty"`
	Severity         string   `json:"severity"`
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	PrimaryURL       string   `json:"primaryUrl,omitempty"`
	CVSSScore        float64  `json:"cvssScore,omitempty"`
	CVSSSource       string   `json:"cvssSource,omitempty"`
	PublishedDate    string   `json:"publishedDate,omitempty"`
	LastModifiedDate string   `json:"lastModifiedDate,omitempty"`
	References       []string `json:"references,omitempty"`
}
type TrivyResultGroup struct {
	Type            string               `json:"type,omitempty"`
	Target          string               `json:"target,omitempty"`
	Class           string               `json:"class,omitempty"`
	Vulnerabilities []TrivyVulnerability `json:"vulnerabilities,omitempty"`
}
type TrivyScanDetails struct {
	Target     string             `json:"target"`
	Source     string             `json:"source"`
	ScannedAt  int64              `json:"scannedAt"`
	Summary    ScanSummary        `json:"summary"`
	Results    []TrivyResultGroup `json:"results,omitempty"`
	RawJSON    string             `json:"rawJson,omitempty"`
	ParseError string             `json:"parseError,omitempty"`
}
type MalwareScanSummary struct {
	Target       string   `json:"target"`
	Source       string   `json:"source"`
	ScannedAt    int64    `json:"scannedAt"`
	Infected     bool     `json:"infected"`
	ThreatsFound []string `json:"threatsFound"`
}
type ReleaseExcerpt struct {
	Tag    string `json:"tag"`
	Text   string `json:"text"`
	Weight int    `json:"weight"`
}
type ReleaseRiskSummary struct {
	Repo                 string           `json:"repo"`
	LatestTag            string           `json:"latestTag"`
	LatestPublishedAt    int64            `json:"latestPublishedAt"`
	ReleasesAnalyzed     int              `json:"releasesAnalyzed"`
	TotalRisk            int              `json:"totalRisk"`
	BreakingChangeLikely bool             `json:"breakingChangeLikely"`
	HighlightedExcerpts  []ReleaseExcerpt `json:"highlightedExcerpts"`
	GeneratedAt          int64            `json:"generatedAt"`
}
type UpdateStartRequest struct {
	ContainerID string `json:"containerId"`
	TargetImage string `json:"targetImage"`
	ValidateURL string `json:"validateUrl"`
}
type UpdateStartResponse struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
}
type UpdateStepEvent struct {
	JobID     string `json:"jobId"`
	Step      string `json:"step"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}
type AIAnalysisSummary struct {
	RiskScore       int      `json:"riskScore"`
	RiskLevel       string   `json:"riskLevel"`
	Summary         string   `json:"summary"`
	BreakingChanges []string `json:"breakingChanges"`
}
type UpdateJobStatus struct {
	JobID       string             `json:"jobId"`
	ContainerID string             `json:"containerId"`
	TargetImage string             `json:"targetImage"`
	ValidateURL string             `json:"validateUrl"`
	Status      string             `json:"status"`
	CreatedAt   int64              `json:"createdAt"`
	UpdatedAt   int64              `json:"updatedAt"`
	Error       string             `json:"error"`
	AIAnalysis  *AIAnalysisSummary `json:"aiAnalysis,omitempty"`
	Steps       []UpdateStepEvent  `json:"steps"`
}
type Settings struct {
	DiscordWebhookURL    string          `json:"discordWebhookUrl"`
	GotifyURL            string          `json:"gotifyUrl"`
	GotifyToken          string          `json:"gotifyToken"`
	PortainerURL         string          `json:"portainerUrl"`
	PortainerApiKey      string          `json:"portainerApiKey"`
	AIProvider           string          `json:"aiProvider"`
	OpenAIKey            string          `json:"openaiKey"`
	OpenAIModel          string          `json:"openaiModel"`
	AnthropicKey         string          `json:"anthropicKey"`
	AnthropicModel       string          `json:"anthropicModel"`
	GeminiKey            string          `json:"geminiKey"`
	GeminiModel          string          `json:"geminiModel"`
	InstanceURL          string          `json:"instanceUrl"`
	ValidateURLPattern   string          `json:"validateUrlPattern"`
	EnvironmentOverrides map[string]bool `json:"environmentOverrides"`
}
