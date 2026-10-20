# Implementation status

Updated: 2026-10-20 12:30 (Asia/Saigon). Branch: codex/f1-telemetry-redesign-impl. Baseline: c95b78c.

State: the nine original changes plus deployment-configuration and session-analytics are implemented and locally verified. Gate D remains externally blocked.

## Current step

Both new changes are complete: 24/24 session-analytics tasks and 12/12 deployment-configuration tasks. A five-minute rated load with analytics enabled recorded 614,400/614,400/614,400 datagrams with equal digests and zero drops. Nothing is running.

## Deployment configuration

- `server/internal/config/env.go` applies the documented `F1TELE_*` variables with CLI > env > file > default precedence, records a source per value, and fails startup on invalid input with the variable name.
- Environment-owned values reject `PATCH /config` with `409 environment_locked`; secrets stay redacted.
- `GET /config` reports sources, restart-only keys and the game-facing UDP advertise host; `/health` reports the bound and advertised addresses.
- The web Settings workspace is a read-only grouped report with source labels and restart badges. Tests cover precedence, invalid values, environment locking, endpoint creation, advertise host and the operator reference table.
- The portable Windows shipment is configured in the native control-center Settings page, which reads the same effective values, marks environment-locked rows and opens the configuration folder. Tests cover configured, missing-secret, restart and environment-locked rows.

## Session analytics

- Migration 004 adds participants, lap analytics, traces, analytics stints and session summaries.
- The tracker derives lap and sector times, distance, average/top/trap speed, positions and gaps, compound/age, fuel/ERS, pit and validity flags, provisional corner speeds, stints, practice/qualifying runs and a session summary.
- Backfill derives existing recordings once without rewriting them; the file hash is checked in a test.
- Recorded History, Pit wall and Analysis read stored analytics only, with tabs, charts, comparison, theoretical best, export, replay, consistency and the post-race engineer panel.
- Evidence: `docs/rework/evidence/session-analytics.md`, screenshots `analytics-*.png`, and `release-qualification/analytics-load-5m.json`.

Completed since the last checkpoint: branch-scoped previous advice is loaded from the catalog, measured outcomes compare planned pit laps and compounds with observed laps, outcome rows are forward-only in SQLite (migration 003), and `server/README.md` documents model limits, profile editing and where advice lives. `analysis.Summaries` now reports observed top speed, packet-2 trap best with lap, last clean-lap distance over time, an explicitly unsupported corner average, and a cache keyed by the lap checkpoint.

Prior full runs: the first two-hour run (server limits passed) used shape-only clients. The second and third runs passed every server gate but one Drive browser peaked at 513.5 and 564.5 MiB against the 512 MiB browser limit; neither is claimed as passing. Measurements isolated the cause: at a 60 Hz subscription the renderer grew steadily (about 2-4 MiB/min) with a flat 30 MiB JS heap; at 20 Hz the same page stayed flat at 303-304 MiB for ten minutes. Deltas now update the live map in place, the stale check reads three channels, only rendered channels are subscribed, React commits coalesce at 10 Hz, and the UI socket subscribes at 20 Hz. That is a deliberate deviation from the workload document's 60 Hz browser line, recorded in the final report.

The earlier full run was interrupted after 1h46m to add the requested Drive changes, so it cannot be quoted as the final soak. Its server limits passed while it ran.

## Drive redesign (latest)

The user asked for a denser focused view, larger graphs, bars for remaining energy/fuel, car issues under the tyres, and a longer field map with team-coloured dots and abbreviated driver names. Implemented:

- Tyre block reduced to the values it displays, with pressure and brake temperature under each corner.
- Position, gap ahead (large), gap behind from the trailing car's own delta, and gap to the leader.
- Fuel margin with tank and remaining-lap bars; ERS with a session-peak bar, deploy mode and harvest limit.
- Car issues: wing, floor, diffuser, sidepod, gearbox, engine, DRS and ERS fault bars with severity words, pit stops, penalties, warnings and per-corner tyre/brake damage.
- Field map: all 24 cars on three greedy lanes, team colours, three-character driver codes above each dot, and a team legend.
- Analytic traces: completed valid lap times, worst-corner tyre wear, and live gap ahead. The previous gap/ERS/fuel traces repeated numbers already on screen.

Rendered and inspected at 1920x1080, 2560x1440 and 200% reflow. No page scroll at 1080p/1440p; no horizontal overflow; no console errors.

## Passing evidence

- go test -race ./server/... : pass, all packages.
- go vet ./server/... : pass.
- go build ./server/cmd/f1tele : pass.
- Cross-builds CGO=0: linux/amd64, linux/arm64, windows/amd64 : pass.
- openspec validate --all --strict : 9/9.
- python3 docs/rework/validate_artifacts.py : PASS, 32 implementation tasks checked.
- Frontend: vitest 8/8, tsc, lint 0 errors, build : pass.
- dotnet test windows/F1Telemetry.ControlCenter.Tests : 9/9.
- Docker image f1tele:qualification built; digest c1a40705dc88b571a34d96fd8a48bf55f8c318f69d14e34426755be9691ea11a.
- Rendered all six workspaces at 1920x1080 and 2560x1440, plus 200% reflow; no horizontal overflow; Drive has no page scroll.
- Rendered loading, error, stale, recording-stopped and AI-disabled states; focus returns to Cleanup after Cancel; reduced motion leaves zero moving elements.
- Contrast on rendered elements: nav 12.86:1, active tab 6.49:1, secondary label 7.31:1.

## Defects found and fixed this pass

- Stale metrics reverted to available once the whole feed exceeded two seconds.
- Analysis could not read an active MCAP file; it now scans the unfinalized data section.
- Aligned comparison series dropped their lap ID, collapsing both lines onto one key.
- Decimation ran before distance alignment, leaving roughly one-second sample gaps.
- The qualification mix packed each packet type into one burst per second; packet types are now interleaved at their declared rates.
- Lap end sequences included the first packet of the next lap; ends now point at the previous sequence.
- Drive hardcoded a restricted rival gap and showed "stopped user" and "1 laps"; it now uses measurement availability, readable recording states and correct pluralisation.
- Non-Drive workspaces overflowed by the 49px navigation height; all now fit the viewport.
- The engineer card claimed "Live" with no endpoint configured; it now shows Not configured and hides the input.
- Replay on an active recording returned 500; it now explains that replay is available after the recording finishes.
- Network failures showed "TypeError: Failed to fetch"; the API helper now reports "Cannot reach the server."
- Cleanup confirmation lost keyboard focus; Cancel now receives focus and returns it to Cleanup.

## Final soak checks

The run must reach 14,745,600 transmitted, received and recorded datagrams, equal digests, zero application drops, a recording state of recording across rotated files, peak RSS <=512 MiB, growth <=64 MiB over the final 90 minutes, browser peaks <=512 MiB each, and observation-to-render p95 <250 ms.

## Next step

Read the new rated-workload.json and /tmp/browser-evidence.json, update final-report.md and artifact hashes, re-check the final diff, and stop. Gate D stays externally blocked: no configured model provider or key, no Windows 11 x64 host with microphone, no real 2026 game capture.

## Blocker audit

Same external inputs: no provider/key, no Windows 11 x64 with microphone, no real 2026 capture. Independent work continues until the final soak is read.
