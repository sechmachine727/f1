# Design

## Decisions

The raw MCAP recording is already the complete source: every datagram for every car is written and verified by the rated workload. This change adds a derived analytics layer beside it, not a second recorder. Derivation runs incrementally at each lap boundary and is finalized when a session closes, so a race does not rescan its whole file to show a completed lap.

Session classification uses the game session type from packet 1: practice 1-4, qualifying 5-7 (Q1-Q3), race 10-12, sprint 14, time trial 13, unknown otherwise. Sessions sharing a weekend link are grouped for display, with each qualifying segment keeping its own session row and label. A session closes on a new logical session or after a configurable inactivity window (default ten minutes), and the close records `ended_at_ms`, completeness, and the last durable sequence.

## Data model

Migration `004_analytics.sql` adds:

- `session_participants(session_id, car_idx, name, team_id, race_number, ai, first_seq, last_seq)`, upserted as packet 4 arrives.
- `lap_analytics(session_id, branch_id, car_idx, lap, lap_ms, sector1_ms, sector2_ms, sector3_ms, distance_m, avg_speed_mps, top_speed_mps, trap_speed_mps, position_start, position_end, gap_ahead_start_ms, gap_leader_start_ms, compound, tyre_age_laps, fuel_start_kg, fuel_end_kg, ers_start_j, ers_end_j, pit_in, pit_out, valid, track_limits, complete, conditions, extra_json)`, one row per car and lap.
- `lap_traces(session_id, branch_id, car_idx, lap, channel, points BLOB, point_count, method_version)`, distance-bucketed decimated traces.
- `stints(session_id, branch_id, car_idx, index, start_lap, end_lap, compound, laps, best_ms, median_ms, degradation_ms_per_lap, wear_start_pct, wear_end_pct, fuel_start_kg, fuel_end_kg, ers_start_j, ers_end_j, conditions)` and `runs` with the same shape separated by pit visits for practice and qualifying.
- `session_analytics(session_id, branch_id, best_lap_ms, theoretical_best_ms, clean_laps, total_laps, avg_speed_mps, top_speed_mps, completeness, derived_at_ms, method_version)`.

The existing `laps` table remains the durable lap identity used by compare and exports. The new row is keyed by the same session, branch, car and lap, so old recordings can be backfilled without touching their files.

## Derivation

Sector times combine the minute and millisecond fields from packet 2: `minutes*60000 + ms`, with either part missing producing an unavailable sector. Lap distance uses the `m_totalDistance` delta when both endpoints are present; otherwise it integrates `m_lapDistance` and handles the wrap at the start line. Average speed is distance divided by lap time. Top speed is the maximum observed `m_speed` in the lap; trap speed is packet 2's per-car fastest trap value with its lap number.

Gaps and positions come from packet 2 at the first and last accepted sample of the lap. Tyre compound and age come from packet 7; wear, damage and blisters from packet 10; fuel, ERS and deploy mode from packet 7; pit status from packet 2; track limits from packet 2 and packet 10 where available. A value that is restricted for an opponent remains null with its availability, never zero.

Corner performance uses a validated `TrackProfile` entry/exit distance when the configuration supplies one. When it does not, the server derives provisional zones from the bundled circuit outline: each corner's nearest outline distance plus a 50 m entry and exit window. Results are labeled `provisional` and the UI does not call them optimal. A track with neither a profile nor a usable outline reports corner speed as unavailable.

Traces are stored per lap and channel at the configured point cap, using first/min/max/last per distance bucket. Position, gap, tyre and fuel series are stored per session at a bounded cadence. Charts read these rows; only replay and ad hoc range queries read MCAP.

## Backfill

On startup, a catalog migration marks sessions that have no analytics. A bounded worker derives one recording at a time with a 30-second deadline per chunk, writes rows in a transaction, and resumes from the last completed lap. Existing recording bytes and hashes are not modified. A repeated run skips rows already present. The worker yields to live recording and export jobs.

## API and UI data flow

New read routes, added to the versioned contract:

- `GET /api/v1/sessions/{id}/analytics` returns the session summary, participants, stints, runs, corner aggregates and available chart series.
- `GET /api/v1/sessions/{id}/laps/{lap}?car=` returns one lap's analytics and decimated traces.
- `GET /api/v1/sessions/{id}/compare` accepts two to four `(car, lap)` pairs and channels and returns aligned series, sector/corner deltas and theoretical best.

The existing live stream remains the only live source. The frontend adds a recorded-session store keyed by session ID; History opens it and the recorded Pit wall and Analysis read it. That store never merges live state, which removes the current mismatch where a recorded banner sits above live values. The `Recorded · selection drives history views` copy is deleted.

The recorded Pit wall uses a session header plus tabs: Overview, Laps, Sectors, Speed, Tyres, Fuel/ERS, Stints, Corners. Each tab uses the same selected drivers and lap range, with two to four series. History rows show type, track, date, laps, best lap and completeness. Analysis adds a comparison builder and a `post_race` engineer panel using the existing prompt bundle and tools; it sends the selected session, branch and lap IDs as request context.

Practice and qualifying reuse the same components with type-specific summaries: runs and consistency for practice; Q1/Q2/Q3 tabs and progression for qualifying. When a segment or run is absent, the view says so instead of filling it.

## Risks / trade-offs

- Derived rows duplicate some raw evidence. They are sized for review, while MCAP remains the source for exact replay and export.
- Backfilling a large archive is expensive. It is bounded, resumable and skipped when rows exist; the UI marks a session as still deriving.
- Provisional corner zones are geometry-based, not calibrated. They are labeled and never presented as optimal.
- Deleting a raw recording leaves derived charts but disables replay; the UI states that explicitly rather than failing silently.

## Migration

New tables and routes are additive. Existing sessions, laps, recordings and configuration remain valid. Backfill is optional for a session until it is opened, and its progress is visible in the session row.
