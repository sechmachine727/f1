# deterministic-race-strategy evidence

## Commands

    rtk go test ./server/internal/strategy -run TestEstimates
    rtk go test ./server/internal/strategy -run TestCandidates
    rtk go test ./server/internal/strategy -run TestReferenceProfiles
    rtk go test ./server/internal/strategy -run TestNotificationPolicy
    rtk go test ./server/internal/engineer -run 'TestRecordOutcomes|TestPreviousAccepted'
    rtk go test -race ./server/...

All pass on 2026-10-18.

## Estimates

- Clean-lap filtering drops invalid, incomplete, pit and neutralized laps.
- Median pace uses the last five eligible laps; the fixture returns 90400 ms.
- Degradation is a within-stint linear fit that refuses to cross a compound change and needs three laps.
- Fuel finish margin comes from the game's reported laps.
- Energy draw needs two store samples inside ten seconds; without a window the estimate is absent rather than invented.
- Unknown capacity leaves energy absolute; the shipped defaults contain no capacity profile.

## Candidates

- Stay-out plus one candidate per supplied window, plus two-stop pairs, capped at 16 alternatives.
- Track-profile pit loss is used when present; absent evidence leaves pit loss unavailable.
- Empty compounds, windows outside the race, and missing pace produce illegal or unavailable results instead of confident numbers.
- A neutralization active in the snapshot adds an assumption and a limitation.
- A prior evaluation is a value; changing the branch does not mutate it.

## Notifications

- Caution needs 3 seconds across the threshold; critical needs 500 ms.
- Clearance needs 5 seconds inside the band, and conditions clear instead of sticking.
- 104 C stays caution (hysteresis), not critical.
- Simultaneous tyre warnings produce one grouped sentence.
- Non-critical announcements respect the 30-second gap; critical bypasses it.
- Acknowledgment silences until escalation.

## Advice and outcomes

- Accepted advice is persisted with its session, branch, predictions and plan in the catalog. A branch change stops it being carried forward, while the original row remains for review.
- `previousAccepted` only reads accepted output from the same session and branch, so a new branch starts with no carried plan and no evidence cursor.
- Measured outcomes compare the stored plan with catalogued laps completed after the advice: planned versus observed pit lap, planned versus observed compound after the stop, and a closed-window statement when no stop occurred. A missing stop is not claimed while the window is still open.
- Outcome rows are keyed by invocation and only move forward in lap coverage, so a later observation cannot be rewritten by an older one. The advice itself is never updated.
- `server/README.md` documents model limits, profile editing, and where advice and outcomes live.

## Limits

Two units of hysteresis are expressed by the profile bands and a single threshold test, not a separate dead-band knob. Outcome rows are internal evidence: the approved HTTP contract has no read endpoint, so the invocation list is the public view of the stored advice.
