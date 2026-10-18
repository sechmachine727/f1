# Finite implementation and qualification gates

These gates describe future application acceptance. This document does not claim they have passed. Evidence records command, commit, OS/hardware, input fixture/hash, configuration, start/end, exit status and raw artifact paths. Keep secrets/audio content out of published logs.

## Gate A: implementation checks

Each change's tasks.md identifies owned files and focused tests. First create a failing requirement-level test for nontrivial behavior, implement it, then run the named narrow suite. Go packages live under server/ in root module f1tele. The implementing agent must provide the named test commands rather than merely writing a report.

Required aggregate commands:

~~~sh
rtk go test -race ./server/...
rtk go vet ./server/...
rtk go build ./server/cmd/f1tele
rtk openspec validate --all --strict --no-interactive
rtk proxy python3 docs/rework/validate_artifacts.py
~~~

In race_engineer_hub/:

~~~sh
rtk npm run test -- --run
rtk proxy npx tsc --noEmit
rtk npm run lint
rtk npm run build
~~~

A proven unchanged baseline failure must include the same failing command on the baseline commit and a scoped explanation. It remains an explicit exception in the final report, not a passed test. A new failure blocks completion. The artifact validator introduced in this planning delivery verifies documents/schemas/examples only; it does not verify application implementation.

Go cross-builds: CGO_ENABLED=0 GOOS=windows GOARCH=amd64, and linux/amd64 plus linux/arm64, each running go build ./server/cmd/f1tele with isolated output filenames. Windows: dotnet test windows/F1Telemetry.ControlCenter.Tests and dotnet publish windows/F1Telemetry.ControlCenter -c Release -r win-x64 --self-contained true on Windows. Docker: build nativeamd64/arm64, run health/UDP/record/restart smoke against each target. Do not use a Mac build as Windows native qualification.

## Gate B: behavior and UI

Execute protocol golden/malformed/restriction tests, same-frame events, sequence wrap, flashback surviving-prefix/alternate attempts, crash/recovery/import, cap/free-reserve/queue failures, export round trips and replay/live isolation. Execute every supplied scheduler vector and output-acceptance rule. Record reference-profile applicability and unsupported core/corner/capacity cases.

Render the real application at1920x1080 and2560x1440 with all four semantic states, unknown/restricted values, long text and a realistic24-car field. Drive has no scroll at100%scale. Inspect200%zoom/reflow, keyboard/focus, contrast, reduced motion, empty/loading/error/stale/paused/recording-stopped/AI-disabled states. Run real pointer and keyboard History->Analysis->Export and selected-rival workflows. Save before/after screenshots and bounded findings; fix material issues, then stop polishing.

## Gate C: rated workload

Implement the generator and oracle as server/qualification/rated_test.go and helper files. Run a30-second smoke before the2-hour soak. Invocation contract:

~~~sh
rtk go test ./server/qualification -run '^TestRatedWorkload$' -count=1 -timeout=130m -args -duration=2h -udp-rate=2048 -cars=24 -seed=20261018 -clients=3 -evidence-dir=docs/rework/evidence/release-qualification
~~~

The test flags and package are future deliverables of release-qualification, not preexisting commands. Browser clients must render the built real UI: Drive selected telemetry60Hz; Pit wall standings5Hz; Analysis periodic indexed range/compare requests. Use the browser automation method supported by the environment. The fixed [workload contract](contracts/workload.json) defines per-ID sizes/rates, duplicate/reorder/event mix, phase schedule and exact expected traffic. It emits2048datagrams/s and2141822payload bytes/s for7200seconds. IDs8/9/14 are covered separately outside healthy counts. Use the independent sender-count/receiver-intake/MCAP oracle described there; never redefine expected traffic as admitted recorder traffic. Duplicates remain raw and source/OS loss is distinct from application loss.

Phases and timestamps are fixed in workload.json: warmup, reorder, flashback/nested rewind, slow-client/reconnect, cancelled Safety Car invocation and analysis/export load. Same-frame distinct events remain a required protocol fixture. Run deliberate storage-stop/fault recovery as a separate fixture test, not as a hidden excuse for missing data in the healthy full-rate segment. Keep expected intentional faults separate in evidence.

Pass limits:

- Healthy segment: zero application-induced raw-recording drops, exact count/digest match for successfully received datagrams, and source/OS loss counters measured or explicitly unavailable.
- Server resident memory <=512MiB after warmup; growth <=64MiB over the final90minutes. Each browser <=512MiB, with bounded state/log retention.
- p95 accepted-observation-to-local-render <250ms. Every critical event <=500ms. Include local queue/render delay; exclude external provider/network latency only where recorded.
- No process crash, growing backlog, stalled recording, leaked file handles or cross-source replay mutation.

Use local clock-offset handshake plus observation/event IDs to measure renderer latency; save raw samples and percentile computation. Record machine/OS/browser versions. If hardware cannot meet the fixed gate, report the failing measurement; do not increase thresholds silently. Save summaries/logs, not multi-gigabyte raw captures in Git. Keep test captures in a declared external evidence directory with their hashes and user-controlled cleanup.

## Gate D: external qualification

- New real2026 recording: packet16, formula/context, flashback and race-finish behavior; rare v1.2 events covered synthetically and labeled if not seen live.
- Real configured endpoint for Chat Completions and Responses: tool call -> correlated result -> valid final JSON, cancellation, optional rationale absence and provider-scoped continuation. Three fixed runs of critical semantic cases per declared endpoint/model/settings; no automatic prompt tuning.
- Real search provider and direct inspected setup source; unsupported SearXNG JSON and source failure paths verified.
- Windows11 x64 clean-machine portable launch without Docker/.NET installation, tray/stop/flush/recovery and actual game-focus microphone/PTT/playback/rebinding.

Thermal/capacity/corner profiles can ship provisional/unsupported as specified; they cannot be described as calibrated without measured evidence. Lack of a sourced numeric optimum is handled by neutral classification, not a fabricated pass. Missing credentials, game access, capture, Windows host or microphone makes the affected gate externally blocked. Finish independent code/checks, record the missing resource and exact next action, then use the bounded stop rules in HANDOFF.md.

## Gate E: final exit

All nine changes' implementation tasks are checked only when their evidence exists. final-report.md maps requirements/scenarios to commands/artifacts and records exceptions. Prompt source hashes match manifest; OpenSpec/static validators pass; final diff preserves user work and contains no unresolved material finding. Package artifacts and rollback instructions are present. No publishing/merging/archiving occurs without the user's instruction.

Stop successfully only after A–E required checks pass. Blocked status is permitted only for the named unavailable provider/key, Windows/game/microphone host, real2026 capture or required human decision, after independent work is complete. A failing test, memory/latency gate or soak on available hardware is an implementation failure: reproduce, fix and retest it. If a budget ends first, report incomplete and leave a checkpoint; never relabel it externally blocked. Do not substitute fake external evidence, repeat unchanged passing checks, expand scope or keep polishing. If a goal API requires three consecutive blocked turns, record one unchanged blocker audit per continuation and mark blocked at the third, with no repeated expensive work.
