package gitops

import "testing"

func TestNormalizeSourceRejectsTraversalTargetDir(t *testing.T) {
	src := GitSource{
		Name:       "demo",
		URL:        "https://example.com/repo.git",
		Branch:     "main",
		TargetDir:  "../outside",
		AuthMethod: AuthMethodNone,
	}

	if err := NormalizeSource(&src); err == nil {
		t.Fatalf("expected traversal targetDir validation error")
	}
}

func TestNormalizeSourceAuthDefaultsAndRequirements(t *testing.T) {
	src := GitSource{
		Name:      "demo",
		URL:       "https://example.com/repo.git",
		Branch:    "main",
		TargetDir: "stacks/demo",
	}

	if err := NormalizeSource(&src); err != nil {
		t.Fatalf("NormalizeSource returned error: %v", err)
	}
	if src.AuthMethod != AuthMethodNone {
		t.Fatalf("expected auth method default to none, got %q", src.AuthMethod)
	}
	if src.AuthSecret != "" {
		t.Fatalf("expected auth secret cleared for none auth method")
	}

	src.AuthMethod = AuthMethodHTTPToken
	src.AuthSecret = ""
	if err := NormalizeSource(&src); err == nil {
		t.Fatalf("expected missing auth secret validation error")
	}
}

func TestNormalizeDeploymentRejectsAbsoluteComposePath(t *testing.T) {
	dep := GitDeployment{
		GitSourceID: "src-1",
		ComposePath: "/etc/passwd",
	}

	if err := NormalizeDeployment(&dep); err == nil {
		t.Fatalf("expected absolute composePath validation error")
	}
}

func TestNormalizeDeploymentAllowsAbsoluteOrRelativeEnvFilePath(t *testing.T) {
	dep := GitDeployment{
		GitSourceID: "src-1",
		ComposePath: "docker-compose.yml",
		EnvFilePath: "/mnt/Storage-SSD/dockercompose/app/.env",
	}
	if err := NormalizeDeployment(&dep); err != nil {
		t.Fatalf("expected absolute env file path to be allowed, got: %v", err)
	}

	dep.EnvFilePath = "env/.env.prod"
	if err := NormalizeDeployment(&dep); err != nil {
		t.Fatalf("expected relative env file path to be allowed, got: %v", err)
	}
}

func TestResolvePathUnder(t *testing.T) {
	got, err := ResolvePathUnder("/opt/harborwatch/gitops", "stack/docker-compose.yml")
	if err != nil {
		t.Fatalf("ResolvePathUnder returned error: %v", err)
	}
	if got != "/opt/harborwatch/gitops/stack/docker-compose.yml" {
		t.Fatalf("unexpected resolved path: %s", got)
	}

	if _, err := ResolvePathUnder("/opt/harborwatch/gitops", "../../etc/passwd"); err == nil {
		t.Fatalf("expected traversal validation error")
	}
}
