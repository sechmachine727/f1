# Sourced setup research

## Why

A popular one-lap setup can be unsuitable for another car generation or a race stint. Recommendations need inspected sources, applicability and measurable player-applied experiments.

## What Changes

- Integrate configurable Tavily/SearXNG and user URLs as bounded research tools.
- Retain inspected source/build/season/track/weather/assists/use-case context and handling tradeoffs.
- Store recommendations and experiments without game control or setup application.

## Capabilities

### New Capabilities

- `setup-research`: Sourced setup research with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

server/internal/setup; setup-research prompt/tools; sourced setup records; Setups/Analysis UI; search configuration.

Dependencies: `scheduled-race-engineer`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
