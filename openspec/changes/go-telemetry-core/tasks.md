# Go telemetry core implementation tasks

Dependencies: none. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Protocol and config

- [ ] 1.1 Create go.mod and server/cmd/f1tele/main.go plus internal/config using the exact defaults/precedence; add config_test.go for overrides, invalid ports, redacted reads and atomic failed PATCH. Verify: rtk go test ./server/internal/config.
- [ ] 1.2 Port header/layout decoding into server/internal/protocol; add decoder_test.go using all packet-size fixtures and a checked-in2026 golden fixture derived from supplied structures. Cover all17 packet IDs, v1.2 events,24 slots, signed scales and invalid lengths. Verify: rtk go test ./server/internal/protocol.
- [ ] 1.3 Add protocol/channel_catalog.go and its golden JSON test for raw field coverage, restrictions, wheel mapping, sentinel and unit conversion. Verify available zero speed remains zero while restricted zero fuel becomes null.

## 2. State and transport

- [ ] 2.1 Create server/internal/state reducer/identity/freshness files and state_test.go. Exercise newer-per-key, same-frame distinct events, uint32 wrap, stale values, same-UID rewind and new UID. Verify: rtk go test -race ./server/internal/state.
- [ ] 2.2 Create server/internal/httpapi health/config/stream handlers using shared schema; add httptest coverage for direct LAN/no Origin rejection, initial snapshot, delta base sequences, resync and slow-client isolation. Verify: rtk go test -race ./server/internal/httpapi.
- [ ] 2.3 Wire serve startup, signal shutdown and read-only state into the executable; document exact game UDP settings and health states in server/README.md. Verify: rtk go build ./server/cmd/f1tele and a UDP fixture-to-stream integration test named TestLiveFixtureStream.

## 3. Change gate

- [ ] 3.1 Run rtk go test -race ./server/... and protocol golden parity against tests/fixtures/real_2025_packets.f1bin; save commands/results in docs/rework/evidence/go-telemetry-core.md. Verify no Python/application deployment cutover appears in this change's diff.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
