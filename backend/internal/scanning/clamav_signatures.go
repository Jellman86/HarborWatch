package scanning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ClamAVSignatureStatus struct {
	EngineVersion     string   `json:"engineVersion"`
	DatabaseVersion   string   `json:"databaseVersion,omitempty"`
	DatabaseTimestamp string   `json:"databaseTimestamp,omitempty"`
	DatabasePublished int64    `json:"databasePublished,omitempty"`
	DatabaseDir       string   `json:"databaseDir,omitempty"`
	DatabaseFiles     []string `json:"databaseFiles,omitempty"`
	LastLocalUpdate   int64    `json:"lastLocalUpdate,omitempty"`
}

func ReadClamAVSignatureStatus(ctx context.Context) (ClamAVSignatureStatus, error) {
	if _, err := exec.LookPath("clamscan"); err != nil {
		return ClamAVSignatureStatus{}, ErrClamAVUnavailable
	}

	cmd := exec.CommandContext(ctx, "clamscan", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ClamAVSignatureStatus{}, fmt.Errorf("clamscan --version failed: %w (%s)", err, string(output))
	}
	engine, dbVersion, dbTimestamp, dbPublished := parseClamVersionLine(firstLine(output))
	dbDir := clamAVDatabaseDir()
	files, lastUpdate, _ := discoverClamDBFiles(dbDir)

	return ClamAVSignatureStatus{
		EngineVersion:     engine,
		DatabaseVersion:   dbVersion,
		DatabaseTimestamp: dbTimestamp,
		DatabasePublished: dbPublished,
		DatabaseDir:       dbDir,
		DatabaseFiles:     files,
		LastLocalUpdate:   lastUpdate,
	}, nil
}

func UpdateClamAVSignatures(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("freshclam"); err != nil {
		return "", errors.New("freshclam is not installed or not on PATH")
	}

	args := []string{
		"--stdout",
		"--datadir", clamAVDatabaseDir(),
		"--checks=" + strconv.Itoa(envInt("HW_CLAMAV_FRESHCLAM_CHECKS", 2, 1, 50)),
	}
	cmd := exec.CommandContext(ctx, "freshclam", args...)
	output, err := cmd.CombinedOutput()
	summary := summarizeFreshclamOutput(string(output))
	if err != nil {
		return summary, fmt.Errorf("freshclam failed: %w (%s)", err, summary)
	}
	return summary, nil
}

func parseClamVersionLine(line string) (engine, dbVersion, dbTimestamp string, dbPublished int64) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", "", 0
	}
	parts := strings.SplitN(line, "/", 3)
	engine = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		dbVersion = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		dbTimestamp = strings.TrimSpace(parts[2])
		for _, layout := range []string{
			"Mon Jan 2 15:04:05 2006",
			"Mon Jan _2 15:04:05 2006",
			time.RFC3339,
		} {
			if ts, err := time.Parse(layout, dbTimestamp); err == nil {
				dbPublished = ts.Unix()
				break
			}
		}
	}
	return engine, dbVersion, dbTimestamp, dbPublished
}

func clamAVDatabaseDir() string {
	raw := strings.TrimSpace(os.Getenv("HW_CLAMAV_DB_PATH"))
	if raw == "" {
		return "/var/lib/clamav"
	}
	return filepath.Clean(raw)
}

func discoverClamDBFiles(dbDir string) ([]string, int64, error) {
	patterns := []string{"*.cvd", "*.cld", "*.cud", "*.inc"}
	seen := map[string]struct{}{}
	files := make([]string, 0, 8)
	var newest int64
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dbDir, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			base := filepath.Base(match)
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			files = append(files, base)
			info, err := os.Stat(match)
			if err == nil && info.ModTime().Unix() > newest {
				newest = info.ModTime().Unix()
			}
		}
	}
	sort.Strings(files)
	return files, newest, nil
}

func summarizeFreshclamOutput(output string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	lines := strings.Split(output, "\n")
	out := make([]string, 0, 6)
	for i := len(lines) - 1; i >= 0 && len(out) < 6; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) == 0 {
		return "No output from freshclam."
	}
	return strings.Join(out, " | ")
}

func firstLine(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	return text
}
