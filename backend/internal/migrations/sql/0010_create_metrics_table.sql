CREATE TABLE IF NOT EXISTS container_metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  container_id TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  cpu_percent REAL NOT NULL,
  memory_usage INTEGER NOT NULL,
  memory_limit INTEGER NOT NULL,
  pids INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_metrics_container_ts ON container_metrics(container_id, timestamp);
