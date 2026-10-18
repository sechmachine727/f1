# Scheduled race engineer

## Why

Continuous model polling is costly and loses useful context when attempts fail. Software-owned scheduling and separate evidence acceptance provide bounded coherent race advice.

## What Changes

- Embed Eino ADK with explicit Chat Completions/Responses endpoints and the exact parent-authored prompt bundle.
- Implement configurable lap/event/time/manual predicates and bounded optional continuous conversations.
- Keep dispatch/evidence cursors separate, carry failed-call evidence, validate outputs and persist portable/provider-specific continuity correctly.

## Capabilities

### New Capabilities

- `race-engineer`: Scheduled race engineer with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

server/internal/engineer; provider/config/tools/scheduler; invocation storage and APIs; prompts/v1 source integration; engineer UI events.

Dependencies: `go-telemetry-core`, `durable-race-history`, `telemetry-analysis-exports`, `deterministic-race-strategy`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
