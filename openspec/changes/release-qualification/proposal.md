# Packaging and release qualification

## Why

Component compilation cannot establish race reliability, provider compatibility or Windows usability. A finite evidence gate is needed before publishing the replacement runtime.

## What Changes

- Build native Windows and Linux amd64/arm64 Docker artifacts with the shared embedded UI and persistent storage.
- Qualify full-rate24-car capture, replay/exports, three clients, two-hour soak, real providers, Windows voice and live2026 data.
- Switch Docker/docs atomically and record explicit pass/fail/unavailable evidence plus rollback instructions.

## Capabilities

### New Capabilities

- `release-qualification`: Packaging and release qualification with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

Dockerfile; docker-compose.yml; docker/entrypoint.sh; .github/workflows/{ci,docker-publish}.yml; server/qualification; Windows release scripts; README/operator docs; evidence reports.

Dependencies: `go-telemetry-core`, `durable-race-history`, `telemetry-analysis-exports`, `deterministic-race-strategy`, `telemetry-workspaces`, `scheduled-race-engineer`, `sourced-setup-research`, `windows-control-center`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
