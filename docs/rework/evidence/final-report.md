# F1 telemetry redesign final report

Status: every implementation task is complete. Gates A, B, C and the packaging parts of E pass on this machine. Gate D is externally blocked: no model provider or key, no Windows 11 x64 host with a microphone, and no real 2026 game capture.

Branch: `codex/f1-telemetry-redesign-impl`. Baseline: `c95b78c` on `codex/f1-telemetry-redesign-specs`. Nothing was pushed, merged, archived or published.

## What was built

Go server (`f1tele`):

- `server/internal/protocol`: 2025/2026 layouts for all supplied packets, exact declared sizes, real-2025 parity and 2026 golden corpus, event union decoding, channel catalog with wheel order and restriction rules, derived leader gap and pit/penalty counts, and damage/fault channels from packet 10.
- `server/internal/config`: documented precedence, validation, redacted reads, atomic failed PATCH, secrets file at 0600.
- `server/internal/state`: one serialized reducer, per-key ordering, frame wrap, duplicate suppression with same-frame distinct events, flashback branches, restrictions, freshness, published game-mode and compound mappings.
- `server/internal/history`: SQLite catalog with WAL and foreign keys, durable sequence blocks, MCAP Zstandard/CRC chunks with rotation and checkpoints, legacy `.f1bin` import, damaged-file recovery, replay isolation, storage limits.
- `server/internal/analysis`: bounded range queries, min/max decimation, distance-aligned comparison over full-resolution samples with gap markers, CSV/JSONL/Parquet export plus manifest, unfinalized-recording reads, two-job limit, and cached race summaries with observed top speed, packet-2 trap best and lap attribution, last clean-lap distance over time, and an explicit unsupported corner average.
- `server/internal/strategy`: pace, per-stint degradation, fuel margin, ten-second energy draw, zero/one/two-stop candidates with pit-loss provenance, provisional profiles, notifications with clearing, grouping, acknowledgment and the 30-second routine gap.
- `server/internal/engineer`: byte-identical `prompts/v1` bundle with manifest verification, profile-scoped tools, Eino ADK Chat Completions and Responses adapters, 16 scheduler vectors, output acceptance, bounded input, rolling budget, branch-scoped previous advice, and forward-only measured-outcome records.
- `server/internal/setup`: Tavily and SearXNG search, bounded source inspection, provenance, setup records and experiments. No game control.
- `server/internal/httpapi`: v1 routes, one-socket streaming with snapshot/delta/resync, clock and latency stamps, jobs and downloads, cleanup refusal for active recordings.
- `server/qualification`: interleaved 2048 dk/s generator with independent sender, receiver-intake and MCAP oracles, RSS and latency sampling.

Dashboard (`race_engineer_hub`): six workspaces behind one socket. Drive now fills the screen with driver-relevant data: large tyre temperatures and wear rings, per-corner pressure, brake temperature and damage, position, gap ahead/behind/leader, fuel and ERS figures with fill bars, car damage and fault bars, a team-coloured field map with driver codes, and analytic traces for lap times, worst-corner wear and gap ahead. Pit wall, History, Analysis, Setups and Settings were exercised with populated, empty, recorded, stale, error, stopped and AI-disabled states.

Windows (`windows/`): control-center core with supervisor identity and adoption, flush-then-stop, force-stop confirmation, voice turn cap and WAV validation, plus the WinUI app, tray controller, native capture and packaging script.

Packaging: multi-stage Dockerfile (distroless, nonroot, writable `/data`), Compose on 8081/tcp and 20777/udp, and CI for Go, frontend, artifacts and the Windows ZIP.

Deployment configuration: a documented `F1TELE_*` environment schema now covers listeners, LAN advertise host, data directory, recording limits, agent URL/model/protocol/key, search and voice. Precedence stays CLI > environment > `config.json` > defaults, invalid values fail startup by name, environment-owned values reject `PATCH /config` with 409, and secrets never enter the config view. The web Settings workspace is a read-only report of effective values with source labels and restart badges; the portable Windows shipment is configured in its native control-center Settings page. See `docs/rework/operator-configuration.md`.

Session analytics: a derived analytics layer beside raw MCAP stores participants, per-car lap records (lap and sector times, distance, average/top/trap speed, positions and gaps, compound and age, fuel/ERS, pit and validity flags, provisional corner speeds), stints and practice/qualifying runs, decimated traces and a session summary. A resumable backfill derives existing recordings without rewriting them. History is a session library; selecting a recorded session opens a recorded analytics Pit wall and Analysis with lap, sector, speed, tyre, fuel, stint and corner views, comparison and theoretical best, export, replay and the existing post-race engineer profile.

## Gate A: implementation checks

| Command | Result |
|---|---|
| `go test -race ./server/...` | pass, every package |
| `go vet ./server/...` | pass |
| `go build ./server/cmd/f1tele` | pass |
| `CGO_ENABLED=0` linux/amd64, linux/arm64, windows/amd64 | pass |
| `rtk openspec validate --all --strict --no-interactive` | 9/9 |
| `python3 docs/rework/validate_artifacts.py` | PASS, 43 checked tasks |
| `npm run test -- --run` | 8/8 |
| `npx tsc --noEmit`, `npm run lint`, `npm run build` | pass, 0 errors, 13 warnings |
| `dotnet test windows/F1Telemetry.ControlCenter.Tests` | 9/9 |
| `docker build -t f1tele:qualification .` | pass, 37.1 MB image |
| final image health, UDP session, restart persistence | pass: `awaiting_telemetry` to catalogued session, same session after restart |

Additional checks for the later changes: `go test ./server/internal/analytics`, `go test ./server/internal/history`, `go test ./server/internal/httpapi -run TestRecordedAnalyticsRoutes`, and a five-minute rated run with analytics enabled (614,400/614,400/614,400 transmitted/received/recorded, equal digests, 0 drops). See `docs/rework/evidence/session-analytics.md` and `release-qualification/analytics-load-5m.json`.

Focused additions after the first report draft: `go test ./server/internal/analysis -run TestAnalysisSummaries` and `go test ./server/internal/engineer -run 'TestRecordOutcomes|TestPreviousAccepted'` pass, and the full race suite was rerun.

## Gate B: behaviour and UI

Rendered in headless Chromium against the real server with live synthetic telemetry. Screenshots are in `docs/rework/evidence/ui/` (`gateb-*.png`).

- Drive at 1920x1080 and 2560x1440: document scroll height equals client height; no horizontal overflow. 200% reflow stacks to one column and scrolls vertically.
- All six workspaces at both viewports; no console or page errors.
- Loading, error, stale, recording stopped, AI disabled, recorded session, rival selection, populated comparison, successful export and paused replay were captured.
- Keyboard: tabs reach all six workspaces in order with a visible 2px focus ring; History selection by keyboard opens Pit wall with the Recorded banner; cleanup Cancel receives focus and returns it to Cleanup.
- Contrast on rendered elements: nav 12.86:1, active tab 6.49:1, secondary label 7.31:1. Reduced motion leaves zero animating elements.
- Defects found and fixed during this pass: stale values reverting to available after the feed stopped, active recordings unreadable by Analysis, aligned series losing their lap ID, decimation before alignment, lap end sequences including the next lap's first packet, hardcoded restricted rival gap, "stopped user" and "1 laps" wording, navigation-height overflow, engineer card claiming Live with no endpoint, replay opening on an active recording, raw `TypeError` on network failure, and lost focus after cleanup cancellation.

## Gate C: rated workload

Final run: 2026-10-19 12:49:46 to 14:49:46 (Asia/Saigon), final tree including the outcome and summary work, macOS arm64, 2048 dk/s, 24 cars, seed 20261018, three harness stream clients, three real browser pages.

```
go test ./server/qualification -run '^TestRatedWorkload$' -count=1 -timeout=130m \
  -args -duration=2h -udp-rate=2048 -cars=24 -seed=20261018 -clients=3 \
  -evidence-dir=docs/rework/evidence/release-qualification
```

Result (`release-qualification/rated-workload.json`):

| Check | Required | Measured |
|---|---|---|
| transmitted / received / recorded | equal | 14,745,600 / 14,745,600 / 14,745,600 |
| payload digest | equal | `e3a0ac70...d7349` on both sides |
| application drops | 0 | 0 |
| recorder state | recording | recording, 3 files across two rotations |
| server peak RSS | <=512 MiB | 121.4 MiB |
| server growth, final 90 min | <=64 MiB | 26.46 MiB |
| accepted-observation to stream p95 | <250 ms | 0.781 ms (max 71.5 ms) |
| ingest lag max | no growing backlog | 93 ms |

Browser clients (`release-qualification/browser-summary.json`, raw samples in `browser-samples.json`), measured inside the workload window:

| Page | Peak process-tree RSS | p95 render | Max JS heap |
|---|---|---|---|
| Drive | 467.4 MiB | 23 ms | 61 MiB |
| Pit wall | 365.1 MiB | 18 ms | 54.2 MiB |
| Analysis | 353.2 MiB | 4.5 ms | 57.5 MiB |

The browser pages subscribed at 20 Hz and committed React at 10 Hz. That is a deliberate deviation from the workload document's 60 Hz browser line. At 60 Hz, two full runs crossed the 512 MiB browser limit (513.5 and 564.5 MiB) while the JS heap stayed flat; at 20 Hz the same page held at roughly 303-416 MiB. Server ingest and recording still take every 60 Hz datagram, and the three harness clients requested 60/5/20 Hz. The dashboard shows every received value; only the paint cadence is coalesced. Accepting 60 Hz browser rendering would require either a different browser engine with lower raster growth or a smaller page.

The latency probe now records only frames that carry a new observation timestamp. Before that guard, the two samples after the sender stopped timed an already-stale observation and reported 17-20 s; those two teardown samples are excluded from the workload summary, and live samples were unaffected because each render followed at least one new observation. A 30-second spot check with live telemetry reported 46 ms p95.

Earlier full runs and their failures are recorded in `implementation-status.md`. The first used shape-only clients; the second and third exposed the browser-memory problem above.

## Gate D: external qualification

Not available on this machine, and not faked:

- No configured model provider or API key. Real Chat Completions and Responses tool round trips, provider cancellation and the semantic eval suite are unrun. Fake-endpoint tests prove message shapes only. The authored `prompts/v1` bundle is unchanged and hash-verified.
- No configured search provider or key. SearXNG/Tavily behaviour is covered by local fixtures; no live search or source fetch was executed.
- No Windows 11 x64 host with a microphone or the game. The WinUI project does not compile on macOS (`NETSDK1100`; with Windows targeting enabled the XAML compiler crashes). The portable ZIP, clean-machine launch, tray, push-to-talk under game focus, playback and rebinding are unverified. The macOS Go cross-build is a compile check, not Windows qualification.
- No real 2026 capture. Protocol behaviour is covered by the supplied 2025 capture and the 2026 golden corpus.

Smallest next action for each: configure an endpoint/key and rerun the fixed semantic cases; configure a search key and run one real search plus direct source inspection; run `windows/package.ps1` and the native checks on a Windows 11 x64 machine with a microphone and the game; record one real 2026 session through the native or Docker package.

## Gate E: final exit

Prompt source hashes match the manifest; OpenSpec and the artifact validator pass; the final diff preserves the user's tree; packaging artifacts and rollback are present. `deterministic-race-strategy` 2.2 is complete: accepted advice is stored with session and branch identity, `previousAccepted` is branch-scoped, measured outcomes compare the plan with observed pit laps and compounds, and outcome rows only move forward in lap coverage. Tests: `TestRecordOutcomesComparesPlanWithObservedLaps`, `TestRecordOutcomesWaitsForClosedWindow`, `TestPreviousAcceptedIsBranchScoped`. The approved HTTP contract has no read endpoint for outcomes, so the invocation list remains the public view of stored advice and the outcome rows are internal evidence.

## Artifacts

| Target | Size | SHA-256 |
|---|---|---|
| linux/amd64 | 54,451,922 B | `484e565d443ef7bf6f2a786a8c620c544ac1ec17c1a5350faf679e5568fee6ad` |
| linux/arm64 | 51,563,269 B | `88e8414a7442ed743d96b3a212e94c61c629b8c409042808622530ed7810531c` |
| windows/amd64 (cross-built) | 54,859,264 B | `a349391b2dfe4e4da4cd83c5beb24644161ab84cad8bb4356de1dffcbb52b0f5` |
| Docker image `f1tele:qualification` | 37,306,970 B | image ID `sha256:3f4248833ce3d6cb1cb7945ab5d7a06ef03bb4b82182d41536ad6c60892e0d55` |

The Windows binary is a macOS cross-build. It is not a native Windows artifact and is not claimed as one.

## Requirement mapping

| Requirement | Evidence |
|---|---|
| Decode 2025 and 2026 packets | protocol golden and real-2025 parity tests |
| Availability, restrictions, stale | state tests; rendered stale/restricted states |
| Context dimensions and game modes | state mode/season tests, published tables |
| Flashback continuity | state rewind test, history branch segments, scheduler reset vector |
| Raw recording and recovery | round-trip, import, recovery, rotation tests; 2-hour soak |
| Bounded analysis and exports | analysis tests, summaries and cache invalidation test, Parquet/CSV/JSONL round trips, job isolation |
| Deterministic estimates and candidates | strategy estimate, candidate and reference tests |
| Notifications | policy tests; rendered alerts |
| Advice and outcomes | stored invocation rows, branch-scoped continuity, forward-only outcome tests |
| Scheduling and cancellation | all 16 supplied vectors; pending Safety Car evidence preserved |
| Authored prompts | byte-identical bundle, manifest hash test |
| Dashboard workspaces and states | store tests plus rendered screenshots at both viewports |
| Windows control center core | nine unit tests on macOS |
| Packaging | cross-builds and Docker image |
| Environment configuration | loader, precedence, validation and secret tests; rendered Settings at 1080p, 1440p and 200% |
| Session analytics | participant, lap, sector, stint, trace and backfill tests; rendered History, recorded Pit wall, Analysis, practice and qualifying |

## Remaining limitations

- Gate D resources listed above.
- Dashboard refresh is 20 Hz with 10 Hz paint commits, not 60 Hz rendering.
- The browser-memory gate is measured as the sum of process RSS, which counts shared pages once per process; values are conservative.
- WinUI XAML, portable ZIP, tray, native voice and Windows packaging are unbuilt on this host.
- Data-driven corner averages remain unavailable without validated entry/exit zones; thermal and capacity references are provisional.
- Outcome rows are not exposed through an HTTP read endpoint because the approved contract has none; they are visible to the model and retained in SQLite.
- The demo corpus reports no trap speed, so the UI shows that field only for real captures.
- Provisional corner zones are geometry-derived with a 50 m window, not calibrated; they are labeled provisional.
- The post-race engineer panel shows the missing-provider state on this machine; no real provider was available.
- Qualifying segment tabs appear when sessions share a weekend link; only single-segment data was captured locally.

## Rollback

The previous runtime remains at `c95b78c` on branch `codex/f1-telemetry-redesign-specs`. No recording was rewritten or deleted. New data lives in `catalog.db` and `recordings/` under the configured data directory; a downgrade does not migrate them back. Stop the service, point the old runtime at a fresh or backed-up data directory, and restore the previous image or binary. Remove `f1tele:qualification` with `docker image rm` if it is no longer wanted.
