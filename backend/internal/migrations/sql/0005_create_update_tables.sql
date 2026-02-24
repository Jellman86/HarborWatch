CREATE TABLE IF NOT EXISTS update_runs (
  id TEXT PRIMARY KEY,
  container_id TEXT NOT NULL DEFAULT '',
  container_name TEXT NOT NULL DEFAULT '',
  target_image TEXT NOT NULL,
  validate_url TEXT NOT NULL,
  status TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  ai_analysis TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS update_steps (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  step TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT NOT NULL,
  ts INTEGER NOT NULL
);
