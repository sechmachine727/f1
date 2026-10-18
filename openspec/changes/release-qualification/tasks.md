# Packaging and release qualification implementation tasks

Dependencies: go-telemetry-core, durable-race-history, telemetry-analysis-exports, deterministic-race-strategy, telemetry-workspaces, scheduled-race-engineer, sourced-setup-research, windows-control-center. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Packaging and operator behavior

- [x] 1.1 Update Dockerfile, docker/entrypoint.sh, docker-compose.yml and frontend embedding build for the native server/v1 routes, preserving /data and configurable endpoints. Verify local container health, UDP receipt, browser connection and recording persistence after restart; document exact commands in README.md.
- [x] 1.2 Update .github/workflows/ci.yml and docker-publish.yml to test Go/frontend, build nativeamd64/arm64 images and build the Windows ZIP without publishing during local qualification. Verify workflow syntax and artifact version/prompt/schema consistency; retain baseline rollback identifiers in operator docs.

## 2. Finite system qualification

- [x] 2.1 Implement server/qualification tests/generator and the digest/count oracle specified in docs/rework/qualification.md, including24car data and failure phases. Verify a short30second smoke before the full soak; no recorder sampling or fake pass counters.
- [ ] 2.2 Run the exact2hour/3client rated soak and latency/memory gates once after component tests pass. Save redacted logs, counts/digests, raw latency samples and resource series in docs/rework/evidence/release-qualification/. A failed gate gets a focused fix/retest, not a blanket relaxation.
- [ ] 2.3 Collect real2026 capture parity, real Chat/Responses tool round trips/semantic evals, and Windows clean-launch/tray/flush/PTT evidence. Record an unavailable input as an external blocker, not a pass or an unchecked silent omission.

## 3. Release exit gate

- [ ] 3.1 Run strict OpenSpec/document validation, all affected automated suites/builds and the skeptical final diff review. Produce docs/rework/evidence/final-report.md with requirement-to-evidence mapping, artifacts/hashes, failures/pending gates and rollback. Stop when the handoff goal completion or blocked gate is satisfied; do not publish, archive or loop on cosmetic refinements.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
