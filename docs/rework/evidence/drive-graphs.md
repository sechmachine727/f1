# Drive graph controls and live Pit wall

Evidence for `drive-graph-controls`. Implemented 2026-10-20.

## What changed

- Drive has three configurable graph slots. Each has a metric selector, a range selector, a latest value, a labelled time axis, a labelled value axis, three time ticks and three value ticks.
- Metrics: lap time, worst tyre wear, gap ahead, gap to leader, ERS store, fuel, engine temperature, tyre surface maximum, tyre core maximum, speed, throttle, brake and brake temperature maximum.
- Every continuously available metric is sampled at 1 Hz into a ten-minute ring. Lap times enter the lap-time ring when the two-second catalogue poll reports a completed valid lap.
- Ranges are 30 seconds, 1 minute, 2 minutes, 5 minutes and 10 minutes; the default is 2 minutes. A range only changes the visible window. Samples are trimmed only at the ten-minute boundary, so switching ranges or metrics never clears history.
- The window ends at the newest sample and holds when the feed stops. Null samples break the line rather than drawing zero.
- The live Pit wall polls the active session's laps every two seconds. Pace and stint update while driving, and the current lap appears as a provisional dashed pace point until its final time replaces it. The trace stack keeps appending the current lap.

## Verification

| Check | Result |
|---|---|
| `npm test` | 76 tests, 16 files, pass |
| `npm run lint` | 0 errors, 17 pre-existing warnings |
| `npx tsc -p tsconfig.app.json --noEmit` | no new errors; two pre-existing: `ExpandableReportPanel.tsx`, `circuitData.ts` |
| `npm run build` | pass |
| Browser at 1920x1080 | no page scroll (1080/1080), no horizontal overflow; graphs show units, `-2:00`, `-1:00`, `now`, and `TIME` |
| Browser at 2560x1440 | no page scroll (1440/1440) |
| Metric and range switch | Graph 2 moved from tyre wear to ERS store and from 2 minutes to 10 minutes; labels changed to `MJ` and `-10:00 / -5:00 / now` |
| Live Pit wall | Pace count advanced from 2 to 3 eligible laps over 95 seconds of synthetic driving; the chart redrew without reload |
| Console | no page errors or failed requests |

Screenshots: `ui/drive-graphs-1920.png`, `ui/drive-graphs-2560.png`, `ui/pit-pace-live-1920.png`.

## Known limits

- The browser pass used the synthetic 2026 corpus, not a real game stint. Lap points are timestamped when the two-second poll sees them, so a completed lap can sit up to two seconds late on the time axis.
- The Drive lap-time graph shows completed laps only; the in-progress lap is provisional in the Pit wall pace chart.
- A single-point series uses a 2% value-axis pad, so its line is a dot rather than a flat segment.

