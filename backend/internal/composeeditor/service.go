package composeeditor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/dotenv"
	"gopkg.in/yaml.v3"
)

var (
	ErrPathNotAllowed = errors.New("path not allowed for compose editor")
	ErrHashConflict   = errors.New("file hash conflict")
)

type SaveError struct {
	Path string
	Err  error
}

func (e *SaveError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func (e *SaveError) Unwrap() error { return e.Err }

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) LoadProjectFiles(project ProjectDescriptor) ([]FileState, *FileState, error) {
	files := make([]FileState, 0, len(project.ConfigFiles))
	for _, cfg := range project.ConfigFiles {
		st, err := readFileState(strings.TrimSpace(cfg))
		if err != nil {
			return nil, nil, err
		}
		files = append(files, st)
	}

	envPath := ""
	if wd := strings.TrimSpace(project.WorkingDir); wd != "" {
		envPath = filepath.Join(wd, ".env")
	}
	if strings.TrimSpace(envPath) == "" {
		return files, nil, nil
	}

	envState, err := readOptionalFileState(envPath)
	if err != nil {
		return nil, nil, err
	}
	return files, &envState, nil
}

func (s *Service) ValidateDraft(ctx context.Context, project ProjectDescriptor, composeDrafts []DraftFile, envDraft *EnvDraft) ValidationResult {
	diagnostics := []ValidationDiagnostic{}

	composeByPath := map[string]string{}
	for _, draft := range composeDrafts {
		p := strings.TrimSpace(draft.Path)
		if p == "" {
			continue
		}
		composeByPath[p] = draft.Content
	}
	orderedPaths := normalizePaths(project.ConfigFiles)
	if len(orderedPaths) == 0 {
		return ValidationResult{
			OK: false,
			Diagnostics: []ValidationDiagnostic{{
				Severity: "error",
				Source:   "compose-spec",
				Message:  "compose project has no config files",
			}},
		}
	}

	for _, p := range orderedPaths {
		content, ok := composeByPath[p]
		if !ok {
			raw, err := os.ReadFile(p)
			if err != nil {
				diagnostics = append(diagnostics, ValidationDiagnostic{
					Severity: "error",
					Source:   "yaml",
					Path:     p,
					Message:  fmt.Sprintf("read compose file: %v", err),
				})
				continue
			}
			content = string(raw)
		}
		var node any
		if err := yaml.Unmarshal([]byte(content), &node); err != nil {
			diagnostics = append(diagnostics, ValidationDiagnostic{
				Severity: "error",
				Source:   "yaml",
				Path:     p,
				Message:  err.Error(),
			})
		}
	}

	if envDraft != nil && envDraft.Exists {
		if _, err := dotenv.UnmarshalWithLookup(envDraft.Content, nil); err != nil {
			diagnostics = append(diagnostics, ValidationDiagnostic{
				Severity: "error",
				Source:   "env",
				Path:     strings.TrimSpace(envDraft.Path),
				Message:  err.Error(),
			})
		}
	}

	if hasErrorDiagnostics(diagnostics) {
		return ValidationResult{OK: false, Diagnostics: diagnostics}
	}

	tmpDir, err := os.MkdirTemp("", "harborwatch-compose-validate-*")
	if err != nil {
		return ValidationResult{
			OK: false,
			Diagnostics: []ValidationDiagnostic{{
				Severity: "error",
				Source:   "compose-spec",
				Message:  fmt.Sprintf("create temp dir: %v", err),
			}},
		}
	}
	defer os.RemoveAll(tmpDir)

	tempComposeFiles := make([]string, 0, len(orderedPaths))
	for idx, srcPath := range orderedPaths {
		content, ok := composeByPath[srcPath]
		if !ok {
			raw, err := os.ReadFile(srcPath)
			if err != nil {
				diagnostics = append(diagnostics, ValidationDiagnostic{
					Severity: "error",
					Source:   "compose-spec",
					Path:     srcPath,
					Message:  fmt.Sprintf("read compose file: %v", err),
				})
				continue
			}
			content = string(raw)
		}
		targetPath := filepath.Join(tmpDir, fmt.Sprintf("compose-%02d.yml", idx+1))
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			diagnostics = append(diagnostics, ValidationDiagnostic{
				Severity: "error",
				Source:   "compose-spec",
				Path:     srcPath,
				Message:  fmt.Sprintf("write temp compose file: %v", err),
			})
			continue
		}
		tempComposeFiles = append(tempComposeFiles, targetPath)
	}

	if hasErrorDiagnostics(diagnostics) {
		return ValidationResult{OK: false, Diagnostics: diagnostics}
	}

	optsFns := []composecli.ProjectOptionsFn{
		composecli.WithWorkingDirectory(tmpDir),
		composecli.WithName("harborwatch_local_compose_validation"),
		composecli.WithOsEnv,
	}

	if envDraft != nil && envDraft.Exists {
		tempEnv := filepath.Join(tmpDir, ".env")
		if err := os.WriteFile(tempEnv, []byte(envDraft.Content), 0o600); err != nil {
			diagnostics = append(diagnostics, ValidationDiagnostic{
				Severity: "error",
				Source:   "env",
				Path:     tempEnv,
				Message:  fmt.Sprintf("write temp env file: %v", err),
			})
			return ValidationResult{OK: false, Diagnostics: diagnostics}
		}
		optsFns = append(optsFns, composecli.WithEnvFiles(tempEnv), composecli.WithDotEnv)
	}

	options, err := composecli.NewProjectOptions(tempComposeFiles, optsFns...)
	if err != nil {
		diagnostics = append(diagnostics, ValidationDiagnostic{
			Severity: "error",
			Source:   "compose-spec",
			Message:  err.Error(),
		})
		return ValidationResult{OK: false, Diagnostics: diagnostics}
	}
	if _, err := options.LoadProject(ctx); err != nil {
		diagnostics = append(diagnostics, ValidationDiagnostic{
			Severity: "error",
			Source:   "compose-spec",
			Message:  err.Error(),
		})
		return ValidationResult{OK: false, Diagnostics: diagnostics}
	}

	return ValidationResult{
		OK:          true,
		Diagnostics: diagnostics,
	}
}

func (s *Service) PrettifyComposeDrafts(composeDrafts []DraftFile) ([]DraftFile, error) {
	out := make([]DraftFile, 0, len(composeDrafts))
	for _, draft := range composeDrafts {
		var node any
		if err := yaml.Unmarshal([]byte(draft.Content), &node); err != nil {
			return nil, fmt.Errorf("parse %s: %w", strings.TrimSpace(draft.Path), err)
		}
		formatted, err := yaml.Marshal(node)
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", strings.TrimSpace(draft.Path), err)
		}
		out = append(out, DraftFile{
			Path:           draft.Path,
			Content:        string(formatted),
			ExpectedSHA256: draft.ExpectedSHA256,
		})
	}
	return out, nil
}

func (s *Service) SaveDraft(project ProjectDescriptor, composeDrafts []DraftFile, envDraft *EnvDraft) (SaveResult, error) {
	allowedCompose := map[string]struct{}{}
	for _, p := range normalizePaths(project.ConfigFiles) {
		allowedCompose[p] = struct{}{}
	}

	wd := strings.TrimSpace(project.WorkingDir)
	allowedEnvPath := ""
	if wd != "" {
		allowedEnvPath = filepath.Join(wd, ".env")
	}

	for _, draft := range composeDrafts {
		path := strings.TrimSpace(draft.Path)
		if _, ok := allowedCompose[path]; !ok {
			return SaveResult{}, &SaveError{Path: path, Err: ErrPathNotAllowed}
		}
		if err := verifyExpectedHash(path, strings.TrimSpace(draft.ExpectedSHA256)); err != nil {
			return SaveResult{}, &SaveError{Path: path, Err: err}
		}
		if err := writeFileAtomic(path, []byte(draft.Content)); err != nil {
			return SaveResult{}, &SaveError{Path: path, Err: err}
		}
	}

	if envDraft != nil {
		envPath := strings.TrimSpace(envDraft.Path)
		if envPath == "" {
			envPath = allowedEnvPath
		}
		if allowedEnvPath == "" || envPath != allowedEnvPath {
			return SaveResult{}, &SaveError{Path: envPath, Err: ErrPathNotAllowed}
		}
		if envDraft.Exists {
			if err := verifyExpectedHash(envPath, strings.TrimSpace(envDraft.ExpectedSHA256)); err != nil {
				return SaveResult{}, &SaveError{Path: envPath, Err: err}
			}
			if err := writeFileAtomic(envPath, []byte(envDraft.Content)); err != nil {
				return SaveResult{}, &SaveError{Path: envPath, Err: err}
			}
		}
	}

	composeFiles, envFile, err := s.LoadProjectFiles(project)
	if err != nil {
		return SaveResult{}, err
	}
	return SaveResult{
		ComposeFiles: composeFiles,
		EnvFile:      envFile,
	}, nil
}

func hasErrorDiagnostics(diags []ValidationDiagnostic) bool {
	for _, d := range diags {
		if strings.EqualFold(strings.TrimSpace(d.Severity), "error") {
			return true
		}
	}
	return false
}

func normalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func readFileState(path string) (FileState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileState{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return FileState{}, err
	}
	return FileState{
		Path:      path,
		Content:   string(raw),
		SHA256:    hashString(raw),
		SizeBytes: info.Size(),
		Exists:    true,
		Writable:  fileWritable(path),
	}, nil
}

func readOptionalFileState(path string) (FileState, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileState{
				Path:     path,
				Exists:   false,
				Writable: fileWritable(path),
			}, nil
		}
		return FileState{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileState{}, err
	}
	return FileState{
		Path:      path,
		Content:   string(raw),
		SHA256:    hashString(raw),
		SizeBytes: info.Size(),
		Exists:    true,
		Writable:  fileWritable(path),
	}, nil
}

func hashString(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func verifyExpectedHash(path string, expected string) error {
	if expected == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: file missing", ErrHashConflict)
		}
		return err
	}
	if hashString(raw) != expected {
		return fmt.Errorf("%w: expected=%s actual=%s", ErrHashConflict, expected, hashString(raw))
	}
	return nil
}

func fileWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		_ = f.Close()
		return true
	}
	if os.IsNotExist(err) {
		dir := filepath.Dir(path)
		tf, terr := os.CreateTemp(dir, ".harborwatch-write-test-*")
		if terr != nil {
			return false
		}
		name := tf.Name()
		_ = tf.Close()
		_ = os.Remove(name)
		return true
	}
	return false
}

func writeFileAtomic(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".harborwatch-compose-edit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
