package updates

import "testing"

func TestNewestBackupName_PicksLatestTimestamp(t *testing.T) {
	got, err := newestBackupName([]string{
		"abc_backup_1700000000",
		"abc_backup_1700000100",
		"abc_backup_1699999999",
	}, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc_backup_1700000100" {
		t.Fatalf("expected latest timestamp backup, got %q", got)
	}
}

func TestNewestBackupName_FallsBackDeterministically(t *testing.T) {
	got, err := newestBackupName([]string{
		"abc_backup_two",
		"abc_backup_one",
	}, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Lexicographically highest fallback value is selected for deterministic rollback behavior.
	if got != "abc_backup_two" {
		t.Fatalf("expected deterministic fallback selection, got %q", got)
	}
}

func TestLiveContainerRefPrefersContainerName(t *testing.T) {
	got := liveContainerRef(Request{ContainerID: "id-123", ContainerName: "gluetun"})
	if got != "gluetun" {
		t.Fatalf("expected container name ref, got %q", got)
	}
}

func TestLiveContainerRefFallsBackToContainerID(t *testing.T) {
	got := liveContainerRef(Request{ContainerID: "id-123"})
	if got != "id-123" {
		t.Fatalf("expected container id fallback, got %q", got)
	}
}

func TestComposeUpArgsIncludesAllComposeFiles(t *testing.T) {
	req := Request{
		ComposeConfigFiles: []string{"/tmp/base.yml", "/tmp/override.yml"},
		ComposeEnvFiles:    []string{"/tmp/.env", "/tmp/.harborwatch.env"},
	}
	got := composeUpArgs(req, "web")
	want := []string{"compose", "-f", "/tmp/base.yml", "-f", "/tmp/override.yml", "--env-file", "/tmp/.env", "--env-file", "/tmp/.harborwatch.env", "up", "-d", "--no-deps", "--force-recreate", "web"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] got %q want %q (all args=%v)", i, got[i], want[i], got)
		}
	}
}

func TestComposePullArgsIncludesAllComposeFiles(t *testing.T) {
	req := Request{
		ComposeConfigFiles: []string{"/tmp/base.yml", "/tmp/override.yml"},
		ComposeEnvFiles:    []string{"/tmp/.env", "/tmp/.harborwatch.env"},
	}
	got := composePullArgs(req, "web")
	want := []string{"compose", "-f", "/tmp/base.yml", "-f", "/tmp/override.yml", "--env-file", "/tmp/.env", "--env-file", "/tmp/.harborwatch.env", "pull", "web"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] got %q want %q (all args=%v)", i, got[i], want[i], got)
		}
	}
}
