# Operator configuration

Every value resolves in this order: command line, environment, `config.json`, embedded defaults. The Settings workspace shows the effective value and its source. Environment variables are the supported way to configure Docker images and native installs without editing files inside the container or application bundle.

Restart means the running process must restart for the change to take effect. Values marked live apply to the next invocation or the next recorded datagram.

| Variable | Default | Restart | Meaning |
|---|---|---|---|
| `F1TELE_DATA_DIR` | platform data directory | yes | Catalog, configuration and recordings root |
| `F1TELE_CONFIG` | `<data dir>/config.json` | yes | Configuration file path |
| `F1TELE_HTTP_ADDR` | `0.0.0.0:8081` | yes | Dashboard and API listener as `host:port` |
| `F1TELE_UDP_ADDR` | `0.0.0.0:20777` | yes | Telemetry listener as `host:port` |
| `F1TELE_UDP_ADVERTISE_HOST` | first non-loopback IPv4 | live | Host shown to the game when it runs on another machine |
| `F1TELE_RECORDING_ENABLED` | `true` | live | Record received datagrams |
| `F1TELE_RECORDING_CAP_BYTES` | `107374182400` | live | Total recording cap in bytes (100 GB) |
| `F1TELE_RECORDING_FREE_RESERVE_BYTES` | `5368709120` | live | Free disk kept unused (5 GB) |
| `F1TELE_RECORDING_CHUNK_BYTES` | `1048576` | live | MCAP chunk size in bytes |
| `F1TELE_RECORDING_ROTATE_BYTES` | `268435456` | live | Recording rotation size in bytes |
| `F1TELE_RECORDING_CHUNK_FLUSH_MS` | `1000` | live | Checkpoint cadence in milliseconds |
| `F1TELE_SECURITY_ORIGIN_CHECK` | `true` | live | Reject cross-origin browser requests |
| `F1TELE_AUTH_PASSWORD` | generated once | live | Dashboard password; never returned by the API |
| `F1TELE_AGENT_ENABLED` | `false` | live | Enable model-backed engineer calls |
| `F1TELE_AGENT_ENDPOINT_ID` | `env` | live | Identifier of the endpoint to use |
| `F1TELE_AGENT_BASE_URL` | none | live | OpenAI-compatible base URL |
| `F1TELE_AGENT_MODEL` | none | live | Model name sent to the provider |
| `F1TELE_AGENT_PROTOCOL` | `chat_completions` | live | `chat_completions` or `responses` |
| `F1TELE_AGENT_API_KEY` | none | live | Provider key; never returned or rendered |
| `F1TELE_SEARCH_PROVIDER` | `disabled` | live | `tavily`, `searxng` or `disabled` |
| `F1TELE_SEARCH_BASE_URL` | none | live | Search endpoint |
| `F1TELE_SEARCH_API_KEY` | none | live | Search key; never returned or rendered |
| `F1TELE_VOICE_ENABLED` | `false` | live | Enable transcription and speech |
| `F1TELE_VOICE_BASE_URL` | none | live | Voice provider base URL |
| `F1TELE_VOICE_TRANSCRIPTION_MODEL` | none | live | Transcription model |
| `F1TELE_VOICE_TRANSCRIPTION_API_KEY` | none | live | Transcription key; never returned or rendered |
| `F1TELE_VOICE_SPEECH_MODEL` | none | live | Speech model |
| `F1TELE_VOICE_SPEECH_API_KEY` | none | live | Speech key; never returned or rendered |

## Docker

The image binds `0.0.0.0:8081` and `0.0.0.0:20777` by default and stores data in the `/data` volume. The bundled Compose file passes the variables above from the host environment. When the game runs on another machine, set `F1TELE_UDP_ADVERTISE_HOST` to the Docker host's LAN address; the dashboard shows `<host>:20777` as the address to enter in the game.

```sh
F1TELE_UDP_ADVERTISE_HOST=192.168.1.20 docker compose up -d
```

To move the dashboard port, set `F1TELE_HTTP_ADDR` and change the published port in Compose together. To point the game at a different UDP port, set `F1TELE_UDP_ADDR` and publish that port instead.

## Invalid values

Startup fails and names the variable when a documented variable cannot be parsed. A value owned by the environment cannot be changed through `PATCH /api/v1/config`; the API returns `409 environment_locked`. Change the variable and restart instead.

## Windows

The portable Windows package is configured in the control center's own Settings page, not in the web UI. That page reads the bundled server's effective values and writes the app's local configuration; the web Settings workspace only reports what is running. A variable set in the user environment still overrides the local file, and the native page labels it as set by the environment instead of offering an edit that would have no effect.
