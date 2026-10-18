# telemetry-workspaces evidence

Date: 2026-10-18.

## Commands

    cd race_engineer_hub
    npm run test -- --run        # 8 passed
    npx tsc --noEmit             # pass
    npm run lint                 # 0 errors, 8 pre-existing fast-refresh warnings
    npm run build                # pass

## Stream store

src/test/live.test.ts covers:

- snapshot replaces the metric set;
- delta merges by metric id and keeps untouched metrics;
- explicit removed_metric_ids delete entries;
- a base_seq mismatch returns resync instead of merging stale state;
- resync_required never mutates the store;
- stale values render as a dash plus a state word, and zero remains a real value;
- connect() constructs exactly one socket across repeated subscription calls.

## Rendered workspaces

Screenshots are in docs/rework/evidence/ui/:

- ws-drive-1920x1080.png and ws-drive-2560x1440.png: Drive. At 1920 the document scrollHeight equals clientHeight (987 == 987). Typography scales with viewport height at 1440p.
- ws-pit-1920x1080.png and ws-pit-2560x1440.png: Pit wall with a synthetic 24-car field, Drivers and Teams views, selected driver, stint strip and pit alternatives.
- ws-history-1920x1080.png: season tabs including Unassigned, recorded sessions with lap count and recording state.
- ws-analysis-1920x1080.png: session picker, lap list, compare and export controls.
- ws-setups-1920x1080.png and ws-settings-1920x1080.png: empty setups state and redacted configuration view.
- live-zoom2x.png: 1280x800 at device scale 2, used as the reflow check.

Pit wall includes an engineer card with a text request box, recent invocation status and alert acknowledgment, so live questions work without the native app.

Analysis includes a replay panel: open, play/pause, speed 0.25-8, seek, close. Playback advances on the server and is polled every 500 ms. Replay state lives in its own reducer; with a replay open, live health still moved from awaiting_telemetry to live for a new session, and the recording stayed byte-identical.

Values came from server/cmd/f1demo, which sends the qualification corpus on UDP 20777 to the real binary. Nothing was injected into the DOM.

Keyboard: with body focused, Tab reaches Drive, Pit wall, History, Analysis, Setups and Settings in order, each reporting `outline: solid 2px rgb(79, 212, 138)`. Enter on History opens the workspace; a session button is reachable by Tab and Enter opens Pit wall with the Recorded banner. In the cleanup confirmation, the Cancel button receives focus on open and focus returns to Cleanup after Cancel.

Contrast, measured from the palette pairs on the dark surface:

| Use | Colours | Ratio |
|---|---|---|
| body text | #e5e5e5 on #11141a | 14.64:1 |
| secondary | #a3a3a3 on #11141a | 7.31:1 |
| inactive nav | #d4d4d4 on #0d1015 | 12.86:1 |
| active tab | #4fd48a on #1d3b2c | 6.49:1 |
| caution | #f0a03c on #11141a | 8.60:1 |
| critical | #f25a5c on #11141a | 5.61:1 |
| in range | #5fd68f on #11141a | 10.10:1 |
| cold | #7fb0ff on #11141a | 8.39:1 |

Muted labels were #737373 at 3.89:1, below AA for small text. They are now #a3a3a3.

Zoom/reflow: gateb-*-zoom200.png files are 960x540 CSS viewports at 2x scale, mirroring a 200% zoom layout width. Drive stacks to one column, type stays legible, and the page scrolls instead of clipping. Reduced motion: with `prefers-reduced-motion: reduce`, zero elements report an animation or transition.

Navigation: selecting a session in History opens Pit wall with a persistent Recorded banner and the season; Back to live clears the selection and leaves the live source untouched.

## Observed and fixed during this check

- Unavailable channels were rendering the word stale inside numeric slots. They now show a dash, with availability stated separately.
- The Pit wall crashed the page twice: a helper was shadowed by a local `laps` array, and a Go nil slice arrived as JSON null. Both are fixed and the workspace renders.
- The subscription only carried the player car, so Drivers showed one row. The single socket now subscribes to all 24 cars; deltas still contain only changed metrics.
- The 1440p view kept 1080p type sizes. The workspace root now scales font size with viewport height.

## States captured

- gateb-drive-loading.png: disconnected, waiting for telemetry, unavailable values.
- gateb-drive-stale.png: Stale header, no-update values and neutral state words.
- gateb-drive-recording-stopped.png: Live with recording state Stopped.
- gateb-history-error.png: Cannot reach the server.
- gateb-pit-ai-disabled.png: race engineer Not configured, with no request form.
- gateb-analysis-populated.png, gateb-analysis-export.png and gateb-analysis-replay-paused.png: two-lap comparison, succeeded export with Download, and a paused replay.
- All six workspaces at 1920x1080 and 2560x1440, plus 200% reflow, with no horizontal overflow. Drive has no page scroll; Pit wall scrolls for its 24-car field.

## Drive information density

The first implementation left large empty areas and showed a hardcoded restricted rival gap. The finished layout uses the space for data a driver checks mid-lap:

- Position, gap ahead, gap behind (from the car behind's own delta), gap to the leader, and lap count.
- Fuel margin as a number plus bars for tank fill and margin against remaining laps; ERS store as a number plus a bar against the session peak, with deployment mode and harvest limit.
- Speed, gear, DRS state, current and last lap time, engine temperature and RPM.
- Per-tyre pressure, brake temperature, blisters and wear alongside the large surface/core temperatures and wear rings.
- Car issues: front and rear wing, floor, diffuser, sidepod, gearbox, engine, DRS and ERS fault levels with severity words and bars, plus pit stops, penalties and warnings, and per-corner tyre and brake damage.
- Field map: every car at its lap distance on three collision-free lanes, coloured by team with a three-letter driver code above each dot and a team legend.
- Three analytic traces: completed valid lap times from the catalog, worst-corner tyre wear over the stint, and live gap to the car ahead. Running lap time and ERS level were replaced because they restate the numbers already shown.

New protocol channels back this: `timing.gap_leader_s`, `timing.pit_stops`, `timing.penalties_s`, `timing.warnings`, `session.total_laps`, per-corner `brake.*.damage_pct`, and car damage/fault channels from packet 10. `TestGapLeaderAndDamageChannels` checks the derived leader gap, pit/penalty counts, damage values, wheel mapping, and that opponent damage is restricted rather than zero. The channel catalog golden file was regenerated deliberately.

The map labels use driver-name abbreviations, which for the synthetic corpus are numbers (`01`, `02`); with real driver names they read as three letters (for example `VER`).

## Not verified

- Headless Chromium does not drive a native select popup, so the Analysis session picker was exercised by pointer; its surrounding controls are ordinary buttons and the History-to-Pit selection was exercised by keyboard.
- Engineer requests against a real provider are unverified: no endpoint or key is configured.
- Team colours map the game's published team IDs; a custom or unknown team falls back to a neutral dot and team legend entry rather than a wrong livery.
