CREATE TABLE IF NOT EXISTS container_rules (
    container_id TEXT PRIMARY KEY,
    container_name TEXT NOT NULL DEFAULT '',
    update_policy TEXT DEFAULT 'manual',
    validate_url TEXT DEFAULT '',
    validate_mode TEXT DEFAULT 'both',
    validate_timeout_sec INTEGER DEFAULT 45,
    validate_interval_sec INTEGER DEFAULT 2,
    bypass_ai INTEGER DEFAULT 0,
    skip_health_check INTEGER DEFAULT 0,
    ai_validate_logs INTEGER DEFAULT 0,
    auto_rollback INTEGER DEFAULT 1,
    inherit_automation INTEGER DEFAULT 1,
    upgrades_automation INTEGER DEFAULT 1,
    maintenance_automation INTEGER DEFAULT 1,
    security_automation INTEGER DEFAULT 1,
    restart_on_unhealthy INTEGER DEFAULT 0,
    unhealthy_restart_cooldown_sec INTEGER DEFAULT 0
);
