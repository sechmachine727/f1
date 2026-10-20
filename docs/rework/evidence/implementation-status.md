# Implementation status

Updated: 2026-10-20 15:45 (Asia/Saigon). Branch: codex/f1-telemetry-redesign-impl. Baseline: c95b78c.

State: the nine original changes plus deployment-configuration, session-analytics and telemetry-traces-and-live-compare are implemented. Gate D remains externally blocked.

## Current step

telemetry-traces-and-live-compare: 26 of 26 tasks complete. Nothing is committed or published.

Complete: per-lap sample storage, restriction handling, decimation, backfill, the telemetry route, season-aware deploy labels, team colours, two-car live selection, the comparison grid, the selected-car stream buffer, the shared zoomable trace stack, recorded Pit wall and Analysis traces, History metadata, both browser passes, the five-minute load and the two-hour soak.

## What changed

- Migration 005 adds `lap_samples` and `session_participants.your_telemetry`. Each completed lap stores at most 900 points of `[session_time_ms, throttle, brake, gear, ers_j, deploy_mode, overtake_available, overtake_active]`.
- The tracker samples packet 6 and enriches energy from packet 7 and overtake state from packet 16. Restricted non-player energy is stored null; throttle, brake and gear stay because they are broadcast.
- Decimation keeps both ends, every gear change and every brake on/off transition. The method version is `lap-samples-v1` and `lap-analytics-v2`.
- Backfill now selects sessions with laps and no samples, including sessions that already have a summary.
- `GET /sessions/{id}/telemetry` serves stored samples for up to four cars with optional lap bounds. Empty is a success with `laps_available`.
- Live Pit wall compares two cars with team identity colours, deltas, tyre wear, damage and the same values Drive shows. Mode 3 is Overtake in 2025 and Boost in 2026; 2026 also draws an overtake availability strip.
- `TraceStack` gives throttle, brake, gear and ERS one time axis with wheel, button and keyboard zoom, drag and arrow pan, reset, and a follow-latest control on live data.
- Recorded Pit wall gains an Inputs tab with car and lap-range selection; Analysis gains time-aligned traces; History rows show circuit, date and duration.

## Passing evidence

- `go test -race ./server/...`: pass, all packages.
- `go vet ./server/...`, `go build ./server/cmd/f1tele`: pass.
- Five-minute rated run with samples: 614,400/614,400/614,400 transmitted/received/recorded, equal SHA-256 digests, 0 drops, RSS peak 86.6 MB, p95 1.04 ms. `release-qualification/traces-load-5m.json`.
- Two-hour soak with samples: 14,745,600/14,745,600/14,745,600 across 3 recordings, equal digests, 0 drops, RSS peak 101.9 MB, growth 9.9 MB over the last 90 minutes, p95 0.57 ms, max ingest lag 98 ms. `release-qualification/traces-soak-2h.json`.
- Frontend: `npm test` 60/60, `npm run lint` 0 errors, `tsc` shows only the two pre-existing errors in `ExpandableReportPanel.tsx` and `circuitData.ts`.
- Browser: live Pit wall and recorded Inputs at 1920x1080 and 2560x1440, keyboard row selection, keyboard zoom 100% to 154% live and 150% recorded, throttle and brake read from the selected-car buffer, no page errors. Screenshots `ui/traces-*.png`.
- `openspec validate telemetry-traces-and-live-compare --strict`: valid.

## Known limits

- The synthetic corpus has one constant compound, deploy mode and throttle pattern, so the rendered traces are flat. It has no restricted participant, so the restricted state rests on the tracker, route and comparison tests.
- `DetailedTyresView` carries the shared zoom controls but is not mounted by any workspace, so the browser pass could not exercise it.

## Next step

None for this change. Awaiting user review; no commit, push or archive was requested.

## Blocker

Gate D is unchanged and external: no configured model provider or key, no Windows 11 x64 host with microphone and game, no real 2026 capture. Sample capture needs none of these.

