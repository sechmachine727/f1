# Tasks

## 1. Metric history

- [x] 1.1 Add the metric registry and a bounded 1 Hz ring that stores null gaps and trims only samples older than ten minutes; verify retention, trimming and gap storage in unit tests.
- [x] 1.2 Drive the rings from one 1 Hz tick that samples every supported metric regardless of slot selection; verify a metric switched after several minutes still has history.

## 2. Drive graph component

- [x] 2.1 Build the compact graph component with metric and range selects, three time ticks ending at the newest sample, three value ticks, axis titles, latest value and unit, and line breaks at gaps; verify tick labels and gaps with Testing Library.
- [x] 2.2 Verify range changes only move the window: switching 30 seconds, 10 minutes and back keeps the retained samples and never restarts the series.

## 3. Drive layout

- [x] 3.1 Replace the three fixed Spark slots with configurable slots and keep the lap time, tyre wear and gap ahead defaults; verify slots change independently.
- [x] 3.2 Verify Drive still fits 1920x1080 and 2560x1440 without page scroll with the new headers and axes, and that each axis is readable in a screenshot.

## 4. Live laps source

- [x] 4.1 Add the shared live-laps poll used by Drive and the Pit wall; verify completed valid laps update within five seconds, new laps appear once, and no poll runs for a recorded session.
- [x] 4.2 Feed the Drive lap-time series from that poll with the lap number as the key; verify a completed lap is appended once and a provisional value is not written to the lap-time graph.

## 5. Live Pit wall charts

- [x] 5.1 Feed the live Pit wall pace and stint from the shared laps poll and add the in-progress lap as a provisional pace point; verify the provisional point grows and is replaced without duplicating the lap.
- [x] 5.2 Confirm the live trace stack appends the current lap and refreshes the stored series after each lap increment; verify with a fake sample source that the current lap appears before completion and no gap appears after the lap closes.

## 6. Integration and evidence

- [x] 6.1 Run `npm test`, `npm run lint`, `npx tsc -p tsconfig.app.json --noEmit` and `npm run build`; record the results and the two pre-existing type errors if they remain.
- [x] 6.2 Verify in the browser at 1920x1080 and 2560x1440 while live: default graphs, metric switch with history, 30-second to 10-minute range round trip, frozen feed, and a live Pit wall pace update after a lap; save screenshots and note any limitation.
- [x] 6.3 Update `docs/usage.md` with the graph selectors, axis behaviour and range retention, and record the change evidence.
- [x] 6.4 Re-read the final diff against the specs and resolve material findings before reporting completion.

## Workflow follow-up

- Archive the change after the project's review and qualification gates are satisfied.
- Sync the capability deltas into the canonical specs at archive time.

