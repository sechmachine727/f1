# Settings

Every setting resolves in this order: command line, environment, `config.json`, embedded defaults. The Settings workspace shows the effective value and its source. Environment variables are the supported way to configure Docker and native installs without editing files inside a container or an application bundle.

Some values apply immediately; others need a restart. The Settings workspace marks the restart-only values.

## Where configuration lives

| Install | File | Secrets |
|---|---|---|
| Docker | <code>config.json</code> in the <code>/data</code> volume | <code>secrets.json</code> in the same volume |
| Native | <code>&lt;data dir&gt;/config.json</code>, default under the platform data directory | <code>secrets.json</code> beside it |
| Windows portable | Written by the control center's Settings page | Managed by the control center |

Set `F1TELE_CONFIG` to point at a different config file. Secret values are written with mode 0600 and are never returned by the API or rendered in the dashboard.

## Environment variables

| Variable | Default | Restart | Meaning |
|---|---|---|---|
| `F1TELE_DATA_DIR` | platform data directory | yes | Catalog, configuration and recordings root |
| `F1TELE_CONFIG` | `<data dir>/config.json` | yes | Configuration file path |
| `F1TELE_HTTP_ADDR` | `0.0.0.0:8081` | yes | Dashboard and API listener |
| `F1TELE_UDP_ADDR` | `0.0.0.0:20777` | yes | Telemetry listener |
| `F1TELE_UDP_ADVERTISE_HOST` | first non-loopback IPv4 | live | Host the dashboard tells you to enter in the game |
| `F1TELE_SECURITY_ORIGIN_CHECK` | `true` | live | Reject cross-origin browser requests |
| `F1TELE_AUTH_PASSWORD` | generated on first start | live | Dashboard password; never returned by the API |
| `F1TELE_RECORDING_ENABLED` | `true` | live | Record received datagrams |
| `F1TELE_RECORDING_CAP_BYTES` | `107374182400` | live | Total recording cap (100 GB) |
| `F1TELE_RECORDING_FREE_RESERVE_BYTES` | `5368709120` | live | Free disk kept unused (5 GB) |
| `F1TELE_RECORDING_CHUNK_BYTES` | `1048576` | live | MCAP chunk size |
| `F1TELE_RECORDING_ROTATE_BYTES` | `268435456` | live | Recording rotation size |
| `F1TELE_RECORDING_CHUNK_FLUSH_MS` | `1000` | live | Checkpoint cadence |
| `F1TELE_AGENT_ENABLED` | `false` | live | Enable model-backed engineer calls |
| `F1TELE_AGENT_ENDPOINT_ID` | `env` | live | Endpoint identifier to use |
| `F1TELE_AGENT_BASE_URL` | none | live | OpenAI-compatible base URL |
| `F1TELE_AGENT_MODEL` | none | live | Model name sent to the provider |
| `F1TELE_AGENT_PROTOCOL` | `chat_completions` | live | `chat_completions` or `responses` |
| `F1TELE_AGENT_API_KEY` | none | live | Provider key; never returned |
| `F1TELE_SEARCH_PROVIDER` | `disabled` | live | `tavily`, `searxng` or `disabled` |
| `F1TELE_SEARCH_BASE_URL` | none | live | Search endpoint |
| `F1TELE_SEARCH_API_KEY` | none | live | Search key; never returned |
| `F1TELE_VOICE_ENABLED` | `false` | live | Enable transcription and speech |
| `F1TELE_VOICE_BASE_URL` | none | live | Voice provider base URL |
| `F1TELE_VOICE_TRANSCRIPTION_MODEL` | none | live | Transcription model |
| `F1TELE_VOICE_TRANSCRIPTION_API_KEY` | none | live | Transcription key; never returned |
| `F1TELE_VOICE_SPEECH_MODEL` | none | live | Speech model |
| `F1TELE_VOICE_SPEECH_API_KEY` | none | live | Speech key; never returned |

For the exhaustive operator view with the same values, see [operator-configuration.md](rework/operator-configuration.md).

## Dashboard password

The dashboard, API and streams require a password. On the first start with a fresh data directory, the server generates a 20-character password, writes only its hash to `auth.json`, and prints the plaintext once to the process log. Later starts print nothing.

Set `F1TELE_AUTH_PASSWORD` instead when the deployment should not generate one. An environment password owns authentication and a reset command refuses to override it.

To replace the password without the old one:

```bash
docker compose exec f1tele /f1tele auth reset
```

The interactive command asks twice. A pipeline reads once and skips confirmation:

```bash
printf '%s\n' "$NEW_PASSWORD" | docker compose exec -T f1tele /f1tele auth reset --stdin
```

Reset rotates the session-signing key, so every browser is signed out. Passwords are 8 to 256 characters. There is no username or token-pairing flow.

Origin validation is enabled by default. It compares the Origin host to the request Host and rejects a mismatch. Set `F1TELE_SECURITY_ORIGIN_CHECK=false` only for a trusted proxy or client that cannot preserve the dashboard host. Requests without an Origin header remain usable by CLI clients.

The server still speaks HTTP. A password and session cookie are not protected on the wire; put a TLS reverse proxy in front of the service when it is reachable beyond a trusted LAN.

## Docker

The image binds `0.0.0.0:8081` and `0.0.0.0:20777` and stores data in `/data`. The bundled Compose file passes the variables from the host environment; empty values are ignored, so an unset variable keeps the default.

```bash
F1TELE_UDP_ADVERTISE_HOST=192.168.1.20 docker compose up -d
```

To move the dashboard port, set `F1TELE_HTTP_ADDR` and change the published port in Compose together. To point the game at a different UDP port, set `F1TELE_UDP_ADDR` and publish that port instead.

Recording caps accept raw byte counts. For example, a 20 GB cap:

```bash
F1TELE_RECORDING_CAP_BYTES=20000000000 docker compose up -d
```

## Model, search and voice

The engineer, search and voice features stay disabled until you configure a provider.

- `F1TELE_AGENT_ENABLED=true` and a base URL, model, protocol and key enable race-engineer requests.
- `F1TELE_SEARCH_PROVIDER=tavily` or `searxng` with a base URL and key enables setup research search.
- `F1TELE_VOICE_ENABLED=true` with a base URL, transcription model and speech model enables voice routes.

An OpenAI-compatible URL does not guarantee both protocols. Choose `chat_completions` or `responses` to match the provider. Missing endpoints leave the affected feature off; the dashboard states that instead of pretending it is live.

## Agent settings and activity

The Settings workspace is the web editing surface for race-engineer behavior:

- Agent: enablement, trigger rules and cadence, continuous mode, hourly budgets, deadlines, context and output limits, steps, tool calls, temperature and top-p.
- Prompt: per-profile revision editor with a side-by-side diff before saving, revision selection, inline deletion confirmation and reset to the authored prompt.
- Compaction: enablement, trigger percentage, recent-tail percentage, current usage and a manual Compact now action with optional focus.
- Connections and Security: read-only endpoint, recording, server, Origin and password state.

Endpoint URL, model, protocol and API key stay outside the web editor. Configure them through the environment, `config.json`, or the Windows control center.

The Logs workspace lists every invocation with status, trigger, model, prompt revision, tokens, estimated cost, latency, event count and decision. Open a row for the exact system-prompt snapshot, model-visible input, output, tool timeline, compaction history and raw JSON. The live Pit wall exposes the same activity in a drawer, and History links the recorded race to the session-filtered view.

Logs are append-only. They survive recording-file cleanup and are removed only with the catalogued session. Payloads are capped at 1 MiB per event; over-cap events store a truncation marker and hash. API keys, authorization headers, opaque provider continuation and hidden reasoning are never stored in activity payloads.

## Windows portable build

On first launch the control center opens its Settings window before starting the server. The dashboard password is at the top; a generated password appears once in a copyable dialog. Set the listeners, recording cap, model endpoint, search and voice here. Saving writes `config.json` and `secrets.json`, then starts the server.

Later Settings saves stop the owned server, write the files and restart it. A value set in the process environment overrides the local file and is shown read-only. The reset button accepts a new password and confirmation without the old password and signs out every browser session.

Agent and harness settings, prompt revisions and activity logs remain in the password-protected web dashboard.

## Invalid values and locking

Startup fails and names the variable when a documented value cannot be parsed. A value owned by the environment cannot be changed through `PATCH /api/v1/config`; the API returns `409 environment_locked`. Change the variable and restart instead.

Requests that change configuration use `If-Match` with the current revision. A missing precondition returns `428`; a stale revision returns `409`.

