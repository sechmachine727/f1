# Design

## Context

See `proposal.md` for motivation and the two specs for the contract. Current state that shapes the approach:

- `Drive.tsx` renders three fixed `Spark` SVGs. Only gap ahead and worst tyre wear are sampled, once per second, into a 240-point ring. Lap times are polled from `GET /sessions/{id}/laps` every 15 seconds. There are no axis labels or selectors.
- The live Pit wall keeps `laps` only when `selection.sessionId` is set. While driving that id is null, so the pace chart and stint strip never receive catalogued laps. The trace stack does append current-lap samples through `useLiveSamples`.
- The catalog already exposes completed laps for the active branch through the existing laps route. No server change is needed.

## Goals / Non-Goals

Goals: three configurable, labelled Drive graphs with bounded ten-minute history; range changes that never reset history; a live Pit wall that updates pace, stint and traces while driving.

Non-goals: free pan/zoom in Drive, new telemetry channels, server changes, or altering recorded History and Analysis.

## Decisions

### Sample every metric into one shared ring set

Add a metric registry covering lap time, worst tyre wear, gap ahead, gap to leader, ERS store, fuel mass, engine temperature, tyre surface maximum, tyre core maximum, speed, throttle, brake and brake-temperature maximum. Each entry supplies a label, unit, precision, colour and a reader from the live snapshot.

One 1 Hz tick in Drive appends a value or null to each metric's ring. Rings hold 600 samples, ten minutes at 1 Hz. Slots read from the same rings. This keeps switching a slot instantaneous and preserves history; sampling only the three visible metrics would start a newly selected metric empty. Thirteen rings of 600 nullable numbers is negligible.

The rings live in refs, and the tick increments a version counter. React renders once per second instead of once per sample, matching the existing Drive budget.

Alternatives: keep per-slot buffers (duplicates data and loses history on switch); use the general analysis query (needs a recording scan for glance data and cannot show the current lap).

### Lap time is a timestamped series from a shared live-laps poll

Add `useLiveLaps(sessionId, branchId, enabled)` that polls the existing laps route every two seconds while live and returns the completed valid laps. Drive's lap-time ring appends new laps once, keyed by lap number, at the poll time. The same hook feeds the Pit wall pace and stint, removing the current recorded-only fetch.

Alternative: subscribe to lap events on the WebSocket. The stream carries events, but lap completion is not a guaranteed event payload, so polling the catalogue is the reliable path.

### Range changes only move the window

The range selector chooses 30 seconds, 1 minute, 2 minutes, 5 minutes or 10 minutes. Rendering uses `[newest - range, newest]`. Sampling always retains ten minutes and trims only samples older than that. Switching to 30 seconds and back to 10 minutes must show the old data.

The spec's "truncate outside the minimum range" is implemented as "do not render outside the selected window"; the retained ring is untouched.

### Follow the newest sample, freeze on stop

The window end is the newest sample. While live it advances with each tick. When packets stop, Drive already freezes the last snapshot; the graph then holds its last window and values. Unavailable samples remain null and break the line, so a restricted or missing channel never draws a zero.

### Compact SVG graphs with explicit axes

Extend `Spark` into a small graph component: a header row with two labelled selects (metric, range) and the latest value with unit; a plot with three Y ticks, three X ticks ending at the newest sample, axis titles, and a line that breaks at gaps. It reuses the existing SVG approach instead of adding Recharts to Drive, keeping the render cost low and the layout stable.

Axis ticks use compact time labels ("-5:00", "-2:30", "now") and metric precision. The vertical domain pads by a small fraction so a flat series still shows a meaningful band.

### Live Pit wall uses the same laps source

The live Pit wall uses `useLiveLaps` for the pace chart and stint strip. The pace chart adds the in-progress lap as a provisional point from `timing.current_lap_ms`, keyed by lap number; when the catalogue returns that lap, the final time replaces the provisional point. The trace stack keeps appending current-lap samples and refetches stored samples after each lap increment.

Recorded sessions keep their existing recorded fetch and never mix live laps.

## Risks / Trade-offs

- Poll placement error: a completed lap is timestamped when the two-second poll sees it, so a lap point can sit up to two seconds late on the time axis. The trend remains correct. If exact placement matters later, record lap completion wall time server-side.
- Thirteen rings at 600 points is about 7,800 values. Negligible memory, but the 1 Hz tick must not allocate a new map per metric.
- Three graphs with headers and axes need more vertical space than the old sparklines. The 1080p layout must keep the page from scrolling; verify both target resolutions.
- Provisional pace points must be keyed by lap number to avoid duplicates when the catalogue catches up.
- Polling every two seconds adds one small request per two seconds while live; it stops on recorded selections.

## Migration Plan

Frontend-only. No schema, API, recording or configuration change. Existing Drive defaults remain lap time, tyre wear and gap ahead. Deploy the dashboard with the server; no data migration or rollback work beyond the previous bundle.

## Open Questions

None.

