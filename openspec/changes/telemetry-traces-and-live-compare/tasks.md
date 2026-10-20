# Tasks

## 1. Sample storage and read route

- [x] 1.1 Add migration 005 with `lap_samples` and `session_participants.your_telemetry`, plus catalog upsert and batched read methods; verify a round trip and rerun through `go test ./server/internal/history`.
- [x] 1.2 Extend the tracker sample to session time, throttle, brake, gear, ERS energy, deploy mode and overtake flags, sampling on packet 6 with packet 7 and 16 enrichment; verify a synthetic 2025 lap and a 2026 lap in `go test ./server/internal/analytics`.
- [x] 1.3 Store restricted non-player energy and deployment as null while keeping throttle, brake and gear; verify a restricted participant case and a public player case in the same test.
- [x] 1.4 Implement the 900-sample decimation with first, last, gear-change and brake-state protection; verify the bound, both ends and a shift-heavy lap in a table test.
- [x] 1.5 Extend backfill to select sessions without samples, including sessions with an existing summary; verify a summarized legacy session gains samples, a second run changes nothing and the recording digest is unchanged.
- [x] 1.6 Add `GET /api/v1/sessions/{id}/telemetry` with car and lap bounds, 404 for unknown sessions and an empty coverage response; verify two-car payload, over-limit rejection and missing-session cases in `go test ./server/internal/httpapi`.
- [x] 1.7 Make the deployment-mode label season-aware (Overtake for 2025, Boost for 2026, unclassified for unknown) for the stream and export paths; verify both seasons and the unknown case in the existing protocol and export tests.

## 2. Live Pit wall comparison

- [x] 2.1 Add the team identity map and readable ink helper with neutral fallback; verify known teams, unknown identifiers and 3:1 contrast in `src/lib/teams.test.ts`.
- [x] 2.2 Replace the single detail car with a two-car selection that replaces the earliest on a third pick; verify selection, replacement and table state with Testing Library.
- [x] 2.3 Build the comparison grid for position, gaps, speed, throttle and brake, gear, ERS, fuel, tyre wear and damage with deltas and unavailable labels; verify matched values show a direction and a restricted value suppresses its delta.
- [x] 2.4 Add the selected-car stream subscription and current-lap ring buffer; verify trimming at the lap boundary and that the main 24-car subscription is unchanged in a unit test.
- [x] 2.5 Compose the live detail panel with the trace stack, pace and pit panels, keeping team identity colours separate from status colours; verify the populated, restricted, stale and no-car states with Testing Library and a browser pass.

## 3. Shared time-axis charts

- [x] 3.1 Add chart window helpers for zoom about a point, pan, clamp, reset and follow detection; verify each case including a zero-length range in `src/lib/chartWindow.test.ts`.
- [x] 3.2 Implement window-aware downsampling that keeps bucket extremes and returns stored points at fine zoom; verify point bounds and that a one-sample brake spike survives the full-race reduction.
- [x] 3.3 Build `TraceStack` with throttle and brake, gear, ERS plus the deployment band, shared crosshair, navigator strip, zoom buttons, drag pan and keyboard controls; verify band segments for 2025 modes and 2026 overtake states in component tests.
- [x] 3.4 Wire follow-latest so panning away freezes the window and the control resumes at the newest sample; verify the freeze and resume behaviour without a live socket using a fake sample source.
- [x] 3.5 Replace the plain chart containers in the recorded Pit wall, Analysis and tyre wear history with the shared window controls; verify `npm run lint`, `npx tsc -p tsconfig.app.json --noEmit` and `npm test` stay clean.

## 4. Recorded workspaces and History

- [x] 4.1 Add the recorded Pit wall telemetry view with lap range, car selection, stored-sample fetch and pending-backfill states; verify two cars and a lap range render from the route response in a component test.
- [x] 4.2 Add time-aligned traces to Analysis for the picked laps while keeping the distance comparison; verify different lap durations end at their own times and nothing is stretched.
- [x] 4.3 Add circuit name, date, duration and type to History rows with the unknown-track fallback; verify mapped and unmapped identifiers in a component test and keep the recorded handoff.
- [x] 4.4 Confirm recorded views never read the live store by asserting the recorded components take their samples only from the telemetry route; verify with a test that mounts a recorded view beside a live snapshot.

## 5. Integration and qualification

- [x] 5.1 Run `go test -race ./server/...`, `go vet ./server/...`, `go build ./server/cmd/f1tele`, `npm test`, `npm run lint`, `npx tsc -p tsconfig.app.json --noEmit` and `npm run build`; record the exact results in the change evidence.
- [x] 5.2 Rerun the five-minute rated analytics load and the two-hour soak with sample capture enabled; verify exact datagram counts, equal digests, zero drops and a server memory peak inside the documented budget.
- [x] 5.3 Verify the live and recorded views at 1920x1080 and 2560x1440 with two drivers, full-race zoom, keyboard-only zoom and pan, a restricted rival and a session with no samples; save screenshots and note any limitation.
- [x] 5.4 Document the sample tuple, route limits and band rules in `docs/rework/evidence/telemetry-traces.md` and add the route to the contract table; verify the documented commands and field names against the running server.
- [x] 5.5 Re-read the final diff against the specs and record material findings, if any, in the change evidence; resolve them before reporting completion.

## Workflow follow-up

- Archive the change only after the project's review and qualification gates are satisfied.
- Sync the capability deltas into the canonical specs at archive time.

