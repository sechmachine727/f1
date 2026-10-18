# go-telemetry-core evidence

Date: 2026-10-18. Branch: codex/f1-telemetry-redesign-impl.

## Commands

    rtk go test ./server/internal/protocol
    rtk go test ./server/internal/config
    rtk go test -race ./server/internal/state
    rtk go test -race ./server/internal/httpapi
    rtk go test ./server/internal/server -run TestLiveFixtureStream
    rtk go build ./server/cmd/f1tele

All pass on darwin/arm64, Go 1.27.1.

## Coverage

- 33 packet layouts for formats 2025 and 2026; declared sizes cross-checked against the official tables (packet 3 is a 33-byte prefix of the 45-byte union).
- 2026 golden corpus: one packet per ID including packet 16, decoded field-by-field against the supplied structures.
- Real 2025 capture parity: 13 datagrams from tests/fixtures/real_2025_packets.f1bin compared field-by-field.
- Malformed input: short header, unknown format, unknown packet ID and truncated body are rejected without touching state.
- Channel catalog golden file with 351 entries: canonical channels plus raw official fields, stable across repeated runs.
- Reducer: newer-per-key, late lower-rate packets, uint32 frame wrap, duplicate suppression, same-frame distinct events, stale expiry.
- Identity: separate session/fragment/branch, same-UID flashback keeps session, new UID starts a fragment, season conflict retains evidence and leaves the season unknown.
- Availability: restricted rival fuel/energy are null, not zero; zero speed stays available.
- HTTP: no Origin or credential requirement, snapshot then delta with base_seq, slow-client resynchronisation, 428 without If-Match, redacted config.
- UDP fixture to WebSocket integration test asserts an available speed value reaches the stream.

## Limits

Slow-client behaviour is tested at the hub level; no real browser was throttled during this check. The rendered UI check lives in telemetry-workspaces evidence.
