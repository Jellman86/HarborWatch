package composecli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolvePrefersDockerComposePlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	toolDir := t.TempDir()
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  exit 0
fi
echo "$0 $@" >> "$TEST_LOG"
`)

	originalPath := os.Getenv("PATH")
	originalLog := os.Getenv("TEST_LOG")
	logPath := filepath.Join(toolDir, "calls.log")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)
	t.Setenv("TEST_LOG", logPath)
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
		_ = os.Setenv("TEST_LOG", originalLog)
	})

	runner, err := Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if runner.Binary != "docker" {
		t.Fatalf("expected docker binary, got %q", runner.Binary)
	}
	if strings.Join(runner.PrefixArgs, " ") != "compose" {
		t.Fatalf("expected compose prefix, got %#v", runner.PrefixArgs)
	}
}

func TestResolveFallsBackToDockerComposeBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	toolDir := t.TempDir()
	writeTestScript(t, filepath.Join(toolDir, "docker"), `#!/bin/sh
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then
  echo "compose plugin missing" >&2
  exit 1
fi
exit 0
`)
	writeTestScript(t, filepath.Join(toolDir, "docker-compose"), `#!/bin/sh
if [ "$1" = "version" ]; then
  exit 0
fi
exit 0
`)

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+originalPath)
	t.Cleanup(func() { _ = os.Setenv("PATH", originalPath) })

	runner, err := Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if runner.Binary != "docker-compose" {
		t.Fatalf("expected docker-compose binary, got %q", runner.Binary)
	}
	if len(runner.PrefixArgs) != 0 {
		t.Fatalf("expected no prefix args, got %#v", runner.PrefixArgs)
	}
}

func TestResolveReturnsHelpfulErrorWhenComposeUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}
	toolDir := t.TempDir()
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", toolDir)
	t.Cleanup(func() { _ = os.Setenv("PATH", originalPath) })

	_, err := Resolve(context.Background())
	if err == nil {
		t.Fatalf("expected Resolve to fail when no compose runtime exists")
	}
	if !strings.Contains(err.Error(), "docker compose") || !strings.Contains(err.Error(), "docker-compose") {
		t.Fatalf("expected error to mention both compose runtimes, got %q", err.Error())
	}
}

func writeTestScript(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write script %s: %v", path, err)
	}
}
