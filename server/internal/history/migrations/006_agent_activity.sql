CREATE TABLE IF NOT EXISTS agent_events (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  invocation_id TEXT,
  session_id TEXT,
  branch_id TEXT,
  profile TEXT,
  kind TEXT NOT NULL,
  at_ms INTEGER NOT NULL,
  duration_ms INTEGER,
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_sha256 TEXT NOT NULL DEFAULT '',
  truncated INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS agent_events_invocation ON agent_events(invocation_id, seq);
CREATE INDEX IF NOT EXISTS agent_events_session ON agent_events(session_id, branch_id, seq);

CREATE TABLE IF NOT EXISTS agent_compactions (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  invocation_id TEXT,
  trigger TEXT NOT NULL,
  focus TEXT,
  from_seq INTEGER NOT NULL DEFAULT 0,
  to_seq INTEGER NOT NULL DEFAULT 0,
  before_tokens INTEGER NOT NULL DEFAULT 0,
  after_tokens INTEGER NOT NULL DEFAULT 0,
  summary_json TEXT NOT NULL,
  model TEXT NOT NULL DEFAULT '',
  usage_json TEXT NOT NULL DEFAULT '{}',
  created_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS agent_compactions_conversation ON agent_compactions(conversation_id, created_at_ms);

CREATE TABLE IF NOT EXISTS agent_messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content_json TEXT NOT NULL,
  tool_call_id TEXT,
  invocation_id TEXT,
  compacted_into TEXT,
  created_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS agent_messages_conversation ON agent_messages(conversation_id, created_at_ms);

CREATE TABLE IF NOT EXISTS prompt_revisions (
  id TEXT PRIMARY KEY,
  profile TEXT NOT NULL,
  body TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS prompt_revisions_profile ON prompt_revisions(profile, created_at_ms);

CREATE TABLE IF NOT EXISTS prompt_current (
  profile TEXT PRIMARY KEY,
  revision_id TEXT,
  updated_at_ms INTEGER NOT NULL
);
