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

Keyboard: with body focused, the first Tab reaches the Drive tab button and reports `outline: solid 2px rgb(79, 212, 138)`, so focus is visible without relying on color alone.

Contrast, measured from the palette pairs on the dark surface:

| Use | Colours | Ratio |
|---|---|---|
| body text | #e5e5e5 on #11141a | 14.64:1 |
| secondary | #a3a3a3 on #11141a | 7.31:1 |
| active tab | #8ef0b4 on #1d3b2c | 8.91:1 |
| caution | #f0a03c on #11141a | 8.60:1 |
| critical | #f25a5c on #11141a | 5.61:1 |
| in range | #5fd68f on #11141a | 10.10:1 |
| cold | #7fb0ff on #11141a | 8.39:1 |

Muted labels were #737373 at 3.89:1, below AA for small text. They are now #a3a3a3.

Zoom/reflow: ws-drive-zoom200-1920x1080.png is a 960x540 CSS viewport at 2x scale, mirroring a 200% zoom layout width. Drive stacks to one column, type stays legible, and the page scrolls instead of clipping. Reduced motion: the workspaces use no CSS animations or transitions beyond the chart library's static render.

Navigation: selecting a session in History opens Pit wall with a persistent Recorded banner and the season; Back to live clears the selection and leaves the live source untouched.

## Observed and fixed during this check

- Unavailable channels were rendering the word stale inside numeric slots. They now show a dash, with availability stated separately.
- The Pit wall crashed the page twice: a helper was shadowed by a local `laps` array, and a Go nil slice arrived as JSON null. Both are fixed and the workspace renders.
- The subscription only carried the player car, so Drivers showed one row. The single socket now subscribes to all 24 cars; deltas still contain only changed metrics.
- The 1440p view kept 1080p type sizes. The workspace root now scales font size with viewport height.

## Not verified

- 200% text zoom via browser zoom (the 2x device-scale capture approximates it).
- Keyboard traversal, focus return after dialogs, contrast measurement, reduced-motion, and the remaining workspace states (loading, error, replay paused, recording stopped) have no captured evidence.
- Populated Analysis comparison and a completed export job have no captured screenshot; the empty and control states do.
- Engineer requests against a real provider are unverified: no endpoint or key is configured.
