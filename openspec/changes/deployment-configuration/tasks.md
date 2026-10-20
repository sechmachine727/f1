# Tasks

## 1. Environment schema

- [x] 1.1 Implement the table-driven environment loader in `server/internal/config` for every variable in design.md, keeping CLI > env > file > default precedence. Verify with unit tests for each precedence pair and for CLI winning over env.
- [x] 1.2 Add typed parsers for booleans, byte sizes, durations, addresses and comma lists, and fail startup with the variable name on invalid input. Verify malformed address, negative cap and bad boolean tests all return named errors.
- [x] 1.3 Add source tracking for effective values. Verify `GET /api/v1/config` reports `cli`, `env`, `file` or `default` for listeners, recording cap, data directory and agent endpoint, and never reports a secret value.
- [x] 1.4 Make environment-controlled values immutable through `PATCH /config` with a clear conflict response. Verify a patch to an env-controlled listener is rejected and leaves the effective config unchanged.

## 2. Docker and operator surface

- [x] 2.1 Add the documented variable table and a Compose example that binds UDP on `0.0.0.0:20777` and shows the game-facing `host:port`. Verify `docker compose config` and a container started with `F1TELE_UDP_ADDR=0.0.0.0:30777` reports that listener in health and Settings.
- [x] 2.2 Add `F1TELE_UDP_ADVERTISE_HOST` and LAN address selection, and show the exact address to enter in the game. Verify Settings shows the configured advertise host and falls back to a non-loopback interface.
- [x] 2.3 Document image defaults, secret variables and restart requirements in the operator reference. Verify every variable in design.md appears exactly once with its default and restart classification.

## 3. Web Settings

- [x] 3.1 Rebuild the web Settings workspace as a read-only grouped report of effective values with source labels and restart badges, including listeners, data directory, recording limits, agent endpoint/model/protocol, search, voice and feature availability. Verify rendered screenshots at 1920x1080 and 2560x1440 show no secret value, no editing control, and no horizontal overflow.
- [x] 3.2 Add empty, loading, error, and restart-required states with accessible labels and keyboard focus. Verify a failed config request and a changed UDP address render the right state without claiming the new value is active.

## 4. Native Settings

- [x] 4.1 Add the configuration surface to the Windows control center Settings page: read the bundled server's effective values, write the app's local configuration for editable settings, and show source and restart state. Verify control-center tests cover configured, missing-secret, restart-required and environment-locked rows.
- [x] 4.2 Add an open-configuration-folder action for the exact file path, without exposing secret values. Verify the path follows `F1TELE_DATA_DIR` and `F1TELE_CONFIG` when those are set.

## 5. Integration

- [x] 5.1 Run `go test -race ./server/...`, frontend tests/typecheck/lint/build, control-center tests, strict OpenSpec validation and the artifact validator. Verify all pass and the rendered Settings screenshots are stored in evidence.

## Workflow follow-up

- Archive this change only after implementation evidence and requested review; publishing requires separate authorization.
