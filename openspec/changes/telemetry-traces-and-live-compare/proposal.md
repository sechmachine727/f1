# Proposal

## Why

The live Pit wall still behaves like a standings table with a single small detail column, so a second person watching the session cannot inspect or compare cars in the depth Drive gives the player. Recorded views show lap aggregates but almost no within-lap evidence: throttle, brake, gear and ERS deployment are recorded in the raw datagrams and never surfaced. A full race also produces charts that cannot be read at race length because they can only render in full.

## What Changes

- Make each live Pit wall driver row open a live detail panel with the values Drive uses for the player: speed, gear, throttle and brake, ERS store and deployment state, fuel margin, tyre wear and temperatures, damage, position and gaps, current stint and recent laps.
- Allow two drivers to be inspected together, including the player, with team-coloured identity for each car and the existing blue, green, orange and red status colours kept for cold, normal, caution and critical values. Compared values show deltas where both cars report an available number.
- Add time-axis telemetry charts for throttle, brake, ERS store energy and gear. A deployment band under the ERS chart shows none, medium, hotlap and the season's push state: overtake in 2025 and boost in 2026. A 2026 session also shows overtake unavailable, available and active as its own state. Unknown seasons and restricted channels stay neutral and labelled.
- Show the same telemetry charts in the recorded Pit wall and in Analysis, using completed-lap evidence stored with the session rather than rescanning raw recordings on every view. History opens the recorded Pit wall, so a session selected from History reaches the same charts.
- Capture per-lap time-series samples beside the existing lap analytics: session time, throttle, brake, gear, ERS energy and deployment or overtake state, decimated per lap and marked unavailable where the sender restricts a channel. Existing recordings gain the samples through the resumable backfill without rewriting their files.
- Make every analysis chart zoomable and scrollable: 100% shows the complete range, wheel, pinch and keyboard zoom, drag and scroll pan, an explicit zoom readout and a reset. A live chart adds a follow-latest control that returns the window to incoming samples after the operator pans away.
- Improve History rows with circuit name, date, duration and session type while keeping the existing filters and the recorded-view handoff. The `Recorded · selection drives history views` copy stays removed.

Out of scope: changing the deterministic strategy or analytics method, adding a chart framework, and Drive's inline glance sparklines, which stay static because they are read while driving.

## Capabilities

### New Capabilities

- `driver-comparison`: live Pit wall inspection and two-driver comparison, including team identity colours, status colour semantics, deltas and the live detail panel.
- `telemetry-charts`: time-axis throttle, brake, gear and ERS charts, deployment and overtake bands, zoom, pan, follow-latest and the shared chart behaviour used across workspaces.

### Modified Capabilities

- `session-archive`: per-lap time-series samples, restricted-channel availability, the session telemetry read route and backfill of existing recordings.
- `analytics-workspace`: recorded Pit wall and Analysis gain the telemetry charts and shared interaction; History gains circuit, date and duration detail.

## Impact

- `server/internal/analytics`: sample capture, decimation, restricted handling, backfill and the read model.
- `server/internal/history`: migration for per-lap samples and a batched query by session, branch and car.
- `server/internal/httpapi`: a read-only session telemetry route; existing routes stay compatible.
- `race_engineer_hub`: live Pit wall detail and compare, team colour mapping, shared zoomable chart component, recorded Pit wall and Analysis chart additions, History metadata.
- Tests: tracker derivation, restricted samples, backfill idempotence, route bounds, chart window arithmetic and team colour fallbacks.

Dependencies: `session-analytics`, `telemetry-workspaces` and `deployment-configuration` as already implemented. No new runtime dependency. The installed Recharts version remains the renderer.
