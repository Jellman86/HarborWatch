package composeeditor

type ProjectDescriptor struct {
	ProjectName string
	WorkingDir  string
	ConfigFiles []string
}

type FileState struct {
	Path       string `json:"path"`
	Content    string `json:"content,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	Exists     bool   `json:"exists"`
	Writable   bool   `json:"writable"`
	ReadError  string `json:"readError,omitempty"`
	WriteError string `json:"writeError,omitempty"`
}

type DraftFile struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	ExpectedSHA256 string `json:"expectedSha256,omitempty"`
}

type EnvDraft struct {
	Path           string `json:"path,omitempty"`
	Content        string `json:"content"`
	Exists         bool   `json:"exists"`
	ExpectedSHA256 string `json:"expectedSha256,omitempty"`
}

type ValidationDiagnostic struct {
	Severity string `json:"severity"`
	Source   string `json:"source"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type ValidationResult struct {
	OK          bool                   `json:"ok"`
	Diagnostics []ValidationDiagnostic `json:"diagnostics"`
}

type SaveResult struct {
	ComposeFiles []FileState `json:"composeFiles"`
	EnvFile      *FileState  `json:"envFile,omitempty"`
}
