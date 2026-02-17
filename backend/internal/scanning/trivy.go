package scanning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

var ErrTrivyUnavailable = errors.New("trivy is not installed or not on PATH")

type trivyScanner struct{}

func NewTrivyScanner() Scanner {
	return trivyScanner{}
}

func (trivyScanner) Name() string { return "trivy" }

func (trivyScanner) Scan(ctx context.Context, target string) (Result, error) {
	if _, err := exec.LookPath("trivy"); err != nil {
		return Result{}, ErrTrivyUnavailable
	}

	args := []string{
		"image",
		"--quiet",
		"--format", "json",
		"--scanners", "vuln",
		"--timeout", trivyInternalTimeout(),
		target,
	}
	cmd := exec.CommandContext(ctx, "trivy", args...)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("trivy scan timed out or was cancelled: %w", ctx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Result{}, fmt.Errorf("trivy scan failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return Result{}, fmt.Errorf("trivy scan failed: %w", err)
	}

	var parsed struct {
		Results []struct {
			Vulnerabilities []struct {
				Severity string `json:"Severity"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return Result{}, fmt.Errorf("parse trivy JSON: %w", err)
	}

	res := Result{
		Target:  target,
		Source:  "trivy",
		RawJSON: string(output),
		Scanned: time.Now().UTC().Unix(),
	}
	for _, group := range parsed.Results {
		for _, vuln := range group.Vulnerabilities {
			switch vuln.Severity {
			case "CRITICAL":
				res.Critical++
			case "HIGH":
				res.High++
			case "MEDIUM":
				res.Medium++
			case "LOW":
				res.Low++
			default:
				res.Unknown++
			}
		}
	}

	return res, nil
}

func trivyInternalTimeout() string {
	if v := os.Getenv("HW_TRIVY_INTERNAL_TIMEOUT"); v != "" {
		return v
	}
	return "10m"
}
