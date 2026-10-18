# Driver and pit-wall workspaces implementation tasks

Dependencies: go-telemetry-core, durable-race-history, telemetry-analysis-exports, deterministic-race-strategy. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Connection and navigation

- [ ] 1.1 Create race_engineer_hub/src/store/telemetryStore.ts and store tests for one socket, stable snapshots, subscription replace, sequence mismatch, stale values and isolated replay; migrate the seven hooks to selectors. Verify: rtk npm run test -- --run from race_engineer_hub.
- [x] 1.2 Replace Index.tsx shell with the six specified routes and persistent source context; test history-to-pit-wall selection,2025/2026/Unassigned tabs and AI-disabled state. Verify tsc and route/store tests without mock nominal values.

## 2. Approved views and analysis

- [x] 2.1 Implement Drive components/styles from docs/rework/ui references, including circular wear, >=56px temperatures, independent semantic states, MJ fallback and stable call region. Add fixtures for all colors, unknown/restricted data and long text. Verify rendered screenshots at1920x1080 and2560x1440 with no scroll at100%scale.
- [x] 2.2 Implement Drivers/Teams comparison, selected car, stint/pace/gap/pit panels and historical banners. Verify24cars, a restricted rival, missing team data and replay-paused views in the real browser.
- [x] 2.3 Implement History/Analysis/Setups/Settings workflows over real APIs, cancellable chart requests, export job feedback and explicit manual cleanup. Test selection preservation after failure and source/applicability labels; include unit/integration tests in each patch.

## 3. Accessibility and change gate

- [ ] 3.1 Exercise keyboard selection/comparison/export and inspect focus, contrast, reduced motion,200%zoom/reflow, populated/empty/loading/stale/error states. Save screenshots and a concise inspection record in docs/rework/evidence/telemetry-workspaces.md.
- [x] 3.2 Run rtk npm run test -- --run, rtk proxy npx tsc --noEmit, rtk npm run lint and rtk npm run build in race_engineer_hub. Separate proven baseline lint failures from regressions; verify exactly one live WebSocket in the browser and do not claim visual acceptance from unit tests alone.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
