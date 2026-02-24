CREATE TABLE IF NOT EXISTS scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  critical INTEGER NOT NULL,
  high INTEGER NOT NULL,
  medium INTEGER NOT NULL,
  low INTEGER NOT NULL,
  unknown INTEGER NOT NULL,
  raw_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS malware_scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  container_name TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  infected INTEGER NOT NULL,
  threats_found TEXT NOT NULL,
  raw_output TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scan_jobs (
  job_id TEXT PRIMARY KEY,
  container_name TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  source TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  completed_at INTEGER NOT NULL DEFAULT 0
);
