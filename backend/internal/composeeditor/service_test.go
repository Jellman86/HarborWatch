package composeeditor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDraft_InvalidYAML(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewService()
	result := svc.ValidateDraft(context.Background(), ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  workdir,
		ConfigFiles: []string{composeFile},
	}, []DraftFile{
		{Path: composeFile, Content: "services:\n  app:\n    image: [bad"},
	}, nil)

	if result.OK {
		t.Fatalf("expected validation failure")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics")
	}
}

func TestSaveDraft_FailsOnHashConflict(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewService()
	_, err := svc.SaveDraft(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  workdir,
		ConfigFiles: []string{composeFile},
	}, []DraftFile{
		{
			Path:           composeFile,
			Content:        "services:\n  app:\n    image: nginx:1.26\n",
			ExpectedSHA256: "deadbeef",
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrHashConflict) {
		t.Fatalf("expected ErrHashConflict, got %v", err)
	}
}

func TestSaveDraft_RejectsUnknownPath(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewService()
	_, err := svc.SaveDraft(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  workdir,
		ConfigFiles: []string{composeFile},
	}, []DraftFile{
		{
			Path:    filepath.Join(workdir, "other.yml"),
			Content: "services:\n  app:\n    image: nginx:1.27\n",
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrPathNotAllowed) {
		t.Fatalf("expected ErrPathNotAllowed, got %v", err)
	}
}

func TestSaveDraft_SucceedsComposeAndEnv(t *testing.T) {
	workdir := t.TempDir()
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(workdir, ".env")
	composeInitial := "services:\n  app:\n    image: nginx:1.25\n"
	if err := os.WriteFile(composeFile, []byte(composeInitial), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	svc := NewService()
	files, env, err := svc.LoadProjectFiles(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  workdir,
		ConfigFiles: []string{composeFile},
	})
	if err != nil {
		t.Fatalf("load files: %v", err)
	}
	if len(files) != 1 || env == nil {
		t.Fatalf("expected compose + env states")
	}

	_, err = svc.SaveDraft(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  workdir,
		ConfigFiles: []string{composeFile},
	}, []DraftFile{
		{
			Path:           composeFile,
			Content:        "services:\n  app:\n    image: nginx:1.26\n",
			ExpectedSHA256: files[0].SHA256,
		},
	}, &EnvDraft{
		Path:           envFile,
		Content:        "APP_ENV=staging\n",
		Exists:         true,
		ExpectedSHA256: env.SHA256,
	})
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}

	gotCompose, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if string(gotCompose) != "services:\n  app:\n    image: nginx:1.26\n" {
		t.Fatalf("unexpected compose content: %s", string(gotCompose))
	}
	gotEnv, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if string(gotEnv) != "APP_ENV=staging\n" {
		t.Fatalf("unexpected env content: %s", string(gotEnv))
	}
}

func TestSaveDraft_FailsOnNonWritableEnvFile(t *testing.T) {
	skipIfRoot(t)

	workdir := t.TempDir()
	lockedDir := filepath.Join(workdir, "locked")
	if err := os.MkdirAll(lockedDir, 0o755); err != nil {
		t.Fatalf("mkdir locked dir: %v", err)
	}
	composeFile := filepath.Join(workdir, "docker-compose.yml")
	envFile := filepath.Join(lockedDir, ".env")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.WriteFile(envFile, []byte("APP_ENV=prod\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("chmod dir readonly: %v", err)
	}
	defer func() { _ = os.Chmod(lockedDir, 0o755) }()

	svc := NewService()
	_, env, err := svc.LoadProjectFiles(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  lockedDir,
		ConfigFiles: []string{composeFile},
	})
	if err != nil {
		t.Fatalf("load files: %v", err)
	}
	if env == nil {
		t.Fatalf("expected env state")
	}

	_, err = svc.SaveDraft(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  lockedDir,
		ConfigFiles: []string{composeFile},
	}, nil, &EnvDraft{
		Path:           envFile,
		Content:        "APP_ENV=staging\n",
		Exists:         true,
		ExpectedSHA256: env.SHA256,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrFileNotWritable) {
		t.Fatalf("expected ErrFileNotWritable, got %v", err)
	}
}

func TestSaveDraft_FailsOnNonWritableComposeFile(t *testing.T) {
	skipIfRoot(t)

	workdir := t.TempDir()
	lockedDir := filepath.Join(workdir, "locked")
	if err := os.MkdirAll(lockedDir, 0o755); err != nil {
		t.Fatalf("mkdir locked dir: %v", err)
	}
	composeFile := filepath.Join(lockedDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("chmod dir readonly: %v", err)
	}
	defer func() { _ = os.Chmod(lockedDir, 0o755) }()

	svc := NewService()
	files, _, err := svc.LoadProjectFiles(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  lockedDir,
		ConfigFiles: []string{composeFile},
	})
	if err != nil {
		t.Fatalf("load files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected compose file state")
	}

	_, err = svc.SaveDraft(ProjectDescriptor{
		ProjectName: "demo",
		WorkingDir:  lockedDir,
		ConfigFiles: []string{composeFile},
	}, []DraftFile{
		{
			Path:           composeFile,
			Content:        "services:\n  app:\n    image: nginx:1.26\n",
			ExpectedSHA256: files[0].SHA256,
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrFileNotWritable) {
		t.Fatalf("expected ErrFileNotWritable, got %v", err)
	}
}

func TestPrettifyComposeDrafts(t *testing.T) {
	svc := NewService()
	formatted, err := svc.PrettifyComposeDrafts([]DraftFile{
		{
			Path:    "/tmp/docker-compose.yml",
			Content: "services: {app: {image: nginx:1.25, restart: unless-stopped}}",
		},
	})
	if err != nil {
		t.Fatalf("prettify: %v", err)
	}
	if len(formatted) != 1 {
		t.Fatalf("expected 1 formatted file")
	}
	if formatted[0].Content == "" || formatted[0].Content == "services: {app: {image: nginx:1.25, restart: unless-stopped}}" {
		t.Fatalf("expected formatted yaml output, got %q", formatted[0].Content)
	}
}

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission test is not reliable as root")
	}
}
