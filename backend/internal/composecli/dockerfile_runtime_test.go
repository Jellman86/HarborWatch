package composecli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDockerfileUsesComposePluginInsteadOfLegacyStandalone(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file path")
	}

	dockerfilePath := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "Dockerfile"))
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	text := string(content)
	if strings.Contains(text, "\n    docker-compose \\\n") || strings.Contains(text, "\n    docker-compose\n") {
		t.Fatalf("Dockerfile still installs legacy docker-compose runtime")
	}
	if !strings.Contains(text, "docker compose version") && !strings.Contains(text, "cli-plugins/docker-compose") {
		t.Fatalf("Dockerfile does not appear to install the Docker Compose plugin")
	}
}
