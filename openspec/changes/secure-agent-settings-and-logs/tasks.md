# Implementation Tasks

## 1. Contracts and storage

- [x] 1.1 Add `SecurityConfig`, `Agent.Compaction` and `Agent.Generation` to `server/internal/contract/config.go`; update `defaults.json`, `docs/rework/contracts/v1.schema.json` and examples. Verify `go test ./server/internal/config` and the schema validator.
- [x] 1.2 Add migration `006_agent_activity.sql` for `agent_events`, `agent_compactions`, `agent_messages`, `prompt_revisions` and `prompt_current`; add invocation metadata columns. Verify catalog migration and reopen against an existing database.
- [x] 1.3 Add history store methods for activity events, compaction summaries, transcript messages and prompt revisions. Verify with SQLite tests covering ordering, pagination, deletion protection and legacy rows.

## 2. Authentication

- [x] 2.1 Implement `server/internal/auth`: auth-file load/create, PBKDF2-SHA256 hash, password generation, constant-time verify, session cookie sign/verify, reset with signing-key rotation, and environment precedence. Verify unit tests for expiry, tampering, wrong password and reset invalidation.
- [x] 2.2 Factor data-directory resolution out of `config.Load` and add `f1tele auth ensure --json` and `f1tele auth reset [--stdin]`. Verify interactive and piped command tests plus a first-start Docker-style log capture.
- [x] 2.3 Add Origin validation and auth middleware around the HTTP mux; exempt health, login, status and static assets. Apply the same checks before WebSocket and SSE writes. Verify cross-origin, same-origin, missing-Origin and unauthenticated cases.
- [x] 2.4 Add login, logout and status routes with per-source failure throttling and generic errors. Verify cookie attributes, 401/429 behavior and no password leakage.

## 3. Activity log

- [x] 3.1 Implement the activity recorder with redaction, payload cap, hash, write-before-broadcast and stale-running reconciliation. Verify event ordering, truncation markers and restart behavior.
- [x] 3.2 Add cursor-paginated `GET /api/v1/engineer/logs` and `GET /api/v1/engineer/logs/{id}` with session, branch, profile, status, trigger, model, error, compaction and time filters. Verify pagination and legacy detail behavior.
- [x] 3.3 Add authenticated SSE `GET /api/v1/engineer/activity/stream` with `Last-Event-ID` replay and heartbeats. Verify reconnect replay, session filtering and pre-upgrade rejection.
- [x] 3.4 Instrument the engineer invoker and manager with queued, context, compaction, provider, tool, validation, acceptance, notification, cancellation, error and interruption events. Verify a fake-provider timeline from start to finish.

## 4. Transcript and compaction

- [x] 4.1 Add deterministic conversation IDs and transcript persistence for live strategy, post-race and setup research. Verify scope isolation and tool-result persistence.
- [x] 4.2 Build context from the system prompt, latest summary, recent tail and current input; store the exact request snapshot. Verify prompt snapshot, token estimate and legacy empty history.
- [x] 4.3 Add token accounting from provider usage plus deterministic estimation. Verify threshold decisions against the effective context limit.
- [x] 4.4 Add the internal summarizer prompt and structured `CompactionSummary`; preserve plans, decisions, evidence, assumptions, outcomes, events and coverage. Verify old telemetry drops first and accepted strategy rows remain unchanged.
- [x] 4.5 Add threshold, provider-overflow, 70/50/35 recovery, two-compaction cap, manual focus and budget accounting. Verify each path with a fake provider and a no-summary failure.
- [ ] 4.6 Clear provider continuation across compaction and record the reset. Verify Chat and Responses transcript replay behavior where the provider fixture supports it.
- [x] 4.7 Add `GET /api/v1/engineer/context` and `POST /api/v1/engineer/compact`. Verify usage response, manual focus and disabled-history states.

## 5. Prompt revisions

- [x] 5.1 Add authored prompt lookup, custom revision storage and `Assemble(profile, store)` returning text, revision and hash. Verify custom text receives the schema instruction and the correct tool set.
- [x] 5.2 Add prompt list/detail/create/select/delete/reset routes with profile validation, 128 KiB limit and current-revision deletion refusal. Verify all routes and audit snapshots after revision deletion.
- [x] 5.3 Record prompt revision, hash and full snapshot with each invocation. Verify a custom-prompt invocation and a legacy authored-default invocation.

## 6. Web authentication and Logs

- [x] 6.1 Add login screen, auth status bootstrap, 401 handling, logout action and top-level Logs navigation. Verify unauthenticated loads stay on login and successful login loads the selected workspace.
- [x] 6.2 Build the Logs table, filters, cursor pagination, detail tabs, copy/download and empty/error/loading states. Verify recorded sessions and legacy rows.
- [x] 6.3 Add the live SSE activity hook, follow/pause behavior and running-count badge. Verify reconnect and auto-follow at desktop and narrow widths.
- [x] 6.4 Add the Pit wall activity drawer and History/recorded-Pit-wall links using session and branch query parameters. Replace EngineerCard's invocation list with the activity link while keeping the question form.

## 7. Web agent settings

- [x] 7.1 Split Settings into Agent, Prompt, Compaction, Connections and Security sections; keep server and recording read-only and mark environment-owned values.
- [x] 7.2 Implement agent form controls for scheduling, continuous mode, budgets, deadlines, context/output, steps, tools and sampling with server validation and If-Match conflict handling. Verify each editable field and disabled environment field.
- [x] 7.3 Add the advanced rules JSON editor and prompt revision editor with Monaco, side-by-side diff, revision selection, inline delete confirmation and reset. Verify save/diff/select/delete/reset and current-revision protection.
- [x] 7.4 Add compaction controls, usage display and Compact now focus input. Verify manual compaction appears in activity and the threshold/status fields persist.

## 8. Windows control center

- [ ] 8.1 Add first-run state and a native Settings window shown before server startup; route later Settings access through the main window. Verify fresh and returning launches with WinUI tests.
- [ ] 8.2 Call `auth ensure --json` on first run and show the generated password once in a code box with copy. Verify no second display and environment-owned behavior.
- [ ] 8.3 Implement native fields for Origin validation, listeners, data directory, recording, agent endpoint/secrets, search and voice, with environment locking and save/restart. Verify config/secrets writes and restart behavior.
- [ ] 8.4 Add new-password/confirmation reset through `auth reset --stdin` and links to web agent settings and logs. Verify existing sessions fail after reset.

## 9. Verification and documentation

- [x] 9.1 Run `go test -race ./server/...`, frontend `npm test`, `npm run lint`, `npm run build`, `npx tsc --noEmit` and `openspec validate --all --strict`. Record exact results and baseline failures.
- [x] 9.2 Add operator docs for login, Docker password output/reset, native first-run/reset, Origins, Logs, prompt revisions, compaction and environment variables.
- [ ] 9.3 Exercise the authenticated dashboard, Logs, Pit wall drawer and Settings at 1920 and 2560 widths; save screenshots and console evidence.
- [ ] 9.4 Run the native Windows app on a real Windows host for first-run, popup-once, save/restart and reset; record the external qualification result.

## 10. Session transition repair

- [x] 10.1 Reset pause, samples, frame ordering, participants, laps, events and season state when the session UID changes.
- [x] 10.2 Treat a same-UID return to the session-clock origin as a new logical session.
- [x] 10.3 Add reducer tests and an end-to-end WebSocket test for Q1-to-Q2 and restart transitions.
- [x] 10.4 Set `F1TELE_DATA_DIR=/data` in the container image and Compose so password reset uses the same data directory as the server.

## 11. Tyre corner telemetry

- [x] 11.1 Add a corner-aware packet read and use it for every wheel-array channel in the reducer.
- [x] 11.2 Add unit and golden-packet tests proving four corner values remain distinct.
- [x] 11.3 Use a continuous green-to-red wear color in the Drive ring and tyre-wear graph.
- [x] 11.4 Base live freezing on player speed and the pause flag so auxiliary stale channels do not stop wear color updates.

## Qualification notes

- Windows Core builds and its 15 tests pass on macOS. The WinUI project cannot compile its XAML here because the Windows SDK XamlCompiler.exe is a Windows executable. Tasks 8.1-8.4 remain open until a Windows host build and launch pass.
- Provider continuation replay remains open in 4.6; compaction records a continuation reset marker, but the existing Responses adapter does not persist and replay opaque provider items.
- The contract validator passes 79 definitions, 74 fixtures and 19 semantic reference cases. Its artifact scan still reports the pre-existing em dash in docs/rework/evidence/telemetry-traces.md.
