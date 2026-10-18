# f1tele server

One Go process serves the dashboard, records received telemetry, catalogs sessions, and runs the deterministic strategy and race-engineer pipeline. Read docs/rework/master-design.md for the product rules.

## Run

    go build -o f1tele ./server/cmd/f1tele
    ./f1tele serve --data-dir ./data

Defaults are 0.0.0.0:8081 for HTTP, 0.0.0.0:20777 for UDP, and a data directory under XDG_DATA_HOME or LOCALAPPDATA. Override with flags --http, --udp, --data-dir, --config, or the F1TELE_* environment variables. Precedence is CLI, environment, config.json, then embedded defaults.

## Game settings

To try the dashboard without the game, run the corpus generator against a local server:

    go run ./server/cmd/f1demo -seconds 120 -rate 512

It sends the same synthetic 2026 payloads the rated workload uses. It is a development helper, not game evidence.

In F1 25, open Settings, then Telemetry Settings, and set:

| Setting | Value |
|---|---|
| UDP Telemetry | On |
| UDP Broadcast Mode | Off |
| UDP IP Address | 127.0.0.1 |
| UDP Port | 20777 |
| UDP Send Rate | 60 Hz |
| UDP Format | 2026 |
| Your Telemetry | Public (restricted mode nulls rival channels) |

For a second machine on the LAN, set the address to this PC's LAN IP instead of 127.0.0.1.

The dashboard shows awaiting_telemetry until the first datagram arrives. That state is normal. live requires packets within the last two seconds.

## Health and states

GET /api/v1/health reports the process state, active session, recording state, and which optional subsystems are available. Readiness never depends on AI. Recording starts on the first telemetry that establishes a session fragment.

## Storage

Raw datagrams are written to data/recordings/<session>/<fragment>/ as Zstandard-compressed, CRC-protected MCAP with indexes. The catalog is data/catalog.db (SQLite, WAL). Defaults: 100 GiB cap, 5 GiB free reserve, warning at 80%, no automatic deletion.

Recording stops visibly when the cap, reserve, queue, or disk fails. Live telemetry continues. POST /api/v1/history/cleanup requires explicit IDs and confirmed: true; it refuses active recordings and removes catalog rows only after the file is gone.

Recovery copies readable chunks into a new file and reports the lost tail. Import accepts legacy .f1bin <QH timestamp/length records and preserves the original file.

## Recording and replay

Recording is automatic. POST /api/v1/recording/stop flushes and closes the current file; POST /api/v1/recording/start opens a new one after an explicit stop. Replays load a recording into their own reducer, so seeking never changes live state or live dispatch.

Tested fixture workflow:

    go test ./server/internal/history -run 'TestRecordingRoundTrip|TestLegacyImportAndRecovery'
    go test ./server/internal/server -run TestReplayIsolation

## Analysis and exports

POST /api/v1/analysis/query returns bounded series over a half-open session-time range, capped at 24 cars, 16 channels and 20000 points per series, with a 30-second deadline. POST /api/v1/analysis/compare aligns two to four laps by distance and leaves gaps where samples are farther than 250 ms apart.

    curl -s -X POST localhost:8081/api/v1/exports -H 'Content-Type: application/json' \
      -d '{"session_id":"<id>","branch_id":"<id>","range":{"from_seq":"1","to_seq":"100000"},"cars":[0],"channels":["car.speed_mps"],"format":"csv"}'
    curl -s localhost:8081/api/v1/jobs/<job-id>
    curl -OJ localhost:8081/api/v1/downloads/<download-id>

Each ZIP holds data.<format> and manifest.json. CSV text fields that start with =, +, - or @ are prefixed with an apostrophe, and the manifest records that policy.

Cancelling a job passes request cancellation into the scan. Exports run only when the free reserve allows; a refused export leaves history untouched.

## Configuration and secrets

GET /api/v1/config returns the active and desired configuration plus secrets_configured booleans. PATCH requires If-Match with the current revision; a missing precondition returns 428 and a stale one 409. Secret values are persisted in secrets.json with mode 0600 and are never echoed.

Endpoint, model, protocol, or search changes take effect on the next invocation. Bind-address and data-directory changes require a restart and are listed in restart_required.

## Race engineer

The server embeds docs/rework/prompts/v1 byte-for-byte and verifies the manifest at runtime. Calls use Eino with the Chat Completions adapter; the Responses adapter is not wired in this build. Tools are read-only and restricted by profile; setup research alone exposes search and fetch.

Scheduling is software-owned: lap, event, interval, manual and ALL/ANY rules with per-rule cooldowns. Dispatch and evidence cursors are separate, so a failed, cancelled or superseded invocation leaves pending Safety Car and weather evidence for the next window. Rolling 60-minute request and token ceilings block new invocations rather than partially sending them.

## Voice

Voice routes accept WAV 16-bit PCM mono 16 kHz, at most 30 seconds and 1 MiB, and proxy to the configured provider with server-side keys. POST /api/v1/voice/speech returns audio/wav for an accepted invocation.

## Troubleshooting

- bind udp ... address already in use: another process owns 20777. Stop it or pass --udp.
- Health stays awaiting_telemetry: check the game's IP, port, and that broadcast mode is off.
- stopped_limit: the cap or free reserve was reached. Delete recordings explicitly, then start a new recording.
- Recording state stopped_error: the writer failed. The log carries the reason; existing files stay readable.
