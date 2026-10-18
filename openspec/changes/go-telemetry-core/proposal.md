# Go telemetry core

## Why

The Python bridge keeps partial live state and multiple browser sockets. A typed Go core provides complete 2025/2026 observations while isolating collection from UI and AI delays.

## What Changes

- Introduce validated full-payload 2025/2026 decoding, nullable measurement/channel catalog, source/session context and live state ownership.
- Serve embedded UI plus the v1 HTTP/stream contracts with direct local/LAN access.
- Preserve baseline units/flashback behavior while separating raw capture intake from live frame filtering.
- **BREAKING**: define v1 transport replacing legacy WS8765; retain Python baseline until packaging cutover.

## Capabilities

### New Capabilities

- `telemetry-ingestion`: Go telemetry core with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

Root go.mod; server/cmd/f1tele; server/internal/{protocol,state,httpapi,config}; shared contracts; existing capture fixtures and frontend transport boundary.

Dependencies: none. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
