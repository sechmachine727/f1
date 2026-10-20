CREATE TABLE IF NOT EXISTS lap_samples (
  session_id TEXT NOT NULL,
  branch_id TEXT NOT NULL,
  car_idx INTEGER NOT NULL,
  lap INTEGER NOT NULL,
  start_ms INTEGER NOT NULL,
  lap_ms INTEGER NOT NULL,
  points BLOB NOT NULL,
  point_count INTEGER NOT NULL,
  method_version TEXT NOT NULL,
  PRIMARY KEY (session_id, branch_id, car_idx, lap)
);
CREATE INDEX IF NOT EXISTS lap_samples_car ON lap_samples(session_id, branch_id, car_idx, lap);

