PRAGMA user_version = 1;

CREATE TABLE IF NOT EXISTS source_contexts (
  id TEXT PRIMARY KEY,
  recorder_install_id TEXT NOT NULL,
  sender_ip TEXT NOT NULL,
  next_reserved_seq INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS careers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  source_context_id TEXT NOT NULL REFERENCES source_contexts(id),
  regulation_season INTEGER,
  career_id TEXT REFERENCES careers(id),
  mode_family TEXT NOT NULL,
  track_id INTEGER,
  context_json TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL,
  ended_at_ms INTEGER,
  display_name TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS sessions_started ON sessions(started_at_ms, id);

CREATE TABLE IF NOT EXISTS fragments (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  source_context_id TEXT NOT NULL REFERENCES source_contexts(id),
  game_uid TEXT,
  link_ids_json TEXT NOT NULL DEFAULT '{}',
  source TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL,
  ended_at_ms INTEGER,
  first_seq INTEGER NOT NULL DEFAULT 0,
  last_seq INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS fragments_session ON fragments(session_id);

CREATE TABLE IF NOT EXISTS branches (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  parent_id TEXT REFERENCES branches(id),
  anchor_seq INTEGER,
  anchor_time_ms INTEGER,
  last_inherited_seq INTEGER,
  is_surviving INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS branches_session ON branches(session_id, is_surviving);

CREATE TABLE IF NOT EXISTS recordings (
  id TEXT PRIMARY KEY,
  fragment_id TEXT NOT NULL REFERENCES fragments(id),
  path TEXT NOT NULL,
  bytes INTEGER NOT NULL DEFAULT 0,
  state TEXT NOT NULL,
  durable_offset INTEGER NOT NULL DEFAULT 0,
  first_seq INTEGER NOT NULL DEFAULT 0,
  last_seq INTEGER NOT NULL DEFAULT 0,
  sha256 TEXT,
  created_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS recordings_fragment ON recordings(fragment_id);

CREATE TABLE IF NOT EXISTS laps (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  branch_id TEXT NOT NULL REFERENCES branches(id),
  car_idx INTEGER NOT NULL,
  lap INTEGER NOT NULL,
  start_seq INTEGER NOT NULL,
  end_seq INTEGER NOT NULL,
  time_ms INTEGER,
  validity_json TEXT NOT NULL DEFAULT '{}',
  summary_json TEXT NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX IF NOT EXISTS laps_unique ON laps(session_id, branch_id, car_idx, lap);
CREATE INDEX IF NOT EXISTS laps_range ON laps(session_id, branch_id, car_idx, lap);

CREATE TABLE IF NOT EXISTS stints (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  branch_id TEXT NOT NULL REFERENCES branches(id),
  car_idx INTEGER NOT NULL,
  compound TEXT,
  start_lap INTEGER NOT NULL,
  end_lap INTEGER,
  summary_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  branch_id TEXT NOT NULL REFERENCES branches(id),
  seq INTEGER NOT NULL,
  type TEXT NOT NULL,
  severity TEXT NOT NULL,
  payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS events_range ON events(session_id, branch_id, seq);

CREATE TABLE IF NOT EXISTS gaps (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  branch_id TEXT NOT NULL REFERENCES branches(id),
  from_seq INTEGER NOT NULL,
  to_seq INTEGER NOT NULL,
  reason TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS invocations (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  status TEXT NOT NULL,
  input_json TEXT NOT NULL DEFAULT '{}',
  output_json TEXT,
  usage_json TEXT NOT NULL DEFAULT '{}',
  provider_state_blob TEXT,
  profile TEXT NOT NULL,
  trigger_ids_json TEXT NOT NULL DEFAULT '[]',
  started_at_ms INTEGER,
  ended_at_ms INTEGER,
  notification_disposition TEXT NOT NULL DEFAULT 'none',
  error_json TEXT,
  continuation_reset_reason TEXT
);
CREATE INDEX IF NOT EXISTS invocations_range ON invocations(session_id, branch_id, started_at_ms);

CREATE TABLE IF NOT EXISTS branch_strategy (
  branch_id TEXT PRIMARY KEY REFERENCES branches(id),
  dispatch_cursor_json TEXT NOT NULL DEFAULT '{}',
  evidence_cursor INTEGER,
  accepted_invocation_id TEXT
);
