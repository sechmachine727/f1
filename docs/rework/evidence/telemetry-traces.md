# Telemetry traces and live comparison

Evidence for the `telemetry-traces-and-live-compare` change. Written 2026-10-20.

## Stored samples

`lap_samples` holds one row per session, branch, car and completed lap. Each row carries `start_ms`, `lap_ms`, `point_count` and a JSON array of tuples:

    [session_time_ms, throttle, brake, gear, ers_j, deploy_mode, overtake_available, overtake_active]

`throttle` and `brake` are 0–1 fractions, `gear` is the selected gear, `ers_j` is stored energy in joules, and `deploy_mode` is the raw 0–3 enum. Missing or restricted values are `null`; a chart draws a gap rather than a zero.

Sampling happens on packet 6, which carries throttle, brake, gear and speed. ERS energy and deployment come from the latest packet 7, and overtake or aero state from the latest packet 16. When a participant's `m_yourTelemetry` is 0 and the car is not the player, ERS energy and deployment are stored as `null`; throttle, brake and gear stay available because the sender broadcasts them.

At lap close the trace is reduced to at most 900 points. The first and last samples are kept, every gear change and brake on/off transition is kept, and the remaining slots are filled evenly. A 90-second lap therefore keeps about one point per 100 ms while every shift and braking point survives.

The in-memory buffer holds at most about 4000 samples per car and lap. When a long lap exceeds that, the buffer is halved while keeping both ends and every gear or brake change, so the stored trace still covers the whole lap. The earlier behaviour truncated the newest samples and would have ended a long lap early.

The method version is `lap-samples-v1`; the lap analytics method version moved to `lap-analytics-v2` so new rows are identifiable. Existing recordings gain samples through the startup backfill, which skips sessions that already hold samples, rewrites nothing in the recording, and is safe to rerun.

## Read route

    GET /api/v1/sessions/{id}/telemetry?branch=<branch>&cars=0,1&from_lap=1&to_lap=20

- `cars` takes up to four indices; more returns `400 too_many_cars`, and a malformed list returns `400 bad_request`.
- Omitting `cars` uses the player car. Omitting the lap bounds returns every stored lap.
- The response names the session, branch and regulation season, then each car with name, team, restricted flag and laps. Each lap carries `lap`, `start_ms`, `lap_ms` and `points`.
- An unknown session returns 404. A session with no stored samples returns an empty car list and `laps_available: 0`, which the UI renders as a pending state.
- The route reads SQLite only. The in-progress lap is absent until the lap closes.

## Charts and colours

`TraceStack` renders three stacked panels on one axis: throttle solid and brake dashed, gear as steps, and ERS energy. Zoom starts at 100 percent with the full range; wheel, pinch, buttons and keys zoom, drag and arrows pan, reset restores the full range, and a navigator strip shows the visible window. Live charts follow the newest sample until the operator pans away, and the Follow latest control resumes.

Deployment bands use slate for none, indigo for medium, violet for hotlap and magenta for the push mode. Mode 3 is `Overtake` in 2025 and `Boost` in 2026, matching `docs/rework/reference/structures-2025.txt` and `structures-2026.txt`. A 2026 session also draws a separate overtake strip with teal for available and a darker teal for active. Unknown seasons show the raw mode with an unclassified label.

Team identity colours come from the appendix team ids and are lightened only until they reach 3:1 against `#0b0d11`; unknown, custom and future ids fall back to neutral grey. Status colours keep their existing blue, green, orange and red meanings and are never derived from team colour.

## Verification

| Check | Command | Result |
|---|---|---|
| History round trip and migration | `go test ./server/internal/history` | pass |
| Tracker samples, restriction, decimation, backfill | `go test ./server/internal/analytics` | pass |
| Route payload, bounds, empty session | `go test ./server/internal/httpapi` | pass |
| Season-aware deploy label | `go test ./server/internal/protocol ./server/internal/state ./server/internal/analysis` | pass |
| Full server suite | `go test -race ./server/...` | pass, all packages |
| Vet and build | `go vet ./server/...`, `go build -o f1tele ./server/cmd/f1tele` | pass |
| Frontend behaviour | `npm test` | 60 tests, 12 files, pass |
| Types | `npx tsc -p tsconfig.app.json --noEmit` | no new errors; two pre-existing errors remain (`ExpandableReportPanel.tsx`, `circuitData.ts`) |
| Lint | `npm run lint` | 0 errors |

Rated load with sample capture enabled (`release-qualification/traces-load-5m.json`): 614,400 transmitted, received and recorded, equal SHA-256 digests, 0 application drops, recorder stayed in `recording`, RSS peak 86.6 MB, observation-to-stream p95 1.04 ms. The run derived samples for the whole session.

Two-hour soak on the same code (`release-qualification/traces-soak-2h.json`, raw RSS in `traces-soak-2h-rss.csv`): 14,745,600 transmitted, received and recorded across 3 rotated recordings, equal digests, 0 drops, recorder still `recording`, RSS peak 101.9 MB, growth 9.9 MB over the last 90 minutes, observation-to-stream p95 0.565 ms and max ingest lag 98 ms. The catalog held 936 stored lap traces about 40 minutes in and 1,608 shortly after the first hour, so capture kept pace across the run.

The startup backfill also derived samples for the existing 24-lap review recording without changing its MCAP digest.

Browser pass at 1920x1080 and 2560x1440 with the synthetic 2026 feed and the recorded review session. Selecting a second row showed two cars, team-coloured headers, throttle and brake read from the selected-car buffer, and a NaN-free delta (`3% apart`). Keyboard `=` moved the live window from `0:49.154 – 1:01.509` at 100% to `0:51.213 – 0:59.450` at 154% and kept the follow-latest control offered. Recorded Inputs opened at 100% over 1:28 and `=` produced 150%. No page errors or failed requests in either pass. Screenshots: `ui/traces-live-1920.png`, `ui/traces-live-2560.png`, `ui/traces-live-zoom-1920.png`, `ui/traces-recorded-1920.png`, `ui/traces-analysis-1920.png`.

Rows also respond to Enter and Space, and the final pass selected the second car from the keyboard. The roster keeps its plain text except for the selected rows, which show the team identity colour and a matching left rule.

## Review findings fixed

- A long lap used to exceed the 4000-sample live buffer and lose its newest samples, ending the trace early. The buffer now halves while keeping both ends and every gear or brake change.
- The pathological decimation branch could repeat the last protected index. It now resamples the protected set evenly without duplicates, and bounds of one and two points are handled.
- Throttle and brake read `—` in the comparison until the first selected-car sample arrived, and the delta printed `3%% apart`. The grid falls back to the newest buffered sample and the unit is carried once.
- The live Pit wall took the throttle floor as a 0% trace and drew three panels as one axis block. Panels now carry titles and separators, and a constant ERS series gets a padded domain instead of three identical ticks.
- Rows were mouse-only; Enter and Space now toggle the comparison.

Limitations of that pass: the synthetic corpus reports one constant compound, deployment mode and throttle pattern, so the traces look flat; the corpus also has no restricted participant, so the restricted state is covered by the tracker, route and comparison tests rather than a screenshot. The pending-backfill message is covered by `src/test/recordedPending.test.tsx`, and the live no-lap message appeared before the first synthetic lap.

