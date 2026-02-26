package composesnapshots

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultSnapshotRoot = "/data/compose-snapshots"
)

type SnapshotChangeStatus string

const (
	SnapshotChangeStatusNone      SnapshotChangeStatus = "none"
	SnapshotChangeStatusUnchanged SnapshotChangeStatus = "unchanged"
	SnapshotChangeStatusChanged   SnapshotChangeStatus = "changed"
)

type CreateSnapshotInput struct {
	RootDir      string
	ProjectName  string
	ServiceName  string
	WorkingDir   string
	ConfigFiles  []string
	CreatedAtUTC time.Time
}

type FileFingerprint struct {
	Path         string `json:"path"`
	BaseName     string `json:"baseName,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	ModTimeUnix  int64  `json:"modTimeUnix,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	Exists       bool   `json:"exists"`
	ReadError    string `json:"readError,omitempty"`
	TrackedKind  string `json:"trackedKind,omitempty"` // compose|project_env
	ArchiveEntry string `json:"archiveEntry,omitempty"`
}

type SnapshotManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	SnapshotID    string            `json:"snapshotId"`
	CreatedAt     int64             `json:"createdAt"`
	ProjectName   string            `json:"projectName"`
	ServiceName   string            `json:"serviceName,omitempty"`
	WorkingDir    string            `json:"workingDir,omitempty"`
	ConfigFiles   []string          `json:"configFiles,omitempty"`
	Files         []FileFingerprint `json:"files,omitempty"`
}

type SnapshotRecord struct {
	ID           string           `json:"id"`
	CreatedAt    int64            `json:"createdAt"`
	ArchivePath  string           `json:"archivePath"`
	ManifestPath string           `json:"manifestPath"`
	Manifest     SnapshotManifest `json:"manifest"`
}

type ProjectSnapshotStatus struct {
	Status           SnapshotChangeStatus `json:"status"`
	ChangedFileCount int                  `json:"changedFileCount"`
	SnapshotCount    int                  `json:"snapshotCount"`
	Latest           *SnapshotRecord      `json:"latest,omitempty"`
}

func ResolveRoot(configured string) string {
	if p := strings.TrimSpace(configured); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv("HW_COMPOSE_SNAPSHOT_ROOT")); p != "" {
		return p
	}
	return defaultSnapshotRoot
}

func CreateProjectSnapshot(input CreateSnapshotInput) (SnapshotRecord, error) {
	project := strings.TrimSpace(input.ProjectName)
	if project == "" {
		return SnapshotRecord{}, errors.New("compose snapshot requires project name")
	}
	root := ResolveRoot(input.RootDir)
	if root == "" {
		return SnapshotRecord{}, errors.New("compose snapshot root is empty")
	}
	configFiles := dedupePaths(input.ConfigFiles)
	if len(configFiles) == 0 {
		return SnapshotRecord{}, errors.New("compose snapshot requires config files")
	}
	createdAt := input.CreatedAtUTC.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	projectDir := filepath.Join(root, snapshotProjectDir(project, input.WorkingDir, configFiles))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return SnapshotRecord{}, fmt.Errorf("mkdir snapshot project dir: %w", err)
	}

	snapshotID := buildSnapshotID(createdAt, input.ServiceName)
	archivePath := filepath.Join(projectDir, snapshotID+".zip")
	manifestPath := filepath.Join(projectDir, snapshotID+".manifest.json")

	files, err := collectSnapshotFiles(input.WorkingDir, configFiles)
	if err != nil {
		return SnapshotRecord{}, err
	}
	fingerprints := make([]FileFingerprint, 0, len(files))
	for _, file := range files {
		fp, err := fingerprintFile(file.path, file.kind)
		if err != nil {
			return SnapshotRecord{}, err
		}
		fingerprints = append(fingerprints, fp)
	}

	manifest := SnapshotManifest{
		SchemaVersion: 1,
		SnapshotID:    snapshotID,
		CreatedAt:     createdAt.Unix(),
		ProjectName:   project,
		ServiceName:   strings.TrimSpace(input.ServiceName),
		WorkingDir:    strings.TrimSpace(input.WorkingDir),
		ConfigFiles:   append([]string(nil), configFiles...),
		Files:         fingerprints,
	}
	if err := writeSnapshotArchive(archivePath, manifest); err != nil {
		return SnapshotRecord{}, err
	}
	if err := writeManifestFile(manifestPath, manifest); err != nil {
		_ = os.Remove(archivePath)
		return SnapshotRecord{}, err
	}

	return SnapshotRecord{
		ID:           snapshotID,
		CreatedAt:    manifest.CreatedAt,
		ArchivePath:  archivePath,
		ManifestPath: manifestPath,
		Manifest:     manifest,
	}, nil
}

func DetectProjectChangeStatus(rootDir, projectName, workingDir string, configFiles []string) (ProjectSnapshotStatus, error) {
	root := ResolveRoot(rootDir)
	latest, count, err := loadLatestSnapshot(root, projectName, workingDir, configFiles)
	if err != nil {
		return ProjectSnapshotStatus{}, err
	}
	if latest == nil {
		return ProjectSnapshotStatus{
			Status:        SnapshotChangeStatusNone,
			SnapshotCount: count,
		}, nil
	}

	currentFiles, err := collectSnapshotFiles(workingDir, configFiles)
	if err != nil {
		return ProjectSnapshotStatus{}, err
	}
	current := map[string]FileFingerprint{}
	for _, file := range currentFiles {
		fp, err := fingerprintFile(file.path, file.kind)
		if err != nil {
			return ProjectSnapshotStatus{}, err
		}
		current[fp.Path] = fp
	}
	previous := map[string]FileFingerprint{}
	for _, fp := range latest.Manifest.Files {
		previous[strings.TrimSpace(fp.Path)] = fp
	}

	changed := 0
	seen := map[string]struct{}{}
	for path, prev := range previous {
		seen[path] = struct{}{}
		cur, ok := current[path]
		if !ok {
			changed++
			continue
		}
		if !fingerprintsEquivalent(prev, cur) {
			changed++
		}
	}
	for path, cur := range current {
		if _, ok := seen[path]; ok {
			continue
		}
		if cur.Exists {
			changed++
		}
	}

	status := SnapshotChangeStatusUnchanged
	if changed > 0 {
		status = SnapshotChangeStatusChanged
	}
	return ProjectSnapshotStatus{
		Status:           status,
		ChangedFileCount: changed,
		SnapshotCount:    count,
		Latest:           latest,
	}, nil
}

type trackedFile struct {
	path string
	kind string
}

func collectSnapshotFiles(workingDir string, configFiles []string) ([]trackedFile, error) {
	seen := map[string]struct{}{}
	files := make([]trackedFile, 0, len(configFiles)+1)
	for _, cfg := range dedupePaths(configFiles) {
		if cfg == "" {
			continue
		}
		if _, ok := seen[cfg]; ok {
			continue
		}
		if _, err := os.Stat(cfg); err != nil {
			return nil, fmt.Errorf("snapshot compose file %s: %w", cfg, err)
		}
		seen[cfg] = struct{}{}
		files = append(files, trackedFile{path: cfg, kind: "compose"})
	}
	if wd := strings.TrimSpace(workingDir); wd != "" {
		envPath := filepath.Join(wd, ".env")
		if _, ok := seen[envPath]; !ok {
			if _, err := os.Stat(envPath); err == nil {
				seen[envPath] = struct{}{}
				files = append(files, trackedFile{path: envPath, kind: "project_env"})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func fingerprintFile(path string, kind string) (FileFingerprint, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return FileFingerprint{}, errors.New("empty file path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return FileFingerprint{}, fmt.Errorf("stat %s: %w", path, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileFingerprint{}, fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return FileFingerprint{
		Path:        path,
		BaseName:    filepath.Base(path),
		SizeBytes:   info.Size(),
		ModTimeUnix: info.ModTime().UTC().Unix(),
		SHA256:      hex.EncodeToString(sum[:]),
		Exists:      true,
		TrackedKind: strings.TrimSpace(kind),
	}, nil
}

func writeSnapshotArchive(archivePath string, manifest SnapshotManifest) error {
	tmpPath := archivePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create snapshot archive: %w", err)
	}
	zw := zip.NewWriter(f)

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("marshal snapshot manifest: %w", err)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("create manifest entry: %w", err)
	}
	if _, err := mw.Write(manifestBytes); err != nil {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write manifest entry: %w", err)
	}

	for i, fp := range manifest.Files {
		entryName := fmt.Sprintf("files/%02d-%s", i+1, safeArchiveBaseName(fp.BaseName))
		w, err := zw.Create(entryName)
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("create archive entry for %s: %w", fp.Path, err)
		}
		src, err := os.Open(fp.Path)
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("open snapshot file %s: %w", fp.Path, err)
		}
		if _, err := io.Copy(w, src); err != nil {
			_ = src.Close()
			_ = zw.Close()
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("copy snapshot file %s: %w", fp.Path, err)
		}
		_ = src.Close()
	}

	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close snapshot archive: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close snapshot archive file: %w", err)
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalize snapshot archive: %w", err)
	}
	return nil
}

func writeManifestFile(path string, manifest SnapshotManifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest file: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write manifest file: %w", err)
	}
	return nil
}

func loadLatestSnapshot(rootDir, projectName, workingDir string, configFiles []string) (*SnapshotRecord, int, error) {
	project := strings.TrimSpace(projectName)
	if project == "" {
		return nil, 0, nil
	}
	dir := filepath.Join(rootDir, snapshotProjectDir(project, workingDir, dedupePaths(configFiles)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("read snapshot dir: %w", err)
	}
	manifestNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".manifest.json") {
			manifestNames = append(manifestNames, name)
		}
	}
	sort.Strings(manifestNames)
	if len(manifestNames) == 0 {
		return nil, 0, nil
	}
	count := len(manifestNames)
	latestManifestName := manifestNames[len(manifestNames)-1]
	manifestPath := filepath.Join(dir, latestManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, count, fmt.Errorf("read snapshot manifest: %w", err)
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, count, fmt.Errorf("parse snapshot manifest: %w", err)
	}
	snapshotID := strings.TrimSuffix(latestManifestName, ".manifest.json")
	archivePath := filepath.Join(dir, snapshotID+".zip")
	rec := &SnapshotRecord{
		ID:           snapshotID,
		CreatedAt:    manifest.CreatedAt,
		ArchivePath:  archivePath,
		ManifestPath: manifestPath,
		Manifest:     manifest,
	}
	return rec, count, nil
}

func dedupePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
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

func snapshotProjectDir(projectName, workingDir string, configFiles []string) string {
	key := strings.TrimSpace(projectName) + "|" + strings.TrimSpace(workingDir) + "|" + strings.Join(dedupePaths(configFiles), ",")
	sum := sha256.Sum256([]byte(key))
	return sanitizePathSegment(projectName) + "-" + hex.EncodeToString(sum[:6])
}

func buildSnapshotID(ts time.Time, serviceName string) string {
	service := sanitizePathSegment(strings.TrimSpace(serviceName))
	if service == "" {
		service = "project"
	}
	return ts.UTC().Format("20060102T150405Z") + "-" + service
}

func sanitizePathSegment(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "compose"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "compose"
	}
	return out
}

func safeArchiveBaseName(s string) string {
	s = filepath.Base(strings.TrimSpace(s))
	if s == "" || s == "." || s == string(filepath.Separator) {
		return "file"
	}
	s = strings.ReplaceAll(s, string(filepath.Separator), "_")
	return s
}

func fingerprintsEquivalent(a, b FileFingerprint) bool {
	return a.Exists == b.Exists && a.SHA256 == b.SHA256 && a.SizeBytes == b.SizeBytes
}
