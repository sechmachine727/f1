CREATE TABLE IF NOT EXISTS strategy_outcomes (
  invocation_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  observed_through_lap INTEGER NOT NULL,
  outcomes_json TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL
);
