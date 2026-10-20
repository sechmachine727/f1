# Design

## Decisions

Configuration keeps the existing precedence: CLI, environment, `config.json`, embedded `defaults.json`. `config.Load` gains a source map keyed by dotted config path (`http`, `udp`, `data_dir`, `recording.cap_bytes`, `agent.endpoint.<id>.base_url`, and so on) so the API can report where each effective value came from without duplicating precedence logic in the UI.

Environment application happens in one table-driven function after the file is decoded and before validation. Each entry declares its variable name, target setter, value parser, and whether it is restart-only. Unknown `F1TELE_*` variables do not fail startup; invalid values for known variables do. Parsers accept bytes as plain integers, durations as Go duration strings or milliseconds, booleans as `true`/`false`/`1`/`0`, and lists as comma-separated values.

The supported variables are:

| Variable | Target | Restart |
|---|---|---|
| `F1TELE_DATA_DIR` | data directory | yes |
| `F1TELE_CONFIG` | configuration file path | yes |
| `F1TELE_HTTP_ADDR` | HTTP listen `host:port` | yes |
| `F1TELE_UDP_ADDR` | UDP listen `host:port` | yes |
| `F1TELE_UDP_ADVERTISE_HOST` | LAN host shown to the game | no |
| `F1TELE_RECORDING_ENABLED` | recording enabled | no |
| `F1TELE_RECORDING_CAP_BYTES` | storage cap | no |
| `F1TELE_RECORDING_FREE_RESERVE_BYTES` | free reserve | no |
| `F1TELE_RECORDING_CHUNK_BYTES`, `F1TELE_RECORDING_ROTATE_BYTES` | chunk and rotation sizes | no |
| `F1TELE_RECORDING_CHUNK_FLUSH_MS` | checkpoint cadence | no |
| `F1TELE_AGENT_ENABLED` | agent master switch | no |
| `F1TELE_AGENT_ENDPOINT_ID` | selected endpoint | no |
| `F1TELE_AGENT_BASE_URL`, `F1TELE_AGENT_MODEL`, `F1TELE_AGENT_PROTOCOL` | endpoint definition | no |
| `F1TELE_AGENT_API_KEY` | endpoint secret | no |
| `F1TELE_SEARCH_PROVIDER`, `F1TELE_SEARCH_BASE_URL`, `F1TELE_SEARCH_API_KEY` | search | no |
| `F1TELE_VOICE_ENABLED`, `F1TELE_VOICE_BASE_URL`, `F1TELE_VOICE_TRANSCRIPTION_MODEL`, `F1TELE_VOICE_TRANSCRIPTION_API_KEY`, `F1TELE_VOICE_SPEECH_MODEL`, `F1TELE_VOICE_SPEECH_API_KEY` | voice | no |

`F1TELE_AGENT_API_KEY`, `F1TELE_SEARCH_API_KEY`, and the voice keys are resolved through the existing secret lookup: environment first, then `secrets.json`. When only the generic variable is set, it is used for the selected endpoint; endpoint-specific variables continue to work. Secret values never enter `ConfigView`; only `secrets_configured` booleans and the variable name are exposed.

## API and data flow

`ConfigView` adds `sources: map[string]string` (values `cli`, `env`, `file`, `default`) and `restart_fields` remains the list of changed restart-only keys. No new endpoint is required. The web Settings workspace reads `GET /api/v1/config` and renders grouped sections: Server, Recording, Race engineer, Search, Voice, Deployment. Each row shows the effective value, source, and a restart badge. Secret rows show `configured` or `missing: VARIABLE`.

The Windows portable shipment is configured in the control center's native Settings page, not in the web UI. That page reads the bundled server's effective configuration for display and owns the app's local configuration file for edits (listeners, recording limits, endpoint URL/model/protocol, key status, voice). The web Settings workspace remains a read-only report for every deployment; it never edits the Windows package. Both surfaces read the same server endpoint, so they agree on effective values, and the native page additionally exposes the configuration file location and restart state.

## Docker and native packaging

Compose passes the variables directly to the service and documents a UDP bind of `0.0.0.0:20777` so a game on another machine can reach it. The Settings page computes the advertised address from `F1TELE_UDP_ADVERTISE_HOST` or the first non-loopback interface address and shows the exact `host:port` to enter in the game. No new service, sidecar, or token is introduced.

## Configuration ownership

| Deployment | Configured by | Web Settings |
|---|---|---|
| Docker or server binary | Environment variables or `config.json` | Read-only report |
| Windows portable ZIP | Native control-center Settings | Read-only report of the same effective values |

Environment variables still override a Windows local configuration file when set, matching the documented precedence. The native Settings page shows that a value is environment-locked instead of offering an edit that cannot take effect.

## Validation and failure behavior

Validation happens after all sources are merged, using the existing `Validate`. Parse errors are wrapped with the variable name and expected form. A value that is environment-controlled cannot be changed through `PATCH /config`; the API rejects the change with a clear conflict rather than silently accepting a value that will disappear on restart. Restart-only changes remain in `restart_required` until the process restarts.

## Risks / trade-offs

- Environment precedence can surprise users who edit `config.json` in a container that also sets variables. Settings shows the source, and the operator reference states precedence explicitly.
- Secret values read from the environment cannot be rotated without recreating the process; that is the normal Docker model and is preferable to writing secrets into the image.
- Showing LAN addresses relies on interface discovery; a host with several interfaces may need `F1TELE_UDP_ADVERTISE_HOST` to name the right one.

## Migration

Existing `F1TELE_HTTP_ADDR`, `F1TELE_UDP_ADDR`, `F1TELE_DATA_DIR`, `F1TELE_CONFIG`, and endpoint-specific key variables keep their meanings. No existing configuration file or recording is rewritten.
