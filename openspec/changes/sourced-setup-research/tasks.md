# Sourced setup research implementation tasks

Dependencies: scheduled-race-engineer. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Search and source inspection

- [ ] 1.1 Create setup/search.go adapters and httptest fixtures for Tavily/SearXNG, timeout429/403, malformed JSON and provider-disabled direct URLs. Verify: rtk go test ./server/internal/setup -run TestSearchProviders.
- [ ] 1.2 Implement fetch.go with HTTP(S)/redirect/body/text limits and inspected/truncated metadata. Test source text containing prompt instructions, an unknown build and fetch failure after a promising snippet. Verify: rtk go test ./server/internal/setup -run TestSourceInspection.

## 2. Recommendations and experiments

- [ ] 2.1 Register the exact tools.json search/fetch tools only for setup_research; add semantic fixtures for2025-vs2026, race-vsTT, unknown menu bounds and locked adjustments. Verify no live strategy request can call search or mutate game/config state.
- [ ] 2.2 Create history migration002 and setup records/experiment APIs; wire Setups/Analysis to real saved records and outcome comparisons. Verify failed experiments append outcomes without rewriting the source hypothesis, and source numeric settings cite inspected content. Document configuration and player-applied workflow.

## 3. Change gate

- [ ] 3.1 Run rtk go test -race ./server/... plus frontend tests/typecheck/build. Execute one real configured provider search and direct source fetch or record the precise unavailable-provider gate. Save redacted provenance and applicability cases in docs/rework/evidence/sourced-setup-research.md; no arbitrary meta/optimal claims.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
