# Design

## Context

See `proposal.md` for motivation and `specs/` for the contract. Current state that shapes the approach:

- The Go server decodes every packet into canonical channels, records raw MCAP, and derives lap analytics at lap boundaries through `server/internal/analytics/tracker.go`. `lap_traces` already stores a 300-point distance/speed series per lap.
- The dashboard subscribes once at 20 Hz for 24 cars and paints at 10 Hz. Throttle and brake are decoded but excluded from that subscription, and the stream contract allows a higher rate for selected cars.
- Recorded views read only catalogued rows. Charts use Recharts through plain `ResponsiveContainer` blocks in `RecordedPit`, `RecordedAnalysis`, `DetailedTyresView` and the live Pit wall.
- `m_ersDeployMode` is 0 none, 1 medium, 2 hotlap, 3 overtake in the 2025 structures; the 2026 structures keep 0 to 2 and name 3 boost. Packet 16 carries `m_overtakeAvailable`, `m_overtakeActive` and `m_activeAeroMode` per car, with `m_2026Regulations`. `m_sessionTime` is in every packet header. These fields are read from `docs/rework/reference/structures-2025.txt` and `structures-2026.txt`; channel names and availability follow `docs/rework/contracts.md`.
- `energy.store_j`, fuel and tyre channels are marked restricted for other cars when a participant's telemetry is not public. The analytics tracker currently reads raw packet values without that restriction.
- Team identifiers map through the appendix values already kept in `common/f1_structs/f1_constants.py`. Livery bytes are decoded as an opaque string, so they are not a usable colour source yet.

## Goals / Non-Goals

Goals: store enough within-lap evidence to draw throttle, brake, gear and ERS over a whole race; expose it with bounded queries; give the live Pit wall a two-car comparison built from the existing metric contract; and give every analysis chart one zoom and pan model.

Non-goals: changing lap or stint derivation, adding a chart library, reading livery bytes, interactive Drive sparklines, and model-backed comparison. The strategy, alert and engineer paths are untouched.

## Decisions

### Per-lap sample blob, not per-channel trace rows

Add `lap_samples`, one row per session, branch, car and lap, with a JSON blob of fixed tuples `[session_time_ms, throttle, brake, gear, ers_j, deploy_mode, overtake_available, overtake_active]`. Missing or restricted values are `null`.

Alternatives: five rows in `lap_traces` per lap costs five blobs and five lookups for one view; querying the MCAP on demand rescans a multi-gigabyte file on every zoom and breaks the recorded-view rule that charts come from catalogued evidence. The single blob keeps one index lookup per lap, and 900 points bound the row to roughly 40 KB.

`lap_traces` stays as is for the existing distance-aligned speed comparison. Rewriting its format would break recorded sessions.

### Sample on packet 6, enrich from packets 7 and 16

Throttle, brake and gear arrive on packet 6, which is the per-car high-rate stream. The tracker samples each car when packet 6 is observed and uses the latest ERS value from packet 7 and the latest overtake or aero state from packet 16, both held in the existing car state. ERS energy and overtake values change far more slowly than throttle and brake, so using the latest status value for a packet 6 sample is below the band's visual resolution.

The in-memory sample keeps distance and speed for the existing distance-aligned trace and adds session time, throttle, brake, gear, ERS energy and overtake state for the new tuple. The 4000-sample per-lap memory bound stays, and the added fields keep the worst case near 6 MB across 24 cars.

### Decimate with protected changes

At lap close, keep at most 900 samples. Keep the first and last sample, then every index where gear changes or brake crosses between zero and non-zero, then fill the remaining quota at an even stride over the rest. A 90-second lap at 60 Hz becomes roughly 10 Hz of stored detail while every shift and brake application survives.

Alternatives: plain stride drops some shifts; per-bucket averages smooth braking into ramps. Both would make the zoomed view lie about what the driver did.

### Deployment labels follow the regulation season

The existing `DeployModeLabel` returns Overtake for mode 3 in every session. The band reads the raw enum with the session's regulation season, and the stream and export labels become season-aware: Overtake for 2025, Boost for 2026, and the numeric value with an unclassified label when the season is unknown. A 2026 session draws the separate overtake availability strip from packet 16.

Band hues stay outside the four status colours: none slate `#3a414d`, medium indigo `#5c7cfa`, hotlap violet `#b58cff`, push state magenta `#ff5fa2`; the 2026 overtake strip uses teal `#38d6c4` for available and a filled variant for active. Every band segment carries a text label.

### Restriction is derived, not guessed

Extend the participant record with `m_yourTelemetry`. Migration 005 adds `your_telemetry` to `session_participants`, defaulting to public so existing rows keep their current meaning. When the setting is restricted and the car is not the player, ERS, deployment, fuel and tyre values are stored as `null` in samples. Throttle, brake and gear are broadcast for all cars and stay numeric.

This also stops the new charts from drawing a flat zero ERS line for a restricted rival, which would read as a real telemetry state. Fuel and tyre aggregates keep their current derivation; this change only corrects the sample series.

### One read route, completed laps only

`GET /api/v1/sessions/{id}/telemetry?branch=<id>&cars=0,1&from_lap=1&to_lap=20` returns the session, branch and regulation season, then cars with name, team, restricted flag and per-lap `start_ms`, `lap_ms` and points. Limits: four cars, all laps when no range is given, unknown session is 404, and a session with no derived samples returns an empty list with a coverage count rather than an error.

The route reads SQLite only. It does not touch recordings, so a recorded view never waits on an MCAP scan. The in-progress lap is absent until the lap closes, which is why the live view appends its own buffer.

Analysis requests each involved car over the span of its picked laps and filters to those laps in the browser, so a two-lap comparison does not load the whole race. The recorded Pit wall requests the selected lap range directly.

### Live graphs combine catalogued laps with the current lap

The live Pit wall opens a second `/stream` subscription for the selected cars only, for speed, throttle, brake, gear, ERS store, deploy mode, overtake and boost, at 20 Hz. The module keeps a ring of the current lap plus a short margin. Completed laps come from the telemetry route on mount and after each lap increment; the chart concatenates them with the live buffer. This keeps the dashboard's 24-car subscription unchanged and preserves the browser memory budget.

Alternative: adding throttle and brake to the existing 24-car subscription raises every page's stream cost for data only two panels draw.

### One shared chart window above Recharts

A single `TraceStack` owns `[startMs, endMs]`, follow state and the pointer readout, and renders three stacked panels on the same axis: throttle and brake, gear, ERS with its state band. Team colour identifies the car; line style distinguishes throttle from brake. Each panel receives only the window's samples, decimated to roughly twice the plot width with min and max preserved per bucket.

Zoom uses wheel or pinch about the pointer, plus visible buttons and keys. Pan uses drag, horizontal scroll and arrow keys. A navigator strip under the stack shows the whole session and a draggable window rectangle. The window is clamped to the data range, and the 100 percent state means the full range regardless of session length. Recharts stays the renderer, with animation disabled.

Alternatives: a canvas renderer handles more points but duplicates axes, tooltips and accessibility; a wide scrollable inner chart lets the browser scroll but forces every stored point into the DOM at high zoom.

### Follow-latest is explicit

When the window end is at the latest sample, new samples extend it. Any pan or zoom away from the end sets follow off and freezes the window. The follow control restores the previous span at the newest sample and resumes tracking. No sample insertion moves a window the operator is reading.

### Team identity colours

Add a small team map using the appendix identifiers already in the repository: Mercedes `#27F4D2`, Ferrari `#E8002D`, Red Bull `#3671C6`, Williams `#64C4FF`, Aston Martin `#229971`, Alpine `#FF87BC`, RB `#6692FF`, Haas `#B6BABD`, McLaren `#FF8000`, Sauber `#52E252`. Each entry supplies a base colour for lines and markers and a lightened ink colour that reaches 3:1 against the `#0b0d11` surface. Unknown, generic, custom and future identifiers use neutral grey. Status colours keep their existing values and are never derived from team colour.

The 2026 pack may add teams that the legacy appendix does not name. They fall back to neutral and keep their numeric team label until the 2026 appendix is verified.

### Live comparison layout

The live Pit wall keeps the standings table and turns row selection into a set of at most two cars, replacing the earliest when a third is chosen. The detail region shows the two cars side by side with a header tinted by team colour, a comparison grid for the shared metrics with deltas, then the trace stack and the existing pace and pit panels. A single selection shows one header and one detail column.

### History metadata without a new endpoint

The sessions list already returns `started_at_ms`, `ended_at_ms`, `session_type`, `best_lap_ms` and `track_id`. History resolves the circuit name through the existing `getCircuitInfo` helper and formats the date and duration locally. No server change is needed for this part.

## Risks / Trade-offs

- Full-race response size: four cars at 60 laps is roughly 9 MB of JSON. → The UI requests only the selected cars, and the route rejects requests beyond four. If real races show slower loads, add range paging by lap without changing the point format.
- Live buffer covers the current lap only. → Completed laps come from SQLite after each lap; a page opened mid-lap starts its buffer at that moment and gains the full lap when it closes.
- Packet 16 is a slower status packet, so overtake availability can lag the band by up to its send interval. → The band shows state changes, not activation distance, and the tooltip states the sample time.
- Static team colours can miss a new 2026 team or a custom livery. → Neutral fallback and the numeric team label keep the car identifiable.
- The tracker now samples eight values per packet 6 car entry. → The existing 4000-sample cap bounds memory; the soak test must be rerun to confirm the ingest budget.
- Backfill replays a whole session rather than resuming mid-session. → Sessions already derived are skipped; an interrupted session is redone idempotently. The spec scenario is worded for completion without duplication.

## Migration Plan

1. Add migration 005 with `lap_samples` and `session_participants.your_telemetry`. Both operations are additive.
2. Bump the analytics method version so new rows are identifiable; leave `lap_traces` untouched.
3. Extend backfill to select sessions that have no `lap_samples` rows, including sessions with an existing summary. It rewrites derived rows only and never opens a recording for write.
4. Ship the server before the dashboard so the route exists when the UI asks for it. The UI degrades to the lap-aggregate charts when the route returns no samples.
5. Rollback is the previous commit's binary and web bundle. The new tables and columns are unread by the old code; recordings stay unchanged.

## Open Questions

- The 2026 appendix may rename or add team identifiers. Confirm against the supplied 2026 PDF when a real capture is available; the neutral fallback keeps the UI correct until then.
- 900 samples per lap is chosen to keep full-race JSON manageable. A real race review may justify 1200; the bound is a constant and the format does not change.

