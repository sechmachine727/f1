# Deterministic race strategy and alerts design

## Context

Depends on history/analysis. Existing warning constants are repository heuristics, not game-calibrated optima. Model calls must consume calculations without inventing missing numbers.

## Goals / Non-Goals

Goals: transparent deterministic candidate comparison and one notification arbiter. Non-goals: reproducing a proprietary F1 simulator, claiming global optimality, or treating a reference threshold as a direct failure flag.

## Decisions

Implement server/internal/strategy/{estimates,candidates,alerts,profiles,outcomes}.go. Use Measurement/Estimate and explicit input snapshot scope. Pace = median last5 clean eligible laps (minimum3 for a stable prediction; fewer produces low-confidence observed pace only). Degradation = ordinary least squares lap_ms against tyre_age_laps within one actual compound/stint using at least3 eligible laps; retain intercept, slope, sample count and residual standard error. Do not clamp negative slope silently: classify a negative fit as track/fuel/learning confounding and use zero degradation for a conditional candidate with that limitation.

Candidate lap time = compound baseline + nonnegative fitted degradation*predicted tyre age; baseline comes from comparable clean laps or an explicitly sourced reference. Sum lap times plus pit losses for zero/one/two remaining stops over integer lap boundaries. Use dynamic programming over (lap,compound,age,stops,used_compounds,remaining_sets), max2 additional stops, then return the best16 distinct legal alternatives with stay-out included. Unknown future weather is a scenario assumption, not a probability. Race rules/allocation constraints use observed applicable context; missing rules/set data makes affected options conditional. The returned total is an estimated scenario time with propagated fit/pit-loss error, not guaranteed finishing position.

Pit loss uses a validated track profile or observed matched fractional-lap pit/non-pit elapsed intervals. A pit-lane timer alone is not total race-time loss. Rejoin compares predicted elapsed race-time gaps against nearby rivals with confidence/traffic density; missing opponent pace yields a range or unavailable position. Clean laps exclude invalid, pit, incomplete and neutralized periods. Fuel projection prefers valid game finish margin; an independent consumption model needs3 clean laps and clear assumptions. Energy slope uses10seconds valid samples and reports deployment mode; shortfall requires a supplied energy target/finish plan. Harvest allowance never supplies capacity.

Use a typed source/profile resource catalog, not hardcoded UI thresholds. Bundle legacy surface >103/>108C and wear>=75/>=90 consumed percentages for applicable2025 modern dry context, explicitly uncalibrated. Keep core/cold/optimal and2026 bands unset without a source; users can edit/create a profile and explicitly accept borrowed applicability. Brake/engine/damage heuristics retain separate legacy provenance and no claim of verified damage onset. Direct game failure flags outrank heuristic thresholds. Profile changes re-evaluate state and record version; no model writes profiles.

The arbiter groups related per-tyre warnings into one call; caution requires3s, reference critical500ms, direct game failure events immediate, clearance5s and temperature2C/percentage2point hysteresis. Routine message gap30s; recovery stays in the log unless the driver requests it or a meaningful plan changes. Acknowledge silences unchanged severity; escalation and new critical evidence bypass. Model urgency cannot elevate itself without matching deterministic evidence. One condition ID spans persistence/ack/recovery; the raw event log is never a speech queue.

## Risks / Trade-offs

Simple model error -> publish residual/error bands and measure predictions. Uncalibrated thermal references -> visible labels, narrow applicability and editable profiles. Alert flooding -> fake-clock transition tests and grouped notifications. Missing opponent channels -> explicit nulls/conditional options.

## Migration Plan

Replace legacy alert decisions at the shared strategy boundary, not individual UI panels. Preserve old audit logs for reference; do not reinterpret them as calibrated current events. Derive new outcomes with method versions.
