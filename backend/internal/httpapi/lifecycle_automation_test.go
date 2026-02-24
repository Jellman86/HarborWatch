package httpapi

import (
	"context"
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

type stubSettingsService struct {
	st settings.Settings
}

func (s stubSettingsService) Get(ctx context.Context) (settings.Settings, error)   { return s.st, nil }
func (s stubSettingsService) Save(ctx context.Context, in settings.Settings) error { return nil }

func TestEffectiveContainerRules_UsesGlobalLifecycleDefaultsWhenRuleMissing(t *testing.T) {
	summary := gen.ContainerSummary{ID: "abc", Names: []string{"/gluetun"}}
	existing := rules.ContainerRules{
		ContainerID: "abc",
		Exists:      false,
	}
	stSvc := stubSettingsService{st: settings.Settings{
		DefaultValidateMode:                "docker",
		DefaultValidateTimeoutSec:          120,
		DefaultValidateIntervalSec:         5,
		DefaultAIValidateLogs:              true,
		DefaultAutoRollback:                false,
		DefaultRestartOnUnhealthy:          true,
		UnhealthyRestartCooldownSecDefault: 900,
	}}

	got := effectiveContainerRules(context.Background(), summary, existing, stSvc, nil)
	if got.ValidateMode != "docker" {
		t.Fatalf("expected validate mode docker, got %q", got.ValidateMode)
	}
	if got.ValidateTimeoutSec != 120 {
		t.Fatalf("expected timeout 120, got %d", got.ValidateTimeoutSec)
	}
	if got.ValidateIntervalSec != 5 {
		t.Fatalf("expected interval 5, got %d", got.ValidateIntervalSec)
	}
	if !got.AIValidateLogs {
		t.Fatalf("expected AIValidateLogs true")
	}
	if got.AutoRollback {
		t.Fatalf("expected AutoRollback false from defaults")
	}
	if !got.RestartOnUnhealthy {
		t.Fatalf("expected RestartOnUnhealthy true from defaults")
	}
	if got.UnhealthyRestartCooldownSec != 900 {
		t.Fatalf("expected cooldown 900, got %d", got.UnhealthyRestartCooldownSec)
	}
}

func TestBuildUpdateRequestForContainer_UsesGlobalValidationDefaultsWhenNoRulesService(t *testing.T) {
	res, err := buildUpdateRequestForContainer(
		context.Background(),
		"abc123",
		"ghcr.io/example/app:latest",
		"http://localhost:8080/health",
		"",
		0,
		0,
		false,
		false,
		nil,
		nil,
		nil,
		stubSettingsService{st: settings.Settings{
			DefaultValidateMode:        "docker",
			DefaultValidateTimeoutSec:  95,
			DefaultValidateIntervalSec: 4,
		}},
		nil,
		nil,
		nil,
		updateRequestBuildOptions{},
	)
	if err != nil {
		t.Fatalf("build update request: %v", err)
	}
	if res.Request.ValidateMode != "docker" {
		t.Fatalf("expected validate mode docker, got %q", res.Request.ValidateMode)
	}
	if res.Request.ValidateTimeoutSec != 95 {
		t.Fatalf("expected timeout 95, got %d", res.Request.ValidateTimeoutSec)
	}
	if res.Request.ValidateIntervalSec != 4 {
		t.Fatalf("expected interval 4, got %d", res.Request.ValidateIntervalSec)
	}
}

func TestBuildUpdateRequestForContainer_PropagatesDependentRestartRuleFields(t *testing.T) {
	res, err := buildUpdateRequestForContainer(
		context.Background(),
		"abc123",
		"ghcr.io/example/app:latest",
		"http://localhost:8080/health",
		"",
		0,
		0,
		false,
		false,
		nil,
		nil,
		staticRulesService{rule: rules.ContainerRules{
			Exists:                        true,
			UpdatePolicy:                  "manual",
			ValidateMode:                  "both",
			ValidateTimeoutSec:            45,
			ValidateIntervalSec:           2,
			RestartDependentsAfterUpgrade: true,
			DependentRestartDelaySec:      30,
		}},
		nil,
		nil,
		nil,
		nil,
		updateRequestBuildOptions{},
	)
	if err != nil {
		t.Fatalf("build update request: %v", err)
	}
	if !res.Request.RestartDependentsAfterUpgrade {
		t.Fatalf("expected RestartDependentsAfterUpgrade=true")
	}
	if res.Request.DependentRestartDelaySec != 30 {
		t.Fatalf("expected DependentRestartDelaySec=30, got %d", res.Request.DependentRestartDelaySec)
	}
}
