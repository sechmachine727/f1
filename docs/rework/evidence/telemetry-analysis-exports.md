# telemetry-analysis-exports evidence

## Commands

    rtk go test ./server/internal/analysis
    rtk go test ./server/internal/analysis -run TestAnalysisSummaries

Pass on 2026-10-18.

## Coverage

- Range query rejects more than 24 cars, 16 channels or 20000 points, and an inverted range, before scanning.
- Series are decimated with first/min/max/last per bucket, preserving extremes; the returned point count stays within the cap.
- Lap comparison aligns two to four laps on lap distance with samples matched by session time within 250 ms. Portions outside the distance range return gap markers instead of an invented trace.
- Export ZIP contains exactly data.<format> and manifest.json for CSV and JSONL.
- CSV reopens with the manifest row count; formula-leading text is prefixed with an apostrophe and numeric signs are preserved.
- Parquet reopens with typed nullable columns and the manifest row count.
- Manifest records format, range, units, decoder version, method versions and secrets_included false.
- Summaries: observed top speed is the max `car.speed_mps` sample; trap best is packet 2's per-car fastest trap speed with its lap; the last clean-lap average is recorded distance over lap time; corner average is explicitly unsupported because no validated entry/exit zone profile ships.
- Summaries are cached by session, branch, car and the raw lap checkpoint. Inserting a lap changes the checkpoint, and the next call rebuilds instead of serving the stale result. TestAnalysisSummaries covers all four values and the invalidation.
- Original advice preservation is verified separately by TestPreviousAcceptedIsBranchScoped and the strategy branch-change test: accepted advice stays in the catalog, while a new branch does not inherit it.

## Limits

Analysis job cancellation under a real slow disk, two-job contention, and a full-disk refusal are implemented but not covered by an automated test. Exports were exercised on fixture recordings of 240 datagrams. Corner averages stay unsupported until a sourced, validated zone profile exists.
