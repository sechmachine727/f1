# session-analytics evidence

## Commands

```
go test ./server/internal/analytics -count=1
go test ./server/internal/history -count=1
go test ./server/internal/httpapi -run TestRecordedAnalyticsRoutes -count=1
go test -race ./server/...
go vet ./server/...
npm run test -- --run && npx tsc --noEmit && npm run lint && npm run build
```

All pass. Browser captures are in `docs/rework/evidence/ui/analytics-*.png`.

## Data model

Migration `004_analytics.sql` adds `session_participants`, `lap_analytics`, `lap_traces`, `analytics_stints` and `session_analytics`. The existing `laps`, `stints` (legacy) and recordings tables are untouched.

- `session_participants`: car index, name, team, number, AI flag, first and last observed sequence. Upserted from packet 4; a late joiner is added without losing earlier rows.
- `lap_analytics`: one row per car and lap. Fields and derivations: `lap_ms` from packet 2's last-lap time at the next lap boundary; sectors from packet 11 (`minutes*60000 + ms`, unavailable if a part is missing); `distance_m` from total-distance delta with a lap-distance fallback; `avg_speed_mps = distance / lap time`; `top_speed_mps` from packet 6 samples; `trap_speed_mps` from packet 2 (field is present in the catalog; the demo corpus currently reports 0, so the UI shows it only when non-zero); positions and gaps at the lap boundary from packet 2; compound/age from packet 7; fuel/ERS from packet 7; pit and validity from packet 2; track limits from packet 10 where available; `corner_speeds` in `extra_json`.
- `lap_traces`: decimated distance/speed points, at most 300 per lap, stored with `decimated=1` and method version `lap-analytics-v1`. Charts read this row; MCAP remains the replay source.
- `analytics_stints`: race stints grouped by compound change or pit, and practice/qualifying runs split at pit laps. Best, median and least-squares degradation are stored.
- `session_analytics`: best lap, theoretical best from best sectors, clean and total lap counts, average and top speed, derivation time and method version.

## Behavior verified

- Participants: 24-car fixture; late joiner keeps earlier rows (`TestParticipantsAndTraces`, `TestLapAnalyticsFromLapBoundaryAndHistory`).
- Sectors: minutes-bearing sector combines to milliseconds; a missing part stays unavailable (`TestLapAnalyticsFromLapBoundaryAndHistory`, `TestAnalysisSummaries`).
- Session classification: practice 1-4, Q1-Q3 5-7, race 10-12, time trial 13, sprint 14; qualifying segments group (`TestSessionTypeClassification`).
- Session close: a new logical session closes the previous analytics, and a ten-minute quiet period closes the current one once.
- Stints and runs: race produces stints only; practice and qualifying additionally produce pit-separated runs (`TestSummaryAndCompare` plus the session-type guard).
- Backfill: a pre-analytics recording derives laps once, the recording hash is unchanged, and a repeat run adds no rows (`TestBackfillDerivesWithoutRewritingRecording`).
- Routes: session analytics, lap detail and recorded compare return 200; a one-lap comparison returns 422 (`TestRecordedAnalyticsRoutes`).

## UI verified

- History: session library with type filters, track, laps, best lap and state; qualifying weekend group headers. Screenshot `analytics-history-1920x1080.png`.
- Recorded Pit wall: session header, driver comparison buttons, classification for the full field, lap-time and gap charts, Laps table, Sectors, Speed, Tyres, Fuel/ERS, Stints/runs and provisional Corners tabs. Screenshots `analytics-pit-1920x1080.png`, `analytics-pit-laps.png`, `analytics-pit-corners.png`.
- Analysis: recorded lap picker, compare button, sector deltas, theoretical best, differences, CSV export, replay open/pause/close, selected-lap consistency and a post-race engineer panel using the unchanged `post_race` profile. Screenshot `analytics-analysis-1920x1080.png`.
- Practice and qualifying: captured with the same session reclassified (`analytics-practice-1920x1080.png`, `analytics-qualifying-1920x1080.png`). Practice shows `Runs split by pit visits`; qualifying shows its segment label.
- The recorded views never read the live stream; they use `/sessions/{id}/analytics`, so a recorded selection cannot show current live values. The `Recorded · selection drives history views` copy is removed.

## Limitations

- The demo corpus reports no trap speed, so the UI only shows it when present; real capture is required to exercise it.
- Provisional corner zones come from circuit geometry with a ±50 m window and are labeled provisional, not optimal.
- Qualifying segment progression is presented as grouped segment rows and a segment label; a combined three-segment comparison chart across separate sessions is not implemented.
- The post-race engineer panel is wired to the existing endpoint and shows the missing-provider state on this machine; no real provider was available for a live answer.
- Backfill handles finalized recordings; an active recording is derived live instead.
