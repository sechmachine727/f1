# Implementation status

Updated: 2026-10-18 14:20 (Asia/Saigon). Branch: codex/f1-telemetry-redesign-impl. Baseline: c95b78c.

State: INCOMPLETE. Independent implementation and local checks are largely done; external qualification is unavailable.

## Current step

Rated 2-hour soak running (started 13:52, expected end about 15:52). Evidence will land in docs/rework/evidence/release-qualification/rated-workload.json plus rss.csv and latency.csv.

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

## Next step

Collect soak results and update final-report.md. Then stop: provider, Windows host, microphone and real 2026 capture remain external.

## Blocker audit

Same external inputs as the previous turn: no configured model provider or key, no Windows 11 x64 host with microphone, no real 2026 game capture. Independent work continues where it does not need those.
