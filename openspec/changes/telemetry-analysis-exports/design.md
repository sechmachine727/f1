# Telemetry analysis and exports design

## Context

Consumes history's indexed raw reader/branch resolver and the shared Measurement/ExportRow contracts. Existing frontend Recharts can display the range results; charts do not justify a full normalized shadow recording.

## Goals / Non-Goals

Goals: bounded comparable series and portable exports. Non-goals: invented missing channels, automatic coaching claims, and corner averages from point landmarks.

## Decisions

Create server/internal/analysis with query.go, compare.go, summaries.go and export.go. Queries resolve branch ancestry and raw chunk ranges first, then decode only selected fields. Apply cancellation between chunks and a30-second deadline. Limit concurrent analysis/export jobs to2 and import to1. Derived lap/stint summaries are cached by raw checkpoint, branch and method version; stale caches are rebuilt deterministically.

Use lap distance as the shared comparison axis. Resample with linear interpolation only across neighboring valid samples no more than250ms apart; leave larger gaps as gaps. Preserve flags/pit/invalid markers as overlays, never smooth across them. For chart decimation use first/min/max/last buckets for numeric series, preserving extremes and discontinuities; booleans/enums use change points. Output maximum points per series is a cap, not evidence that omitted samples were constant.

Median last5 eligible laps supplies pace, separate by compound/stint/session conditions. Stint degradation uses at least3 clean laps; record sample count, fit error and method version. Treat fuel-load/track-evolution differences as limitations unless a calibrated correction exists. Top speed is max available speed samples on the selected surviving session; speed-trap best uses packet2 per-car fastest speed/lap, not max sampled top speed. Last clean-lap average uses validated lap distance/time. Corner average is distance/time over an explicit validated zone profile with source and build applicability; no bundled landmark-only profile qualifies.

Export streams the same long-form row schema for all formats. Use typed nullable columns; schema maps seq/UID to UTF8, epoch/session ms to int64, numbers to double and booleans to bool. ZIP includes data.<format> plus manifest.json. CSV uses RFC4180, UTF8, native number formatting and formula escaping for text fields only. Manifest records that policy, raw recording hashes, decoder/method versions and range/conditions/gaps. Write to a new temp artifact, close/checksum, atomically publish download ID. Cancellation deletes only the job-owned incomplete output, never its source; failure retains job error without claiming a usable download.

## Risks / Trade-offs

Interpolation can hide gaps -> hard250ms limit and explicit invalid intervals. Derived estimates mix conditions -> retain context differences and optional filters. Export disk contention -> bounded workers, reserve checks and writer priority. Numeric round-trip -> typed format reader tests.

## Migration Plan

Add endpoints without rewriting existing MCAP data. Summary method_version invalidates only derived cache. Exports are immutable artifacts; a changed schema increments version instead of silently changing old files.
