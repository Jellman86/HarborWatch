package scanning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var ErrClamAVUnavailable = errors.New("clamscan is not installed or not on PATH")

// MalwareResult represents the result of a malware scan.
type MalwareResult struct {
	ContainerName string
	Target        string
	Source        string
	Infected      bool
	FoundThreats  []string
	RawOutput     string
	ScannedAt     int64
}

// MalwareScanner defines the interface for filesystem malware scanning.
type MalwareScanner interface {
	Name() string
	ScanPath(ctx context.Context, path string) (MalwareResult, error)
}

type clamAVScanner struct{}

func NewClamAVScanner() MalwareScanner {
	return clamAVScanner{}
}

func (clamAVScanner) Name() string { return "clamav" }

func (clamAVScanner) ScanPath(ctx context.Context, path string) (MalwareResult, error) {
	if _, err := exec.LookPath("clamscan"); err != nil {
		return MalwareResult{}, ErrClamAVUnavailable
	}

	// clamscan exit codes: 0 = no virus, 1 = virus found, 2 = error
	args := []string{
		"--no-summary",
		"-r",
		"--scan-archive=" + clamBoolFlag("HW_CLAMAV_SCAN_ARCHIVES", false),
		"--max-filesize=" + strconv.Itoa(envInt("HW_CLAMAV_MAX_FILE_MB", 32, 1, 4096)) + "M",
		"--max-scansize=" + strconv.Itoa(envInt("HW_CLAMAV_MAX_SCAN_MB", 512, 8, 16384)) + "M",
		"--max-files=" + strconv.Itoa(envInt("HW_CLAMAV_MAX_FILES", 12000, 200, 1000000)),
		"--max-recursion=" + strconv.Itoa(envInt("HW_CLAMAV_MAX_RECURSION", 16, 1, 128)),
		path,
	}
	cmd := exec.CommandContext(ctx, "clamscan", args...)
	output, err := cmd.CombinedOutput()

	res := MalwareResult{
		Target:    path,
		Source:    "clamav",
		RawOutput: string(output),
		ScannedAt: time.Now().UTC().Unix(),
	}

	if err != nil {
		if ctx.Err() != nil {
			return res, fmt.Errorf("clamscan timed out or was cancelled: %w", ctx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				res.Infected = true
				res.FoundThreats = parseClamOutput(string(output))
				return res, nil
			}
		}
		return res, fmt.Errorf("clamscan failed: %w (%s)", err, string(output))
	}

	return res, nil
}

func clamBoolFlag(key string, fallback bool) string {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return "yes"
	case "0", "false", "no", "off":
		return "no"
	default:
		if fallback {
			return "yes"
		}
		return "no"
	}
}

func parseClamOutput(output string) []string {
	details := parseClamThreatDetails(output)
	threats := make([]string, 0, len(details))
	for _, d := range details {
		if d.Signature != "" {
			threats = append(threats, d.Signature)
		}
	}
	return threats
}

type clamThreatDetail struct {
	Path      string
	Signature string
}

func parseClamThreatDetails(output string) []clamThreatDetail {
	lines := strings.Split(output, "\n")
	details := make([]clamThreatDetail, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, " FOUND") {
			continue
		}

		// Example:
		// /path/to/file: Eicar-Signature FOUND
		idx := strings.LastIndex(line, ": ")
		if idx < 0 {
			continue
		}
		filePath := strings.TrimSpace(line[:idx])
		signature := strings.TrimSpace(strings.TrimSuffix(line[idx+2:], " FOUND"))
		details = append(details, clamThreatDetail{
			Path:      filePath,
			Signature: signature,
		})
	}
	return details
}
