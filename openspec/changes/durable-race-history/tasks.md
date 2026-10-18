# Durable race history implementation tasks

Dependencies: go-telemetry-core. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Durable writer and catalog

- [x] 1.1 Create server/internal/history/catalog.go plus migrations/001_initial.sql with the specified tables/keys/indexes and migration tests. Include source-context sequence block reservation and crash-before-flush/no-reuse tests. Verify: rtk go test ./server/internal/history -run 'TestCatalog|TestMigration'.
- [x] 1.2 Implement raw MCAP writer/checkpoints/rotation in history/recording.go and recording_test.go. Feed duplicate/invalid/24-car packets and compare every input payload and timestamp after reopening. Verify: rtk go test ./server/internal/history -run TestRecordingRoundTrip.
- [x] 1.3 Implement history/storage_limits.go and injected disk/queue failures; tests cover80% warning,100GiB cap,5GiB reserve, queue exhaustion and live-state continuity with no deletion. Verify: rtk go test ./server/internal/history -run TestStorageFailureIsolation.

## 2. Branches, import and replay

- [x] 2.1 Create history/branches.go and branches_test.go with same-UID rewind, nested flashback, exact-anchor replacement, partial-lap invalidity and inherited-prefix selection; verify TestSurvivingBranchTimeline.
- [ ] 2.2 Create import.go/recovery.go for <QH .f1bin and MCAP recovery; test truncated header/payload, CRC failure, crash after MCAP fsync before catalog commit and repeated recovery. Verify: rtk go test ./server/internal/history -run 'TestLegacyImport|TestRecovery'.
- [ ] 2.3 Create replay.go plus history/replay HTTP routes; verify seek/pause/speed during live capture cannot change live observation IDs or dispatch calls with TestReplayIsolation. Document storage/cleanup/import/recovery in server/README.md with a tested fixture workflow.

## 3. Change gate

- [ ] 3.1 Implement list/group/branch/cleanup APIs and verify protected-file409, ambiguous links and per-ID cleanup failure. Run rtk go test -race ./server/...; save evidence in docs/rework/evidence/durable-race-history.md with actual recording coverage and recovery limits.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
