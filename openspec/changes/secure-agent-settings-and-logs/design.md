# Design

## Goals

Add a password and Origin boundary without changing the local-first deployment model. Make the race engineer's behavior editable and observable from the web. Give the engineer durable model-visible context with bounded compaction. Keep endpoint secrets and portable Windows configuration outside the browser.

## Authentication

Add `server/internal/auth`. It owns:

- `auth.json` in the data directory: version, PBKDF2-SHA256 salt, iteration count, derived key and a 32-byte session-signing key, mode 0600.
- A generated password of 20 unambiguous characters, printed only when the file is first created.
- `F1TELE_AUTH_PASSWORD` precedence. When set, the value authenticates and is hashed into a missing auth file; reset refuses while it is set.
- Signed session cookies: base64url payload containing issue time, expiry and a random nonce, plus HMAC-SHA256. Cookie name `f1tele_session`, 30-day MaxAge, HttpOnly, SameSite=Strict, Path=/. Secure is not set because the server speaks HTTP; TLS is a reverse-proxy concern.
- A per-source failure counter, five failures per five minutes, reset on success. Failures use a generic 401; throttled attempts use 429.

The HTTP handler wraps the existing mux. Public paths: `/api/v1/health`, `/api/v1/auth/login`, `/api/v1/auth/status`, and static assets. Everything else under `/api/v1/` requires a valid cookie. The WebSocket and SSE handlers check the cookie before upgrade or streaming. Login/logout/status live in `httpapi`.

Origin validation runs before auth. If an Origin header exists, parse it and compare its host to `r.Host`; allow only http/https schemes. Missing Origin is allowed. A mismatch returns 403. `security.origin_check` defaults true and is env/WinUI-owned; web Settings reports it.

CLI commands live in `server/cmd/f1tele`:

- `auth ensure --json [--data-dir path] [--config path]` returns `{created, source, password?}`. The password field exists only when created.
- `auth reset [--stdin] [--data-dir path] [--config path]` reads a new password, replaces the hash, rotates the signing key and never prints the password.

Data-directory resolution is factored out of `config.Load` so auth commands do not require a valid model configuration.

## Activity Log

Add SQLite tables in the existing catalog:

- `agent_events(seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT UNIQUE, invocation_id TEXT, session_id TEXT, branch_id TEXT, profile TEXT, kind TEXT, at_ms INTEGER, duration_ms INTEGER, payload_json TEXT, payload_sha256 TEXT, truncated INTEGER DEFAULT 0)`.
- `agent_compactions(id TEXT PRIMARY KEY, conversation_id TEXT, invocation_id TEXT, trigger TEXT, focus TEXT, from_seq INTEGER, to_seq INTEGER, before_tokens INTEGER, after_tokens INTEGER, summary_json TEXT, model TEXT, usage_json TEXT, created_at_ms INTEGER)`.
- `agent_messages(id TEXT PRIMARY KEY, conversation_id TEXT, role TEXT, content_json TEXT, tool_call_id TEXT, invocation_id TEXT, compacted_into TEXT, created_at_ms INTEGER)`.
- `prompt_revisions(id TEXT PRIMARY KEY, profile TEXT, body TEXT, sha256 TEXT, created_at_ms INTEGER)`.
- `prompt_current(profile TEXT PRIMARY KEY, revision_id TEXT, updated_at_ms INTEGER)`. A null revision means the authored default.

Add invocation columns for `conversation_id`, `endpoint_id`, `model`, `prompt_revision_id`, `prompt_sha256`, `latency_ms` and `compaction_count`. Migrations are additive and run through the existing migration runner.

An `Activity` recorder writes events before broadcasting. Payloads are redacted and capped at 1 MiB; over-cap payloads store a truncation object with byte count and SHA-256. Provider continuation is never included in event payloads. At startup, stale `running` invocations receive an `interrupted` event.

The API adds cursor-paginated `GET /api/v1/engineer/logs` and `GET /api/v1/engineer/logs/{id}`. The detail response joins the invocation, events, compaction rows and recorded prompt snapshot. SSE is `GET /api/v1/engineer/activity/stream`; it supports `session`, `branch` and `Last-Event-ID`, sends a heartbeat every 15 seconds, and reads persisted events first.

## Transcript and Compaction

Conversation IDs are deterministic:

- live strategy: `conv:<session>:<branch>:live`
- post-race: `conv:<session>:post`
- setup research: `conv:<session>:setup`

The context builder assembles the system prompt, latest compaction summary, a recent verbatim tail and the current `AgentInput`. The model-visible request snapshot is stored on the invocation and as a `context_built` event.

Token accounting uses the provider's `PromptTokens` from the last successful response when available. New content is estimated at UTF-8 bytes/4 plus fixed message overhead. The effective limit is `min(endpoint.context_limit_tokens, agent.context_tokens)`.

Compaction config: enabled by default, trigger 80%, recent tail 25%. A fixed internal summarizer prompt requests a structured `CompactionSummary`. The summary preserves plan, decision timeline, accepted advice, evidence IDs, assumptions, predictions, outcomes, event transitions, coverage gaps and focus instructions. Raw sample/tool payloads are omitted first; the recent tail is preserved.

Overflow recovery targets 70%, 50% and 35% of the rejected request, at most two compactions per logical turn. Compaction uses the same endpoint/model, counts against budgets, creates a `compaction` activity timeline and clears provider continuation. A manual compaction endpoint accepts `{session_id, branch_id, focus?}` and runs even below the threshold.

## Prompt Revisions

The authored default is assembled from the embedded bundle. A custom revision stores the full effective system prompt. The application still appends the output-schema instruction and tools. `Assemble` becomes `Assemble(profile, store)`; it returns text, revision ID and hash.

HTTP routes:

- `GET /api/v1/agent/prompts` returns profile metadata, current revision and revision list.
- `GET /api/v1/agent/prompts/{profile}` returns authored text, current text and revisions.
- `POST /api/v1/agent/prompts/{profile}/revisions` creates and selects a revision.
- `POST /api/v1/agent/prompts/{profile}/revisions/{id}/select` selects an existing revision.
- `DELETE /api/v1/agent/prompts/{profile}/revisions/{id}` deletes a non-current revision.
- `POST /api/v1/agent/prompts/{profile}/reset` selects the authored default.

Revision bodies are limited to 128 KiB. Profiles are the five known engineer profiles. Deleting a revision does not remove invocation snapshots or hashes.

## Web UI

Add a login gate around the root app. The `api` helper throws a typed auth error on 401; the gate returns to login. The top navigation adds Logs and a logout action.

Logs is a dense table plus a detail panel with Overview, Input, Output, Tools, Context and Raw tabs. Live mode uses EventSource and follows new events until the user scrolls away. The Pit wall adds an Agent activity button with a running badge and a right-side drawer. History rows and recorded Pit wall banners link to Logs with session and branch query parameters.

Settings is split into Agent, Prompt, Compaction, Connections and Security. Existing server and recording sections remain read-only. Editable agent fields use the full config PATCH with If-Match. The advanced rules editor is Monaco JSON. Prompt editing uses Monaco DiffEditor and a revision side panel. The current revision's delete control is disabled; deletion uses an inline confirm next to the control.

The frontend adds `@monaco-editor/react` and `monaco-editor`, imported only on the Settings route.

## Windows

`SettingsStore` gains a first-run flag. `App.OnLaunched` shows a Settings window when first run is incomplete; otherwise it shows MainWindow. Settings calls `f1tele.exe auth ensure --json` to obtain the one-time generated password, displays it in a ContentDialog with a code box and copy button, and never displays it again.

Saving first-run Settings writes `config.json` and `secrets.json`, marks first run complete, then starts the server. Saving later settings stops the owned server, writes the files and restarts. Environment-owned values are read-only. Reset opens new-password and confirmation fields and pipes the value to `f1tele.exe auth reset --stdin`.

The native Settings page owns connection, recording, search, voice and security fields. Agent/harness settings and activity logs link to the web dashboard.

## Compatibility and Verification

Config changes are additive; `schema_version` remains 1.0.0. Existing recordings, transcripts and strategy rows are untouched. Legacy invocations show available output and an input-not-captured state. There is no automatic activity-log retention in this change.

Verification uses Go unit and HTTP tests, SQLite migration tests, fake-provider compaction tests, frontend Vitest tests, production build and lint, and a native Windows qualification run. Real provider and real game behavior remain separate qualification evidence.
