# Implementation status

Updated: 2026-10-18 14:20 (Asia/Saigon). Branch: codex/f1-telemetry-redesign-impl. Baseline: c95b78c.

State: INCOMPLETE. Independent implementation and local checks are largely done; external qualification is unavailable.

## Current step

Rated 2-hour soak running in PTY session 9870 (started 13:52, expected end about 15:52). Evidence will land in docs/rework/evidence/release-qualification/rated-workload.json plus rss.csv and latency.csv. Poll that session; do not restart it.

## Added since the previous checkpoint

- Voice transcription and speech proxies with write-only keys, plus tests.
- Published game-mode and compound mappings taken from the supplied PDF appendix.
- Export rows carry their recording fragment; cancelled export test.
- Setup experiment append test; fixed a nested-query deadlock in setup records.
- Session views report real completed laps and recording state.
- Fresh-clone build fixed with a committed embed placeholder and scripts/build_web.sh.

## Passing evidence

- go test -race ./server/... : pass, all packages.
- go vet ./server/... : pass.
- go build ./server/cmd/f1tele : pass.
- Cross-builds CGO=0: linux/amd64, linux/arm64, windows/amd64 : pass.
- openspec validate --all --strict : 9/9.
- python3 docs/rework/validate_artifacts.py : PASS, 24 implementation tasks checked.
- Frontend: vitest 8/8, tsc, lint 0 errors, build : pass.
- Docker image builds (48.3 MB, distroless, nonroot); container health, UDP receipt, catalog and recordings survive restart.
- Rendered Drive at 1920x1080 and 2560x1440: no page scroll; screenshots in docs/rework/evidence/ui/.

## Failures found and fixed this pass

- Stream client race on subscription fields, caught by -race during a 30 s workload.
- Recorder identity read outside the lock in the writer goroutine.
- Channel catalog order depended on map iteration; now sorted and pinned by a golden file.
- Unavailable values rendered the word "stale" inside numeric slots.
- Alerts never cleared when a value stayed inside the band.
- Degradation slope crossed compound changes.
- Docker image could not write /data as nonroot.

## Since that checkpoint

- Full Pit wall: Drivers and Teams, selected driver, stint strip, pace chart, pit alternatives from the strategy API, and an engineer card with question input, invocation status and alert acknowledgment.
- History: season tabs, recorded sessions with laps and recording state, session selection that opens Pit wall with a persistent Recorded banner, and manual per-session cleanup reporting per-file refusals.
- Analysis: lap selection, distance-aligned compare, export with job polling and download link, plus a replay panel with play/pause, seek and 0.25-8x speeds.
- Server fix: `Snapshot` never exposed session time, so replay playheads were always zero. Added a playback clock and a regression test.
- Server fix: cleanup targeting only an active recording returned 200 with a failure entry; it now returns 409.
- Agent input is bounded to 512 metrics and 64 recent plus 64 critical events, with coverage counts and player-car ranking.
- Scheduled invocations now go through the scheduler's dispatch cursor; acceptance advances evidence only for the current branch. A budget block leaves the Safety Car event pending.
- Pit plans must match a legal deterministic candidate; retire requires direct failure evidence.
- Label contrast raised from 3.89:1 to 7.31:1; zoom/reflow captured.

## Next step

Collect soak results and update final-report.md. Then stop: provider, Windows host, microphone and real 2026 capture remain external.

## Blocker audit

Same external inputs as the previous turn: no configured model provider or key, no Windows 11 x64 host with microphone, no real 2026 game capture. Independent work continues where it does not need those.
