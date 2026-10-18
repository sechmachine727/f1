# Driver and pit-wall workspaces design

## Context

Reuse existing React18/Vite/Tailwind/Radix/shadcn/Recharts, circuit assets and accessible primitives. Read docs/rework/ui/approved-workspaces.html and screenshots before changing layout. The HTML is an approved composition reference with static sample data, not production components.

## Goals / Non-Goals

Goals: sub-second scanning, explicit state and coherent review workflows. Non-goals: a generic card dashboard, a new UI framework, or pretending mockup figures are telemetry.

## Decisions

Create src/store/telemetryStore.ts with one connection manager, normalized metric map by source/car/channel, stable snapshot identity and useSyncExternalStore selectors. Existing hooks become selectors over the shared store or are removed when their panels migrate; never keep their own socket loops. On disconnect retain source identity but null stale values; full snapshot replaces state atomically. Keep selected live car and historical session/playhead as separate navigation state.

Use routes /drive, /pit-wall, /history, /analysis, /setups, /settings. Drive is the default landing screen. Top context bar shows regulation season, mode, lap/position, live/replay and recording status. Fixed current-call area precedes four stable tyre columns with >=56px temperature numerals, circular consumed-wear percentages, medium >=28px energy/speed and compact context/chart regions at1080p. Scale composition at1440p, not by adding empty cards. Header/control focus targets remain usable. No-scroll applies to the two reference viewports at100%;200%zoom activates readable detailed reflow.

Implement shared semantic tokens for blue/green/orange/red/neutral and state words/arrows. Verify actual contrast on dark surfaces; use color for meaning, not only borders. Keep compound color and semantic state distinct. Source/provisional labels are readable. Render absoluteMJ when capacity is unknown; unsupported corner average gets a concise unavailable explanation. Track map uses known circuit geometry; a schematic must be labeled. No fake default track or placeholder metrics.

Pit wall is a full-width accessible comparison table with Drivers/Teams toggle, selected-car panel, stint strip, gap/pace plots and alternative strategies.24rows need no virtualization dependency. History uses season tabs, optional career/weekend groups and compact session rows with recording completeness. Analysis offers2–4lap selectors, distance-synchronized plots, gap/validity/conditions overlays and cancellation. Setups shows source applicability, parameter experiments and player-applied wording. Settings groups Connection, Recording, Engineer, Voice and Advanced rules with simple summaries first. AI/search/voice disabled states state exactly which endpoint is missing; do not add onboarding security steps.

Use server job IDs for imports/exports/recovery; preserve selected range on errors. Cleanup is deliberate file/session selection with displayed sizes and a confirmation of deletion, unrelated to pairing/access. Retain operator control feedback for recording start/stop. All data-heavy chart requests abort on changed selection; empty/error/loading states share stable layout.

## Risks / Trade-offs

Stable layout versus text scaling -> explicit detailed reflow at zoom. WebSocket render storms -> selectors and requested cadence, measure before extra memoization. Reference's illustrative core bands/SOC -> contract-driven neutral/MJ substitution in real data.

## Migration Plan

Migrate panels and remove their socket constructors in one frontend transport patch. Preserve reusable track/primitive behavior. Backend engineer/setup capabilities are integrated by their owning changes; the final UI must contain no fake outputs. Packaging switches only after all routes are verified.
