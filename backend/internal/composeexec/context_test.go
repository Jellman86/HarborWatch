package composeexec

import "testing"

func TestContextRunnerArgsIncludeOrderedEnvFiles(t *testing.T) {
	ctx := New("/tmp/project", []string{"/tmp/project/docker-compose.yml", "/tmp/project/override.yml"}, []string{"/tmp/project/.env", "/tmp/project/.harborwatch.env"})
	got := ctx.RunnerArgs("pull", "web")
	want := []string{
		"-f", "/tmp/project/docker-compose.yml",
		"-f", "/tmp/project/override.yml",
		"--env-file", "/tmp/project/.env",
		"--env-file", "/tmp/project/.harborwatch.env",
		"pull", "web",
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] got %q want %q (all args=%v)", i, got[i], want[i], got)
		}
	}
}

func TestContextDeduplicatesBlankFiles(t *testing.T) {
	ctx := New("", []string{"", "/tmp/a.yml", "/tmp/a.yml"}, []string{"", "/tmp/.env", "/tmp/.env"})
	got := ctx.RunnerArgs("up", "-d")
	want := []string{"-f", "/tmp/a.yml", "--env-file", "/tmp/.env", "up", "-d"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d] got %q want %q (all args=%v)", i, got[i], want[i], got)
		}
	}
}
