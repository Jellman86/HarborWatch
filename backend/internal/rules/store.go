package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type ContainerRules struct {
	ContainerID           string `json:"containerId"`
	UpdatePolicy          string `json:"updatePolicy"` // auto, manual, locked
	ValidateURL           string `json:"validateUrl"`
	ValidateMode          string `json:"validateMode"` // http, docker, both
	ValidateTimeoutSec    int    `json:"validateTimeoutSec"`
	ValidateIntervalSec   int    `json:"validateIntervalSec"`
	AIValidateLogs        bool   `json:"aiValidateLogs"`
	AutoRollback          bool   `json:"autoRollback"`
	InheritAutomation     bool   `json:"inheritAutomation"`
	UpgradesAutomation    bool   `json:"upgradesAutomation"`
	MaintenanceAutomation bool   `json:"maintenanceAutomation"`
	SecurityAutomation    bool   `json:"securityAutomation"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS container_rules (
    container_id TEXT PRIMARY KEY,
    update_policy TEXT DEFAULT 'manual',
    validate_url TEXT DEFAULT '',
    validate_mode TEXT DEFAULT 'both',
    validate_timeout_sec INTEGER DEFAULT 45,
    validate_interval_sec INTEGER DEFAULT 2,
    ai_validate_logs INTEGER DEFAULT 0,
    auto_rollback INTEGER DEFAULT 1,
    inherit_automation INTEGER DEFAULT 1,
    upgrades_automation INTEGER DEFAULT 1,
    maintenance_automation INTEGER DEFAULT 1,
    security_automation INTEGER DEFAULT 1
);
`)
	if err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "validate_mode", "TEXT DEFAULT 'both'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "validate_timeout_sec", "INTEGER DEFAULT 45"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "validate_interval_sec", "INTEGER DEFAULT 2"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "ai_validate_logs", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "inherit_automation", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "upgrades_automation", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "maintenance_automation", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "security_automation", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (ContainerRules, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT container_id, update_policy, validate_url, validate_mode, validate_timeout_sec, validate_interval_sec, ai_validate_logs, auto_rollback, inherit_automation, upgrades_automation, maintenance_automation, security_automation 
FROM container_rules WHERE container_id = ?
`, id)

	var r ContainerRules
	var rollback, aiValidateLogs int
	var inheritAutomation, upgradesAutomation, maintenanceAutomation, securityAutomation int
	if err := row.Scan(&r.ContainerID, &r.UpdatePolicy, &r.ValidateURL, &r.ValidateMode, &r.ValidateTimeoutSec, &r.ValidateIntervalSec, &aiValidateLogs, &rollback, &inheritAutomation, &upgradesAutomation, &maintenanceAutomation, &securityAutomation); err != nil {
		if err == sql.ErrNoRows {
			return ContainerRules{
				ContainerID:           id,
				UpdatePolicy:          "manual",
				ValidateMode:          "both",
				ValidateTimeoutSec:    45,
				ValidateIntervalSec:   2,
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
	r.AIValidateLogs = aiValidateLogs == 1
	r.AutoRollback = rollback == 1
	r.InheritAutomation = inheritAutomation == 1
	r.UpgradesAutomation = upgradesAutomation == 1
	r.MaintenanceAutomation = maintenanceAutomation == 1
	r.SecurityAutomation = securityAutomation == 1
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
	_, err := s.db.ExecContext(ctx, `
INSERT INTO container_rules (container_id, update_policy, validate_url, validate_mode, validate_timeout_sec, validate_interval_sec, ai_validate_logs, auto_rollback, inherit_automation, upgrades_automation, maintenance_automation, security_automation)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(container_id) DO UPDATE SET
    update_policy = excluded.update_policy,
    validate_url = excluded.validate_url,
    validate_mode = excluded.validate_mode,
    validate_timeout_sec = excluded.validate_timeout_sec,
    validate_interval_sec = excluded.validate_interval_sec,
    ai_validate_logs = excluded.ai_validate_logs,
    auto_rollback = excluded.auto_rollback,
    inherit_automation = excluded.inherit_automation,
    upgrades_automation = excluded.upgrades_automation,
    maintenance_automation = excluded.maintenance_automation,
    security_automation = excluded.security_automation
`, r.ContainerID, r.UpdatePolicy, r.ValidateURL, r.ValidateMode, r.ValidateTimeoutSec, r.ValidateIntervalSec, aiValidateLogs, rollback, inheritAutomation, upgradesAutomation, maintenanceAutomation, securityAutomation)
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

func (s *Store) ensureColumn(ctx context.Context, columnName, columnDDL string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(container_rules)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, columnName) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE container_rules ADD COLUMN %s %s", columnName, columnDDL))
	return err
}
