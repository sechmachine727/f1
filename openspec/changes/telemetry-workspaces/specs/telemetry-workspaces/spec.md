# Driver and pit-wall workspaces

## Purpose

Help a driver read crucial race information at a glance and review the same evidence in detailed pit-wall, history, analysis and setup workflows.

## ADDED Requirements

### Requirement: Match approved focused hierarchy

The system SHALL implement the saved approved mockup hierarchy with stable positions, circular wear percentages, very large surface/core values and a concise current-call region.

#### Scenario: Supported landscape

- **WHEN** Drive is opened at1920x1080 or2560x1440 at100%scale
- **THEN** its crucial data fits without page scrolling, clipped text or shifting alert regions

#### Scenario: Long driver/call text

- **WHEN** a long name or call arrives
- **THEN** the call region remains stable and full detail is accessible in the engineer view

### Requirement: Use meaningful independent colors

The system SHALL pair semantic colors with text/symbols and classify temperature, wear, deployment and shortfall independently.

#### Scenario: Color cannot be perceived

- **WHEN** the screen is read without distinguishing red/green
- **THEN** state words, values and arrows still communicate the condition

#### Scenario: Unknown thermal band

- **WHEN** a temperature has no applicable profile
- **THEN** the value is visible neutrally with its provisional/unknown context rather than a green optimum claim

### Requirement: Show real connection and recording states

The system SHALL distinguish waiting, partial, live, stale, restricted, replay, paused and recording-failure states without fabricated nominal values.

#### Scenario: No telemetry

- **WHEN** the dashboard starts before the game sends data
- **THEN** the screen shows waiting plus game UDP setup instructions and no meaningless zero metrics

#### Scenario: Recording stopped

- **WHEN** disk protection stops recording while telemetry continues
- **THEN** a persistent recording-stopped reason appears without labeling the live stream disconnected

### Requirement: Provide driver and team comparisons

The system SHALL provide Drivers/Teams tables, selected-driver detail, stint timelines, pace/gap plots and pit alternatives with explicit unavailable rival channels.

#### Scenario: Restricted rival selection

- **WHEN** the selected rival withholds detailed channels
- **THEN** standings remain useful while restricted values are labeled unavailable

#### Scenario: Team totals

- **WHEN** one team car lacks a channel
- **THEN** the aggregation reports coverage rather than summing a missing value as zero

### Requirement: Separate seasons and historical context

The system SHALL provide2025/2026 history tabs and an unassigned view, optional career/weekend groups, and persistent selected recorded/replay context across analysis views.

#### Scenario: Historical navigation

- **WHEN** a user opens a recorded session and switches to Pit wall
- **THEN** the session/season and Recorded banner remain visible; live source selection is unchanged

### Requirement: Provide actionable review and exports

The system SHALL support selected lap comparisons, stint review, sourced setup experiments and selected/session exports with coverage and failure feedback.

#### Scenario: Export failure

- **WHEN** an export job fails from low space
- **THEN** the UI shows the concrete failure and a retry path without a false download link

### Requirement: Maintain accessible operation

The system SHALL support keyboard navigation, visible focus, WCAG2.2AA contrast, reduced motion and reflow/text zoom without hiding crucial information.

#### Scenario: Text zoom

- **WHEN** the user increases text to200%
- **THEN** a readable detailed/reflow view is available without overlapping or clipped controls

#### Scenario: Keyboard workflow

- **WHEN** a user selects history, compares laps and exports using keyboard
- **THEN** focus follows logical order and returns correctly after dialogs

### Requirement: Use one shared live connection

The system SHALL reuse one subscribed browser connection across telemetry panels and recover coherently after disconnect or sequence mismatch.

#### Scenario: Several panels open

- **WHEN** Drive, standings and engineer components subscribe
- **THEN** one socket serves them and a reconnect replaces the source snapshot atomically
