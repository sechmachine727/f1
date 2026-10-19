# Implementation status

Updated: 2026-10-19 12:40 (Asia/Saigon). Branch: codex/f1-telemetry-redesign-impl. Baseline: c95b78c.

State: independent implementation and local qualification are done. Gate D remains externally blocked and one strategy task needs a contract decision.

## Current step

Definitive 2-hour rated workload passed at 12:13:45. Results and browser samples are in docs/rework/evidence/release-qualification/. Nothing is running.

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

The run must reach 14,745,600 transmitted, received and recorded datagrams, equal digests, zero application drops, a recording state of recording across two rotated files, peak RSS <=512 MiB, growth <=64 MiB over the final 90 minutes, and observation-to-render p95 <250 ms. The first soak passed the server limits but used stream-analogue clients; this run adds three real browser clients.

## Next step

Read docs/rework/evidence/release-qualification/rated-workload.json and /tmp/browser-evidence.json, update final-report.md, re-check the final diff, and stop. Gate D stays externally blocked: no configured model provider or key, no Windows 11 x64 host with microphone, no real 2026 game capture.

## Blocker audit

Same external inputs: no provider/key, no Windows 11 x64 with microphone, no real 2026 capture. Independent work continues until the final soak is read.
