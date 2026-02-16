package scanning

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var ErrClamAVUnavailable = errors.New("clamscan is not installed or not on PATH")

// MalwareResult represents the result of a malware scan.
type MalwareResult struct {
	Target       string
	Source       string
	Infected     bool
	FoundThreats []string
	RawOutput    string
	ScannedAt    int64
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
	cmd := exec.CommandContext(ctx, "clamscan", "--no-summary", "-r", path)
	output, err := cmd.CombinedOutput()
	
	res := MalwareResult{
		Target:    path,
		Source:    "clamav",
		RawOutput: string(output),
		ScannedAt: time.Now().UTC().Unix(),
	}

	if err != nil {
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

func parseClamOutput(output string) []string {
	var threats []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, " FOUND") {
			// Example: /path/to/file: Eicar-Signature FOUND
			parts := strings.Split(line, ": ")
			if len(parts) >= 2 {
				threat := strings.TrimSuffix(parts[1], " FOUND")
				threats = append(threats, strings.TrimSpace(threat))
			}
		}
	}
	return threats
}
