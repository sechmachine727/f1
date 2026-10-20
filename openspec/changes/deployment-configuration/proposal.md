# Proposal

## Why

Docker and Windows deployments currently expose only a few transport and data-directory overrides, so an operator running the package on a different machine from the game must edit files inside the container, and the Windows app has no settings surface at all. Configuration must be discoverable, environment-driven where deployment makes files awkward, and visible in the product.

## What Changes

- Add a documented environment schema that covers listeners, recording limits, session storage, agent endpoints and secrets, search, and voice. Environment values override `config.json`, which overrides embedded defaults, matching the existing CLI-first precedence.
- Support Docker and Compose without bind-mounted configuration: UDP/HTTP bind addresses, data directory, recording cap and reserve, chunk/rotation sizes, agent endpoint URL/model/protocol/API key, search provider/key, and voice endpoint/key can all be supplied as environment variables.
- Keep secrets out of logs and API responses. A secret may come from an environment variable or the 0600 secrets file; configuration views report only whether a secret is present.
- Make the web Settings workspace a read-only report of every effective value, where it came from (environment, file, default, or CLI), which values require a restart, and safe instructions for changing them. It must not print secret values and is not the configuration surface for the Windows package.
- Make the Windows control center's Settings page the configuration surface for the portable Windows shipment, with its own local configuration written by the app and the same values shown in the web report.
- Keep the direct local/LAN model: no accounts, tokens, pairing, Origin checks, allowlists, or certificates.

## Capabilities

### New Capabilities

- `runtime-configuration`: environment-driven configuration for Docker and server installs, precedence, secret handling, read-only effective-value reporting in the web workspaces, and the native configuration surface for the Windows shipment.

### Modified Capabilities

None. The existing `windows-control-center` behavior is extended through the new capability's native-settings requirements so this planning change does not depend on an unarchived spec.

## Impact

- `server/internal/config`: environment schema, precedence, source tracking, validation, restart classification.
- `server/internal/httpapi`: configuration view includes source and restart metadata; no secret values.
- `deployment/` and `Dockerfile`: documented environment variables and Compose example.
- `race_engineer_hub/src/pages/Workspaces.tsx`: Settings workspace.
- `windows/`: control-center Settings surface and configuration handoff.
- `docs/rework/`: defaults and operator documentation.

Dependencies: `go-telemetry-core`, `windows-control-center`. This change does not implement application code; it defines the follow-up work.
