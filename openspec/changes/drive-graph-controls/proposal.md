# Proposal

## Why

Drive's three graphs are fixed: lap time, worst tyre wear and gap ahead. A driver cannot inspect ERS, fuel, engine or tyre temperatures without leaving the workspace, and the graphs carry no axis labels, so their scale is guesswork. The live Pit wall has the opposite problem: its pace chart only fetches catalogued laps for recorded sessions, so it stays empty while the player is driving.

## What Changes

- Give each of the three Drive graph slots a metric selector and a time-range selector. Defaults stay lap time, tyre wear and gap ahead.
- Label both axes on every Drive graph: time across the bottom, the metric unit up the side, with readable tick values.
- Expand the metric set to include lap time, worst tyre wear, gap ahead, gap to leader, ERS store, fuel mass, engine temperature, tyre surface temperature, tyre core temperature, speed, throttle, brake and brake temperature.
- Sample every supported metric continuously at 1 Hz, regardless of which metric a slot shows. Retain ten minutes per metric so switching a metric or range shows history instead of starting over.
- Offer 30-second, 1-minute, 2-minute, 5-minute and 10-minute ranges with a 2-minute default. A range only bounds the view; samples are deleted only when they pass the ten-minute retention.
- Keep every graph anchored at the newest sample. A stopped feed leaves the window at the last sample, and unavailable samples render as gaps.
- Refresh the live Pit wall from the active session's catalogued laps every two seconds so pace, stint and lap-based charts update while driving. Show the in-progress lap as a provisional pace point and keep the live trace stack appending the current lap.

Out of scope: free-form pan and zoom in Drive (the range selector replaces it), new backend channels, and changes to recorded History or Analysis.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `telemetry-charts`: Drive graph slots, metric and range selectors, labelled axes, continuous sampling, bounded retention and latest-sample following.
- `driver-comparison`: live Pit wall lap and pace charts refresh from the active session without a reload, including a provisional current lap.

## Impact

- `race_engineer_hub/src/components/Drive.tsx`: graph slots, sampling, selectors and axes.
- `race_engineer_hub/src/lib`: a metric registry, a bounded sample ring, and a shared live-laps poll.
- `race_engineer_hub/src/pages/Workspaces.tsx`: live Pit wall pace, stint and provisional lap.
- Tests: retention and trimming, metric switching without data loss, axis tick math, selector defaults, live lapse polling and provisional-lap replacement.
- No server, schema, API or recording change. The existing `GET /sessions/{id}/laps` route supplies the laps.

