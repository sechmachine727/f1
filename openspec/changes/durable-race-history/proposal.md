# Durable race history

## Why

In-memory state and sampled .f1bin captures cannot provide searchable all-car history or preserve complete flashback attempts. Durable raw recordings and a recoverable catalog enable reliable analysis.

## What Changes

- Record all received datagrams to indexed compressed MCAP and catalog sessions/fragments/branches/laps in SQLite.
- Preserve abandoned flashback attempts, surviving-prefix analysis, import/recovery and isolated replay.
- Enforce editable storage cap/free reserve with visible recording stop and manual cleanup only.

## Capabilities

### New Capabilities

- `race-history`: Durable race history with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

server/internal/history; recording/replay/import/history API handlers; legacy .f1bin reader contract; persistent volume and data path.

Dependencies: `go-telemetry-core`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
