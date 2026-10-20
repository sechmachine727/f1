# Proposal

## Why

The package records raw datagrams but the review surfaces expose almost none of that evidence. History lists little more than a track and lap count, Pit wall shows a live table even for a recorded session, and Analysis compares one channel with no guidance. A player reviewing a race, qualifying or practice session cannot see sector times, speed traces, corner performance or cross-driver comparisons without leaving the product.

## What Changes

- Close and catalogue every session with its type, track, regulation season, cars, completeness and recording state; keep race, qualifying and practice sessions distinguishable, with qualifying segments grouped as Q1/Q2/Q3 when they share a weekend link.
- Derive per-driver, per-lap analytics from the recorded datagrams: lap and sector times, average and top speed, trap speed, position and gaps, tyre compound/age/wear, fuel and ERS, pit in/out, track limits, and provisional corner speeds from track geometry when validated corner zones are unavailable.
- Store per-lap decimated speed traces and per-stint aggregates so charts do not rescan a multi-gigabyte recording, while the raw MCAP remains the source for replay and deeper queries.
- Rebuild History as a session library with type, date, track, duration, laps, best lap and completeness, for race, qualifying and practice. Selecting a session opens the recorded analytics view, not the live stream.
- Turn the recorded Pit wall into a dense analytics workspace with driver/team selectors, a session summary, lap and sector trends, speed and corner comparisons, tyre and fuel/ERS evolution, position and gap charts, and a sortable table of every lap.
- Turn Analysis into a comparison and engineer workspace: choose drivers, laps and channels, compare sector and corner performance, see deltas and theoretical bests, export, and ask the existing post-race engineer profile about the selected evidence.
- Add practice and qualifying views: session best and theoretical best from sector bests, consistency, run/stint structure, tyre usage, and qualifying segment progression.
- Remove the `Recorded · selection drives history views` copy. Recorded state must be evident from the session header and data, not a disclaimer.

## Capabilities

### New Capabilities

- `session-archive`: complete session lifecycle, participant roster, per-lap and per-stint derived records, sector and corner analytics, and retention of raw recordings.
- `analytics-workspace`: History, recorded Pit wall and Analysis behavior for race, qualifying and practice, including graphs, comparisons and engineer assistance.

### Modified Capabilities

None. Existing capabilities remain the underlying transport, recording and engineer contracts; this change adds the analytics layer above them.

## Impact

- `server/internal/history`: migrations for participants, lap sectors and analytics summaries; session close and segment grouping.
- `server/internal/server`: lap/sector extraction and incremental derivation at lap boundaries and session close.
- `server/internal/analysis`: cross-driver summaries, sector and corner comparisons, decimated traces.
- `server/internal/httpapi`: session analytics routes and recorded-session query support; existing routes remain compatible.
- `race_engineer_hub`: History, Pit wall and Analysis workspaces, charts, session-type views, engineer panel.
- `docs/rework/`: analytics data dictionary and qualification cases.

Dependencies: `go-telemetry-core`, `durable-race-history`, `telemetry-analysis-exports`, `scheduled-race-engineer`, `telemetry-workspaces`. The existing `prompts/v1` bundle and `post_race` profile are reused unchanged.
