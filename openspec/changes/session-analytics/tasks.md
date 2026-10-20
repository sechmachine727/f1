# Tasks

## 1. Session archive

- [x] 1.1 Add the analytics migration for participants, lap analytics, traces, stints, runs and session summaries, preserving the existing laps table. Verify a migrated catalog opens, old recordings remain readable, and the migration is repeatable.
- [x] 1.2 Catalogue every participant from packet 4 with first and last observed sequence. Verify a 24-car fixture produces 24 rows and a late joiner is added without losing earlier rows.
- [x] 1.3 Classify sessions from game session type and group qualifying segments by weekend link. Verify race, Q1/Q2/Q3, practice and time-trial fixtures receive the right labels and grouping.
- [x] 1.4 Close sessions on a new logical session or the inactivity window and record end time, completeness and last durable sequence. Verify a stopped fixture closes once and a resumed session does not.

## 2. Lap and stint derivation

- [x] 2.1 Derive lap time, all three sector times, distance, average speed, top and trap speed at each lap boundary. Verify a fixture with minutes-bearing sectors and a missing sector part; zero is not substituted.
- [x] 2.2 Capture position, gaps, compound, tyre age, fuel, ERS, pit flags, validity and track limits at lap boundaries, preserving restricted opponent values as unavailable. Verify player and restricted-rival fixtures.
- [x] 2.3 Build stints for race and runs for practice/qualifying, with best, median, degradation, wear and fuel/ERS change. Verify a six-lap stint, a pit-separated practice run and a missing-data case.
- [x] 2.4 Derive corner minimum and average speed from validated profiles and from provisional circuit geometry, labeling provisional results. Verify a validated profile, a geometry-only track and a track with neither.

## 3. Traces and backfill

- [x] 3.1 Store decimated per-lap traces with first/min/max/last buckets and a labeled method version. Verify a trace over the point cap stays within it and preserves extremes.
- [x] 3.2 Add the bounded, resumable backfill for existing recordings without rewriting files. Verify a pre-analytics recording backfills once, a repeat run adds no rows, and the file hash is unchanged.

## 4. API

- [x] 4.1 Add session analytics, lap detail and recorded compare read routes to the versioned contract and server. Verify handlers return stored data for a completed session and explicit unavailable states for missing traces.
- [x] 4.2 Document the analytics routes and data dictionary. Verify every field returned by the routes is described with its derivation and availability rule.

## 5. History

- [x] 5.1 Rebuild History as a session library with type, track, date, laps, best lap, completeness and state filters. Verify race, qualifying and practice fixture screenshots at 1920x1080 and 2560x1440.
- [x] 5.2 Make a recorded selection load a recorded-session store that never reads live metrics, and delete the `Recorded · selection drives history views` copy. Verify a recorded session opened during a live fixture shows only recorded values.

## 6. Recorded Pit wall

- [x] 6.1 Build the session header, driver selector and Overview/Laps/Sectors/Speed/Tyres/Fuel/Stints/Corners tabs for race review. Verify each tab renders from stored analytics with two to four compared drivers.
- [x] 6.2 Add the sortable all-laps table and team aggregation with contributing cars and missing values. Verify 24-car and restricted-rival fixtures.
- [x] 6.3 Add bounded chart rendering and loading/empty/error/deriving states. Verify a full-race fixture and an analytics-not-ready fixture.

## 7. Analysis and engineer help

- [x] 7.1 Add the comparison builder for two to four laps and multiple channels with aligned traces, sector and corner deltas and theoretical best. Verify clean, mixed-condition and gapped-trace fixtures.
- [x] 7.2 Add the post-race engineer panel with selected session/branch/laps context and configured/missing-provider states. Verify a fake provider round trip, an invalid output and the missing-provider state; the authored prompt bundle remains byte-identical.
- [x] 7.3 Keep export and replay integrated with the recorded session. Verify an export of the selected range succeeds and replay remains available only when the raw recording exists.

## 8. Practice and qualifying

- [x] 8.1 Build the practice view with runs, consistency, theoretical best, tyre/fuel usage and corner performance. Verify a multi-run fixture and a single-run fixture.
- [x] 8.2 Build the qualifying view with Q1/Q2/Q3 segment tabs and progression from the weekend link. Verify three-segment and single-segment fixtures.

## 9. Integration

- [x] 9.1 Run `go test -race ./server/...`, frontend tests/typecheck/lint/build, strict OpenSpec validation and the artifact validator. Verify all pass and store screenshots and exact command results in evidence.
- [x] 9.2 Re-run the rated workload far enough to prove analytics derivation does not drop datagrams or stall recording, and record the result. Verify counts and digests remain exact.

## Workflow follow-up

- Archive this change only after implementation evidence and requested review; publishing requires separate authorization.
