package scanning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	cmd := exec.CommandContext(ctx, "trivy", "image", "--quiet", "--format", "json", target)
	output, err := cmd.Output()
	if err != nil {
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
