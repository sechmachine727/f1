# Deterministic race strategy and alerts

## Why

Useful race calls require repeatable estimates and a controlled notification policy. Existing threshold logs can repeat without considering persistence, deployment intent or driver attention.

## What Changes

- Calculate pace, tyre degradation, fuel/energy projections, pit loss and legal pit/rejoin alternatives with evidence and error bounds.
- Separate deterministic critical alerts from model calls and routine driver notifications.
- Provide sourced editable provisional reference profiles and recommendation/outcome records.

## Capabilities

### New Capabilities

- `race-strategy`: Deterministic race strategy and alerts with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

server/internal/strategy; source/profile resources; strategy/alerts routes; history recommendations; driving/pit-wall estimate inputs.

Dependencies: `durable-race-history`, `telemetry-analysis-exports`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
