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

## Rendered Drive

Screenshots are in docs/rework/evidence/ui/:

- live-1920x1080.png: 1920x1080. Document scrollHeight equals clientHeight (987 == 987), no page scroll.
- live-2560x1440.png: 2560x1440. Typography scales with viewport height; layout still fits.
- live-zoom2x.png: 1280x800 at device scale 2, used as the reflow check.

Values came from the checked-in 2026 golden packets on UDP 20777 against the real binary, not fixtures injected into the DOM.

## Observed and fixed during this check

- Unavailable channels were rendering the word stale inside numeric slots. They now show a dash, with availability stated separately.
- The 1440p view kept 1080p type sizes. The workspace root now scales font size with viewport height.

## Not verified

- 200% text zoom via browser zoom (the 2x device-scale capture approximates it).
- Keyboard traversal, focus return after dialogs, contrast measurement, reduced-motion, and the remaining workspace states (loading, error, replay paused, recording stopped) have no captured evidence.
- Pit wall, History, Analysis, Setups and Settings were not screenshotted.
