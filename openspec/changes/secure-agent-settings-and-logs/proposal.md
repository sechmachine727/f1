# Proposal

## Why

The dashboard currently has no authentication or Origin validation, the web Settings workspace is read-only, and the race engineer keeps no durable model-visible history. Operators can neither tune agent behavior safely nor audit what the agent saw, decided or compacted during a race. This change makes the local web surface accountable and gives the engineer enough context continuity to behave like a coding-agent harness.

## What Changes

- Add password authentication to every dashboard workspace, API route, SSE stream and WebSocket except the public health probe and login/status routes. Use signed 30-day sessions, PBKDF2-SHA256 storage, per-IP login throttling and logout.
- Enable same-host Origin validation by default, disableable through `F1TELE_SECURITY_ORIGIN_CHECK` or native Windows Settings.
- Add first-run password generation, one-time console output for Docker, one-time native password display for Windows, and password reset through `f1tele auth reset` without the previous password.
- Add an append-only agent activity log with invocation list/detail views, filters, full model-visible input/output, tool timelines, usage, latency, errors and compaction events.
- Stream live agent stages over an authenticated SSE endpoint. Add a live Pit wall activity drawer and recorded-session access from History.
- Add a top-level Logs workspace with the LiteLLM-style request table and detail tabs. Prompt/response payloads are stored by default because the local surface is password-protected.
- Make web Settings the editing surface for agent scheduling, budgets, context limits, sampling, compaction and prompt revisions. Endpoint connection fields remain read-only in web Settings.
- Add Monaco-based prompt editing with side-by-side save diffs, immutable revisions, inline deletion confirmation, and reset to the authored prompt.
- Add a durable live-strategy transcript per session and branch, plus separate post-race and setup transcripts. Compact at a configured context percentage using a structured summary while preserving plans, evidence, assumptions, outcomes and accepted strategy.
- Add a native Windows Settings window shown before server startup on first run, with password management, Origin validation, listeners, recording, endpoint and secret configuration. Agent/harness settings and logs remain in the web dashboard.
- Supersede the earlier no-login, no-Origin requirement and the web Settings read-only requirement. Existing historical invocations that predate activity capture remain visible with a clear "input not captured" state.

## Capabilities

### New Capabilities

- `web-authentication`: password lifecycle, signed sessions, Origin validation, public health and login routes, and native/Docker password operations.
- `agent-activity`: append-only invocation events, live SSE activity, Logs workspace, detail views and session-scoped access from Pit wall and History.
- `agent-configuration`: web editing of agent scheduling, budgets, context, sampling and revisioned prompts.
- `agent-context`: durable model-visible transcripts, token accounting, manual and automatic compaction and overflow recovery.
- `windows-settings`: first-run native configuration, password display/reset, Origin toggle and portable-package connection settings.
- `session-continuity`: clean reducer and stream state when the game changes qualifying segments or restarts a session.

### Modified Capabilities

None in the canonical `openspec/specs` store, which has no archived capabilities. The new capabilities explicitly supersede the related requirements in the in-flight `go-telemetry-core`, `deployment-configuration`, `scheduled-race-engineer` and `windows-control-center` changes.

## Impact

- `server/internal/config`: security, compaction and generation configuration; auth-file resolution.
- `server/internal/httpapi`: authentication middleware, Origin checks, login/logout/status, Logs/SSE, context/compaction and prompt-revision routes.
- `server/internal/engineer`: transcript assembly, activity recording, compaction and prompt-revision lookup.
- `server/internal/history`: catalog migrations for agent events, compactions, transcript messages and prompt revisions.
- `server/cmd/f1tele`: `auth ensure` and `auth reset` commands.
- `race_engineer_hub`: login gate, Logs workspace, live activity drawer, History links, editable Settings and Monaco prompt revisions.
- `windows/F1Telemetry.ControlCenter`: first-run Settings window, native configuration, password reset and restart flow.
- `docs/rework` and `docs/settings.md`: schema, examples and operator documentation.

Existing recordings and accepted strategy data are not rewritten. Activity payloads omit secrets, authorization headers, opaque provider continuation and hidden reasoning.
