package gitops

type AuthMethod string

const (
	AuthMethodNone      AuthMethod = "none"
	AuthMethodHTTPToken AuthMethod = "http_token"
	AuthMethodSSHKey    AuthMethod = "ssh_key"
)

type GitSource struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	URL              string     `json:"url"`
	Branch           string     `json:"branch"`
	TargetDir        string     `json:"targetDir"`
	AuthMethod       AuthMethod `json:"authMethod"`
	AuthSecret       string     `json:"authSecret,omitempty"` // Omitted from API responses usually, handle carefully
	SyncIntervalMins int        `json:"syncIntervalMins"`
	LastCommitHash   string     `json:"lastCommitHash"`
	LastSyncError    string     `json:"lastSyncError"`
	LastSyncAt       int64      `json:"lastSyncAt"`
	CreatedAt        int64      `json:"createdAt"`
}

type GitDeployment struct {
	ID               string `json:"id"`
	GitSourceID      string `json:"gitSourceId"`
	ComposePath      string `json:"composePath"`
	EnvVarsJSON      string `json:"envVarsJson"` // Legacy JSON string map of overrides
	EnvFilePath      string `json:"envFilePath"`
	EnvInlineContent string `json:"envInlineContent"` // Raw .env content for HarborWatch-managed overrides
	EnvInlineEnabled bool   `json:"envInlineEnabled"`
	AutoCreated      bool   `json:"autoCreated"`
	Enabled          bool   `json:"enabled"`
	LastDeployedHash string `json:"lastDeployedHash"`
	LastDeployedAt   int64  `json:"lastDeployedAt"`
	LastError        string `json:"lastError"`
}
