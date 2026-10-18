# F1 telemetry redesign

## Outcome and boundaries

Build a driver-first telemetry and race-engineering package for career/solo racing and Time Trial, with durable all-car history, detailed analysis, and a secondary pit-wall workspace. Support other game modes without claiming unavailable opponent channels. Telemetry, recording, replay, and deterministic alerts work without AI.

Approved choices: Go server; existing React 18/Vite/Tailwind/Radix/shadcn/Recharts frontend; SQLite catalog; compressed indexed MCAP datagrams; Eino ADK; Chat Completions and Responses endpoints; WinUI 3/C# on Windows 11 x64. Use one server executable, not independently deployed ingestion, database, or agent services. Setup recommendations do not operate the game.

## Baseline evidence

Observed at baseline commit 1e099b0: Python composes state, timing, alerts and dispatch in telemetry_server/telemetry_server.py. State and session histories are in memory. common/f1_capture stores little-endian timestamp/length/datagram records. Seven frontend hooks open their own WebSockets. Packet 11 histories are accumulated for multiple cars; packet 12 state is filtered to the player. Neuro SAN was removed; dispatch code is a seam, not a functioning runtime. See sources.md for exact evidence.

The current capture policy samples high-rate packets at 10 Hz; the redesign records incoming datagrams without that sampling. UID-only session resets already preserve alerts through flashbacks. Preserve that behavior while adding timeline branches; do not restore session-time-based clearing from older comments.

## Architecture and ownership

Use a root Go module with application code in server/: cmd/f1tele, internal/protocol, internal/state, internal/history, internal/analysis, internal/strategy, internal/engineer, internal/setup, internal/httpapi, and internal/config. The frontend remains race_engineer_hub/. Windows sources live in windows/F1Telemetry.ControlCenter/. Modules are ordinary Go packages in one process. Avoid generic agent/plugin factories and a distributed event bus.

The UDP reader timestamps and enqueues each datagram for recording before decoding/live filtering. A single state owner applies validated packets and emits immutable observations. History writes are serialized independently so charts or providers cannot block ingestion. HTTP handlers query bounded snapshots/ranges. One socket per browser carries subscriptions; hooks read selectors from one external store.

The server embeds the frontend build at packaging time. It listens on 0.0.0.0:8081 and UDP 0.0.0.0:20777 by default. Docker publishes those ports and a persistent data directory. Windows runs the same native server directly. A browser connects directly by local/LAN address. There are no accounts, login, pairing, application tokens, origin checks/allowlists, or mandatory TLS. Do not introduce them during implementation. Provider/search/voice secrets are write-only configuration inputs and never appear in read APIs, recordings, exports, model tools, or diagnostics.

## Protocol, identity, and availability

Supplied 2026 v1.2 PDF takes precedence over v1.1 on changed events; structures and differences must be checked against all supplied artifacts. Keep golden packet-size tests and 2025 real-capture parity. Decode full official payloads, not only dashboard fields. Session History and Tyre Sets cycle car indices; Motion Ex is player-specific. Restricted multiplayer values and withheld rival setups are unavailable, not valid zero measurements.

Maintain separate local session_id, capture fragment_id, game session UID, season/weekend/session link IDs, branch_id, and observation sequence. Link IDs persist across game saves but are not career save files. Match nonzero identifiers only within the same recorded source context; ambiguous career links remain unassigned and editable. Never merge by driver display name or date alone.

UDP format and gameYear describe transport/software, not necessarily car generation. Explicit supported formula/regulation context determines 2025 or 2026; conflicts produce unknown with evidence retained. Unknown sessions appear in Unassigned rather than being forced into a season tab. Record game mode and online status independently. Use nullable measurement wrappers as specified in contracts.md.

## Recording and branches

MCAP stores unchanged received bytes, including datagrams later rejected from live state. Its metadata records decoder version, source identity, receive time, packet identity when decodable, and validation outcome. Use Zstandard chunks, CRCs, message/chunk indexes, 1 MiB or 1-second chunk flush, 256 MiB file rotation, and SQLite recovery checkpoints. Preserve a damaged source file; recover valid chunks into a new file and report the lost tail.

Recordings start automatically on incoming telemetry when enabled (default enabled). Pre-session datagrams remain in an unassigned fragment and are associated when identity arrives. MCAP recording and SQLite summaries use a resumable checkpoint: MCAP durable chunk first, then catalog checkpoint; rebuild missing catalog data from durable chunks after a crash. Catalog changes use transactions and foreign keys. No second full-rate normalized recording.

Storage defaults are in defaults.json: 100 GiB cap, 5 GiB free reserve, warning at 80%, manual cleanup only. Count active/closed raw files and imports against the cap. Export/temp work must respect the free reserve and report refusal before starting if insufficient. A bounded 64 MiB recording queue isolates disk stalls. Queue exhaustion, disk failure, or cap exhaustion visibly stops recording and logs the uncovered interval. Live telemetry continues. Do not silently sample or delete history. Recovery/import jobs cannot overwrite originals.

A flashback event supplies its anchor; a confirmed time rewind with increasing overall frame ID is a fallback when the event is absent. Duplicate/reordered datagrams alone never create a branch. Parent data strictly before the anchor is inherited; child data at/after the anchor replaces it in the normal timeline. Keep abandoned attempts for comparison. Split a partial lap at a rewind; exclude incomplete segments from clean-lap metrics. Rebuild derived strategy/alert conditions from the surviving prefix without re-announcing unchanged conditions; retain the audit trail with branch identity. New session UID starts a new fragment; persisted link IDs can join the logical session after save/resume without erasing fragments.

## Analysis and strategy

Use canonical SI-based internal quantities with documented display conversions. Store timing as integer milliseconds and raw energy in joules. Never divide by the 2026 per-lap harvest allowance to produce battery SOC. A percentage requires a sourced applicable capacity profile; otherwise show absolute energy. Preserve 2025 Overtake and 2026 Boost/overtake eligibility/active-aero fields as separate states.

Analysis filters invalid/incomplete/pit/neutralized laps and flags uncertain comparisons. Pace is median clean-lap pace over the last five eligible laps; degradation uses within-stint lap-age regression only with at least three eligible laps and no compound mixing. Fuel finish margin uses the game's reported fuel-in-laps where valid, with a separately labeled consumption estimate requiring three clean laps. Project energy draw from a 10-second valid window; characterize deployment separately from a finish/target shortfall. Pit loss is estimated from observed pit passes or an editable, sourced track profile, never invented as an observed value.

Generate deterministic stay-out and legal pit alternatives with pit loss, compound inventory, weather and rejoin traffic. Evaluate remaining race time by summing predicted lap times plus pit losses; a linear compound/stint-age model is the first release model, with prediction error bands and calibration provenance. Missing inputs produce unavailable estimates and explicit assumptions. An LLM cannot fill numeric gaps with apparent measurements. Maintain recommendation/outcome records per branch and compare predictions when their horizon is reached.

## Driver communication

Critical deterministic conditions bypass model and routine notification delays. One arbiter prioritizes alerts, merges correlated conditions, deduplicates source events, applies persistence/hysteresis and enforces a 30-second routine message gap. Raw events and accepted agent outputs remain available in history even when speech is suppressed. Acknowledgment silences the current condition until escalation or a materially changed recommendation; it does not clear a fault.

Thermal profiles are sourced provisional references with editable surface/core/compound/season scope and conspicuous confidence. Unknown applicability stays neutral. The bundled legacy surface warning points (>103 C, >108 C) are an explicitly uncalibrated fallback for 2025 modern dry compounds. They do not define a verified optimum or core-temperature thresholds. New proposed cold/target/core bands require a source entry and calibration label; without one those bands remain neutral. The UI still renders the full blue/green/orange/red vocabulary for applicable profiles and fixtures. This avoids manufacturing a physical optimum while allowing user profiles.

## UI and Windows

See ui/README.md and the saved approved-workspaces.html, Drive/Pit wall screenshots, and editable layout drawing. Focused Drive has stable positions, circular wear percentages, very large surface/core temperatures, medium ERS/speed values, current decision, and track context. No scroll at 1920x1080 or 2560x1440 at 100% scale. At text zoom/reflow use the accessible detailed view instead of clipping. Independent wear/temperature states include words and arrows. Illustrative mockup percentages require real capacity context in production.

Workspaces: Drive, Pit wall (Drivers/Teams), History (season tabs), Analysis, Setups, Settings. One selected historical session drives all historical views with a persistent recorded/replay banner; a replay never replaces the live source or emits live agent calls. Unknown, restricted, stale and disconnected states never say Nominal.

Windows: .NET 10 LTS, WinUI 3, unpackaged self-contained x64 portable ZIP, single-instance app, tray continuation. Bundle server and UI; one supervisor starts/adopts its own child by PID plus executable identity and health instance_id, never kills an unrelated process using a port. Stop/Exit requests flush with a 10-second grace period; timeout offers retry/explicit force with data-loss wording, never silently terminates. Use native MediaCapture/MediaPlayer for voice, RegisterHotKey for a configurable global push-to-talk key (default Ctrl+Alt+Space); if exclusive game input blocks it, show an explicit unsupported-binding message and retain text. No Windows service or autostart is installed.

## Agent boundary

Eino owns a bounded tool loop inside one software-scheduled invocation. The application owns cadence, evidence, cancellation, context budgets and accepted strategy. Read-only domain tools replace shell/filesystem/computer control. Use the exact prompts/v1 files. Continuous mode is one logical conversation fed by software, with bounded provider requests, not a model polling tools forever. See contracts.md for scheduler state transitions and provider continuity.

## Migration and release

Develop Go parity beside the baseline; do not delete Python during the first change. Switch frontend transport, Docker entrypoint/ports, docs and launcher coherently after parity. The new v1 API intentionally replaces legacy WS 8765 payloads, with no compatibility server. Retain legacy .f1bin import. Existing history files remain untouched. Until final qualification, the previous Python commit/image remains the rollback path; never rewrite recordings for downgrade.

Build dependency set: Go 1.27.1, Eino v0.9.21, Chat adapter v0.1.13, Responses/agenticopenai v0.2.4, resolving Sonic v1.15.0. Combined modules previously compiled CGO=0 on Windows amd64 and Linux amd64/arm64; real provider behavior remains unqualified. Candidate storage/transport versions are modernc.org/sqlite v1.60.1, github.com/coder/websocket v1.8.15, github.com/foxglove/mcap/go/mcap v1.9.0, github.com/parquet-go/parquet-go v0.32.0. Reproduce builds in the implementation lockfile; do not claim these candidates were runtime-tested.

The implementation goal stops at the concrete gates in HANDOFF.md and qualification.md. A missing provider, Windows host, or real game capture is an explicit external qualification blocker, never grounds for endless refactoring or fabricated success.
