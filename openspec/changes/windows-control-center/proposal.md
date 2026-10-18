# Windows control center and optional voice

## Why

Nontechnical Windows users need a native start-and-open workflow without Docker. Voice capture must work without browser certificate setup and preserve normal server/recording lifecycle.

## What Changes

- Bundle a native Go server in an unpackaged self-contained Windows11 x64 WinUI3 control center.
- Start/open automatically, continue in tray, supervise only the owned server and flush on Stop/Exit.
- Add optional native microphone/audio plus global push-to-talk and configurable voice endpoints.

## Capabilities

### New Capabilities

- `windows-control-center`: Windows control center and optional voice with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

windows/F1Telemetry.ControlCenter; .NET10/Windows App SDK1.8.260921001; server control/voice routes; portable ZIP packaging; Windows qualification.

Dependencies: `telemetry-workspaces`, `scheduled-race-engineer`, `sourced-setup-research`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
