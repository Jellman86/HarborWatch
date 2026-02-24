package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type ContainerRules struct {
	Exists                      bool   `json:"-"`
	ContainerID                 string `json:"containerId"`
	ContainerName               string `json:"containerName"`
	UpdatePolicy                string `json:"updatePolicy"` // auto, manual, locked
	ValidateURL                 string `json:"validateUrl"`
	ValidateMode                string `json:"validateMode"` // http, docker, both
	ValidateTimeoutSec          int    `json:"validateTimeoutSec"`
	ValidateIntervalSec         int    `json:"validateIntervalSec"`
	BypassAI                    bool   `json:"bypassAi"`
	SkipHealthCheck             bool   `json:"skipHealthCheck"`
	AIValidateLogs              bool   `json:"aiValidateLogs"`
	AutoRollback                bool   `json:"autoRollback"`
	InheritAutomation           bool   `json:"inheritAutomation"`
	UpgradesAutomation          bool   `json:"upgradesAutomation"`
	MaintenanceAutomation       bool   `json:"maintenanceAutomation"`
	SecurityAutomation          bool   `json:"securityAutomation"`
	RestartOnUnhealthy          bool   `json:"restartOnUnhealthy"`
	UnhealthyRestartCooldownSec int    `json:"unhealthyRestartCooldownSec"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master WHERE type='table' AND name='container_rules' LIMIT 1`).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("container_rules table missing; run schema migrations before rules store init")
	}
	if err != nil {
		return fmt.Errorf("verify container_rules table: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id, name string) (ContainerRules, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT container_id, container_name, update_policy, validate_url, validate_mode, validate_timeout_sec, validate_interval_sec, bypass_ai, skip_health_check, ai_validate_logs, auto_rollback, inherit_automation, upgrades_automation, maintenance_automation, security_automation, restart_on_unhealthy, unhealthy_restart_cooldown_sec
FROM container_rules WHERE container_id = ? OR (container_name = ? AND container_name != '')
ORDER BY rowid DESC
LIMIT 1
`, id, name)

	var r ContainerRules
	var rollback, aiValidateLogs, bypassAI, skipHealthCheck int
	var inheritAutomation, upgradesAutomation, maintenanceAutomation, securityAutomation, restartOnUnhealthy int
	if err := row.Scan(&r.ContainerID, &r.ContainerName, &r.UpdatePolicy, &r.ValidateURL, &r.ValidateMode, &r.ValidateTimeoutSec, &r.ValidateIntervalSec, &bypassAI, &skipHealthCheck, &aiValidateLogs, &rollback, &inheritAutomation, &upgradesAutomation, &maintenanceAutomation, &securityAutomation, &restartOnUnhealthy, &r.UnhealthyRestartCooldownSec); err != nil {
		if err == sql.ErrNoRows {
			return ContainerRules{
				ContainerID:           id,
				ContainerName:         name,
				UpdatePolicy:          "manual",
				ValidateMode:          "both",
				ValidateTimeoutSec:    45,
				ValidateIntervalSec:   2,
				BypassAI:              false,
				SkipHealthCheck:       false,
				AIValidateLogs:        false,
				AutoRollback:          true,
				InheritAutomation:     true,
				UpgradesAutomation:    true,
				MaintenanceAutomation: true,
				SecurityAutomation:    true,
			}, nil
		}
		return r, err
	}
	r.BypassAI = bypassAI == 1
	r.SkipHealthCheck = skipHealthCheck == 1
	r.AIValidateLogs = aiValidateLogs == 1
	r.AutoRollback = rollback == 1
	r.InheritAutomation = inheritAutomation == 1
	r.UpgradesAutomation = upgradesAutomation == 1
	r.MaintenanceAutomation = maintenanceAutomation == 1
	r.SecurityAutomation = securityAutomation == 1
	r.RestartOnUnhealthy = restartOnUnhealthy == 1
	r.Exists = true
	if strings.TrimSpace(r.ValidateMode) == "" {
		r.ValidateMode = "both"
	}
	if r.ValidateTimeoutSec <= 0 {
		r.ValidateTimeoutSec = 45
	}
	if r.ValidateIntervalSec <= 0 {
		r.ValidateIntervalSec = 2
	}
	normalizeAutomationDefaults(&r)
	return r, nil
}

func (s *Store) Save(ctx context.Context, r ContainerRules) error {
	rollback := 0
	if r.AutoRollback {
		rollback = 1
	}
	aiValidateLogs := 0
	if r.AIValidateLogs {
		aiValidateLogs = 1
	}
	bypassAI := 0
	if r.BypassAI {
		bypassAI = 1
	}
	skipHealthCheck := 0
	if r.SkipHealthCheck {
		skipHealthCheck = 1
	}
	if strings.TrimSpace(r.ValidateMode) == "" {
		r.ValidateMode = "both"
	}
	if r.ValidateTimeoutSec <= 0 {
		r.ValidateTimeoutSec = 45
	}
	if r.ValidateIntervalSec <= 0 {
		r.ValidateIntervalSec = 2
	}
	normalizeAutomationDefaults(&r)
	inheritAutomation := 0
	if r.InheritAutomation {
		inheritAutomation = 1
	}
	upgradesAutomation := 0
	if r.UpgradesAutomation {
		upgradesAutomation = 1
	}
	maintenanceAutomation := 0
	if r.MaintenanceAutomation {
		maintenanceAutomation = 1
	}
	securityAutomation := 0
	if r.SecurityAutomation {
		securityAutomation = 1
	}
	restartOnUnhealthy := 0
	if r.RestartOnUnhealthy {
		restartOnUnhealthy = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO container_rules (container_id, container_name, update_policy, validate_url, validate_mode, validate_timeout_sec, validate_interval_sec, bypass_ai, skip_health_check, ai_validate_logs, auto_rollback, inherit_automation, upgrades_automation, maintenance_automation, security_automation, restart_on_unhealthy, unhealthy_restart_cooldown_sec)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(container_id) DO UPDATE SET
    container_name = excluded.container_name,
    update_policy = excluded.update_policy,
    validate_url = excluded.validate_url,
    validate_mode = excluded.validate_mode,
    validate_timeout_sec = excluded.validate_timeout_sec,
    validate_interval_sec = excluded.validate_interval_sec,
    bypass_ai = excluded.bypass_ai,
    skip_health_check = excluded.skip_health_check,
    ai_validate_logs = excluded.ai_validate_logs,
    auto_rollback = excluded.auto_rollback,
    inherit_automation = excluded.inherit_automation,
    upgrades_automation = excluded.upgrades_automation,
    maintenance_automation = excluded.maintenance_automation,
    security_automation = excluded.security_automation,
    restart_on_unhealthy = excluded.restart_on_unhealthy,
    unhealthy_restart_cooldown_sec = excluded.unhealthy_restart_cooldown_sec
`, r.ContainerID, r.ContainerName, r.UpdatePolicy, r.ValidateURL, r.ValidateMode, r.ValidateTimeoutSec, r.ValidateIntervalSec, bypassAI, skipHealthCheck, aiValidateLogs, rollback, inheritAutomation, upgradesAutomation, maintenanceAutomation, securityAutomation, restartOnUnhealthy, r.UnhealthyRestartCooldownSec)
	return err
}

func normalizeAutomationDefaults(r *ContainerRules) {
	if r.InheritAutomation {
		if !r.UpgradesAutomation {
			r.UpgradesAutomation = true
		}
		if !r.MaintenanceAutomation {
			r.MaintenanceAutomation = true
		}
		if !r.SecurityAutomation {
			r.SecurityAutomation = true
		}
	}
}
