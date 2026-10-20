CREATE TABLE IF NOT EXISTS session_participants (
  session_id TEXT NOT NULL REFERENCES sessions(id),
  car_idx INTEGER NOT NULL,
  name TEXT NOT NULL,
  team_id INTEGER,
  race_number INTEGER,
  ai INTEGER NOT NULL DEFAULT 0,
  first_seq INTEGER NOT NULL,
  last_seq INTEGER NOT NULL,
  PRIMARY KEY (session_id, car_idx)
);

CREATE TABLE IF NOT EXISTS lap_analytics (
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  car_idx INTEGER NOT NULL,
  lap INTEGER NOT NULL,
  lap_ms INTEGER,
  sector1_ms INTEGER,
  sector2_ms INTEGER,
  sector3_ms INTEGER,
  distance_m REAL,
  avg_speed_mps REAL,
  top_speed_mps REAL,
  trap_speed_mps REAL,
  position_start INTEGER,
  position_end INTEGER,
  gap_ahead_start_ms INTEGER,
  gap_leader_start_ms INTEGER,
  compound TEXT,
  tyre_age_laps INTEGER,
  fuel_start_kg REAL,
  fuel_end_kg REAL,
  ers_start_j REAL,
  ers_end_j REAL,
  pit_in INTEGER NOT NULL DEFAULT 0,
  pit_out INTEGER NOT NULL DEFAULT 0,
  valid INTEGER NOT NULL DEFAULT 1,
  track_limits INTEGER NOT NULL DEFAULT 0,
  complete INTEGER NOT NULL DEFAULT 1,
  conditions TEXT NOT NULL DEFAULT '',
  extra_json TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (session_id, branch_id, car_idx, lap)
);
CREATE INDEX IF NOT EXISTS lap_analytics_car ON lap_analytics(session_id, branch_id, car_idx, lap);

CREATE TABLE IF NOT EXISTS lap_traces (
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  car_idx INTEGER NOT NULL,
  lap INTEGER NOT NULL,
  channel TEXT NOT NULL,
  points BLOB NOT NULL,
  point_count INTEGER NOT NULL,
  decimated INTEGER NOT NULL DEFAULT 1,
  method_version TEXT NOT NULL,
  PRIMARY KEY (session_id, branch_id, car_idx, lap, channel)
);

CREATE TABLE IF NOT EXISTS analytics_stints (
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  car_idx INTEGER NOT NULL,
  kind TEXT NOT NULL,
  idx INTEGER NOT NULL,
  start_lap INTEGER NOT NULL,
  end_lap INTEGER NOT NULL,
  compound TEXT NOT NULL DEFAULT '',
  laps INTEGER NOT NULL,
  best_ms INTEGER,
  median_ms INTEGER,
  degradation_ms_per_lap REAL,
  wear_start_pct REAL,
  wear_end_pct REAL,
  fuel_start_kg REAL,
  fuel_end_kg REAL,
  ers_start_j REAL,
  ers_end_j REAL,
  conditions TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (session_id, branch_id, car_idx, kind, idx)
);

CREATE TABLE IF NOT EXISTS session_analytics (
  session_id TEXT PRIMARY KEY,
  branch_id TEXT NOT NULL,
  best_lap_ms INTEGER,
  theoretical_best_ms INTEGER,
  clean_laps INTEGER,
  total_laps INTEGER,
  avg_speed_mps REAL,
  top_speed_mps REAL,
  derived_at_ms INTEGER NOT NULL,
  method_version TEXT NOT NULL
);
