package scanning

import (
	"strings"
	"testing"
)

func TestParseClamVersionLine(t *testing.T) {
	engine, dbVersion, dbTimestamp, dbPublished := parseClamVersionLine("ClamAV 1.4.2/27495/Mon Feb 17 10:30:00 2026")
	if engine != "ClamAV 1.4.2" {
		t.Fatalf("unexpected engine: %q", engine)
	}
	if dbVersion != "27495" {
		t.Fatalf("unexpected db version: %q", dbVersion)
	}
	if dbTimestamp == "" {
		t.Fatalf("expected db timestamp")
	}
	if dbPublished <= 0 {
		t.Fatalf("expected parsed db published timestamp, got %d", dbPublished)
	}
}

func TestSummarizeFreshclamOutput(t *testing.T) {
	raw := "ClamAV update process started\nmain.cvd is up to date\nbytecode.cld updated\nDatabase updated"
	summary := summarizeFreshclamOutput(raw)
	if !strings.Contains(summary, "Database updated") {
		t.Fatalf("expected summary to include latest line, got %q", summary)
	}
}
