# Telemetry analysis and exports implementation tasks

Dependencies: durable-race-history. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Queries and comparison

- [x] 1.1 Create server/internal/analysis/query.go and query_test.go over a recorded fixture with gaps and nested branches; enforce limits/cancellation and min/max decimation. Verify: rtk go test ./server/internal/analysis -run TestRangeQuery.
- [x] 1.2 Create compare.go and compare_test.go covering two-to-four laps, distance alignment,250ms gap boundary, invalid/pit/neutralization markers and unequal conditions. Verify: rtk go test ./server/internal/analysis -run TestLapComparison.
- [ ] 1.3 Implement summaries.go plus speed/outcome tests; verify last clean-lap distance/time, trap-best attribution, unsupported corner zones, stale cache invalidation and original advice preservation with TestAnalysisSummaries.

## 2. Portable exports

- [x] 2.1 Create export.go and export_test.go to emit ZIP data+manifest for CSV/JSONL/Parquet. Reopen all formats and compare numeric/null/bool/text/branch/source fields; test formula-leading labels and negative numbers. Verify: rtk go test ./server/internal/analysis -run TestExportRoundTrip.
- [x] 2.2 Wire analysis/export/jobs/download handlers and document copyable export/review examples in server/README.md. Inject cancellation, timeout, full disk and two-job contention; verify TestAnalysisJobIsolation leaves recorder and originals intact.

## 3. Change gate

- [x] 3.1 Run rtk go test -race ./server/... and validate exported fixtures against docs/rework/contracts/v1.schema.json. Save byte sizes, coverage and format round-trip evidence in docs/rework/evidence/telemetry-analysis-exports.md.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
