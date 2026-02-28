package gitops

import (
	"strings"
	"testing"
)

func TestBuildEnvFileSortsKeys(t *testing.T) {
	content, err := buildEnvFile(map[string]string{
		"Z_VAR": "z",
		"A_VAR": "a",
	})
	if err != nil {
		t.Fatalf("buildEnvFile returned error: %v", err)
	}
	if content != "A_VAR=a\nZ_VAR=z\n" {
		t.Fatalf("unexpected env content: %q", content)
	}
}

func TestBuildEnvFileRejectsInvalidEntries(t *testing.T) {
	if _, err := buildEnvFile(map[string]string{"BAD=KEY": "x"}); err == nil {
		t.Fatalf("expected invalid key error")
	}

	if _, err := buildEnvFile(map[string]string{"OK": "multi\nline"}); err == nil {
		t.Fatalf("expected multiline value error")
	}
}

func TestBuildEnvFileRejectsWindowsLineEndingsInValues(t *testing.T) {
	_, err := buildEnvFile(map[string]string{"A": "value\r\n"})
	if err == nil {
		t.Fatalf("expected multiline value validation error")
	}
	if strings.Contains(err.Error(), "\r") {
		t.Fatalf("expected normalized error text, got %q", err.Error())
	}
}
