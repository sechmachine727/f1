# telemetry-analysis-exports evidence

## Commands

    rtk go test ./server/internal/analysis

Pass on 2026-10-18.

## Coverage

- Range query rejects more than 24 cars, 16 channels or 20000 points, and an inverted range, before scanning.
- Series are decimated with first/min/max/last per bucket, preserving extremes; the returned point count stays within the cap.
- Lap comparison aligns two to four laps on lap distance with samples matched by session time within 250 ms. Portions outside the distance range return gap markers instead of an invented trace.
- Export ZIP contains exactly data.<format> and manifest.json for CSV and JSONL.
- CSV reopens with the manifest row count; formula-leading text is prefixed with an apostrophe and numeric signs are preserved.
- Parquet reopens with typed nullable columns and the manifest row count.
- Manifest records format, range, units, decoder version, method versions and secrets_included false.

## Limits

Analysis job cancellation under a real slow disk, two-job contention, and a full-disk refusal are implemented but not covered by an automated test. Exports were exercised on fixture recordings of 240 datagrams.
