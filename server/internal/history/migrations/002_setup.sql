CREATE TABLE IF NOT EXISTS setup_records (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  invocation_id TEXT NOT NULL,
  output_json TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS setup_sources (
  id TEXT PRIMARY KEY,
  record_id TEXT NOT NULL REFERENCES setup_records(id),
  source_json TEXT NOT NULL,
  content_hash TEXT,
  inspected INTEGER NOT NULL,
  truncated INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS setup_experiments (
  id TEXT PRIMARY KEY,
  record_id TEXT NOT NULL REFERENCES setup_records(id),
  baseline_json TEXT NOT NULL,
  trial_json TEXT NOT NULL,
  conditions TEXT NOT NULL,
  outcome TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL
);
