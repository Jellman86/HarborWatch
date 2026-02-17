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
