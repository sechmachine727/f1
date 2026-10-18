# Scheduled race engineer implementation tasks

Dependencies: go-telemetry-core, durable-race-history, telemetry-analysis-exports, deterministic-race-strategy. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Runtime and providers

- [ ] 1.1 Embed docs/rework/prompts/v1 exact files with a build manifest; create server/internal/engineer/runtime.go and both protocol adapters from installed pinned Eino APIs. Test fake tool loops, fragmented streamed arguments, multiple tool IDs and terminal JSON in providers_test.go. Verify: rtk go test ./server/internal/engineer -run TestProviderContracts.
- [x] 1.2 Implement context.go/validation.go against shared schemas, bounded input, reasoning-summary absence, evidence IDs, message length and stale scope. Turn prompt eval-cases.jsonl into deterministic validation fixtures plus semantic model evaluations; verify TestOutputAcceptance and no prompt edits.

## 2. Scheduler and continuity

- [x] 2.1 Implement scheduler.go using fake monotonic time and persist dispatch/evidence cursors separately. Execute every prompts/v1/scheduler-cases.json vector, including failed/cancelled SC, new event during run, ALL latch, provider change and question non-consumption. Verify: rtk go test -race ./server/internal/engineer -run TestSchedulerVectors.
- [x] 2.2 Implement budget accounting, coalescing, preemption,15s/90s deadlines and bounded continuous mode with visible controls. Verify no timer/model call when both interval and continuous are disabled, no hidden retry and no automatic restart after15minutes with TestInvocationLimits.
- [ ] 2.3 Wire read-only tools, invocation audit/cancel/request APIs and engineer UI status/answer handling. Test cancellation preserves evidence, tool reads cannot exceed frozen scope and notification suppression does not discard accepted advice; document endpoint protocol/config/usage settings.

## 3. Change gate

- [ ] 3.1 Run rtk go test -race ./server/... and the frontend typecheck/build. Qualify a real configured endpoint for each requested protocol with a tool-call/result/final-output round trip and the critical semantic fixture suite; save redacted traces/hashes/usage in docs/rework/evidence/scheduled-race-engineer.md. If credentials/endpoint access are absent, record the external qualification blocker without claiming provider support is verified.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
