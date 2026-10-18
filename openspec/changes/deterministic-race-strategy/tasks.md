# Deterministic race strategy and alerts implementation tasks

Dependencies: durable-race-history, telemetry-analysis-exports. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Estimates and candidate model

- [x] 1.1 Create server/internal/strategy/estimates.go and estimates_test.go with clean-lap filtering, median pace,3-lap regression, negative-slope limitation, fuel margin and10-second energy draw. Verify: rtk go test ./server/internal/strategy -run TestEstimates.
- [x] 1.2 Implement candidates.go with the specified zero/one/two-stop state model, allocation/rules constraints, pit-loss provenance and rejoin uncertainty. Test known hand-calculated race totals, illegal tyres, missing pit loss, neutralization and max16 alternatives. Verify: rtk go test ./server/internal/strategy -run TestCandidates.
- [x] 1.3 Create profiles.go and versioned resources with legacy source IDs and optional null bands; test2025/2026 applicability, core/surface independence and unknown-capacity SOC. Verify: rtk go test ./server/internal/strategy -run TestReferenceProfiles.

## 2. Notifications and outcomes

- [x] 2.1 Create alerts.go/alerts_test.go using a fake monotonic clock. Verify3-second caution,500ms provisional critical, immediate direct failure,5-second clearance,2unit hysteresis,30-second routine gap, grouping, acknowledgment and critical bypass with TestNotificationPolicy.
- [ ] 2.2 Implement immutable outcome recording and strategy/ack APIs; verify branch changes preserve original advice, current plans reset correctly and predicted/observed discrepancies remain visible. Document model limits and profile editing in server/README.md.

## 3. Change gate

- [ ] 3.1 Run rtk go test -race ./server/...; save hand-calculated candidate comparisons, alert timelines and profile provenance in docs/rework/evidence/deterministic-race-strategy.md. Confirm no model latency is on the critical deterministic alert path.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
