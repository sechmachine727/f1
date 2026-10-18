# Driver and pit-wall workspaces

## Why

The current dashboard is insufficient for sub-second driving decisions and durable race review. The approved mockup establishes a dense readable focused view plus detailed pit-wall/history workflows.

## What Changes

- Build Drive, Pit wall, History, Analysis, Setups and Settings from the existing frontend stack and one shared connection.
- Preserve approved tyre/temperature/decision hierarchy, semantic colors and no-scroll landscape layouts.
- Expose actual empty/loading/stale/restricted/replay/recording-error states and accessible controls.

## Capabilities

### New Capabilities

- `telemetry-workspaces`: Driver and pit-wall workspaces with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

race_engineer_hub/src/{pages,components,hooks,lib}; existing tokens/primitives/circuit assets; UI reference artifacts; browser workflow verification.

Dependencies: `go-telemetry-core`, `durable-race-history`, `telemetry-analysis-exports`, `deterministic-race-strategy`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
