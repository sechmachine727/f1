# Sourced setup research

## Purpose

Find and explain applicable setup evidence and preserve player-applied experiments without operating the game or presenting popularity as proof of optimality.

## ADDED Requirements

### Requirement: Search configured sources

The system SHALL support Tavily, self-hosted SearXNG and user-supplied HTTP(S) source URLs with explicit configuration and bounded requests.

#### Scenario: SearXNG JSON disabled

- **WHEN** the chosen instance rejects format=json
- **THEN** research reports the configuration error instead of treating HTML as successful results

#### Scenario: No search provider

- **WHEN** a user supplies a source URL with search disabled
- **THEN** direct bounded source inspection remains possible

### Requirement: Inspect and retain source provenance

The system SHALL retain title, URL, retrieval time, build/regulation/track/conditions/assists/use-case and inspection/truncation state for setup evidence.

#### Scenario: Search lead only

- **WHEN** a search returns a numeric snippet but full fetch fails
- **THEN** the source remains uninspected and no verified full numeric setup is claimed

### Requirement: Match setup applicability

The system SHALL distinguish car generation, game build, race/Time-Trial purpose and conditions, labeling mismatches and unknowns.

#### Scenario: Fast2025 hotlap source

- **WHEN** the player requests a2026 race setup
- **THEN** that source is marked mismatched and never silently recommended as the current race optimum

### Requirement: Explain handling tradeoffs and experiments

The system SHALL pair proposed settings with expected handling effect, tradeoff, source evidence and a comparable one-or-two-variable experiment.

#### Scenario: Rear instability

- **WHEN** the player asks for more rear traction
- **THEN** the recommendation explains stability/responsiveness tradeoffs and a measured test rather than guaranteeing lap-time gain

### Requirement: Respect adjustment context

The system SHALL distinguish recommendations from in-game application and disclose locked/unavailable adjustments when that context is known.

#### Scenario: Parc ferme

- **WHEN** a recommendation includes a currently locked parameter
- **THEN** the UI identifies the restriction and does not imply immediate in-race application

#### Scenario: Apply request

- **WHEN** the user asks the engineer to apply a setup
- **THEN** the application presents player-applied recommendations and performs no game-control action

### Requirement: Persist setup review history

The system SHALL save recommendations, source versions and experiment outcomes without overwriting the original hypothesis.

#### Scenario: Experiment fails

- **WHEN** a setup test worsens stint pace or tyre behavior
- **THEN** the recorded outcome preserves the recommendation and failed result for review
