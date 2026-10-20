# Spec Delta

## Purpose

Give players a complete review workspace for recorded races, qualifying and practice, with dense charts, cross-driver comparison and engineer help grounded in the same recorded evidence.

## ADDED Requirements

### Requirement: History is a session library

History SHALL list recorded sessions with type, track, date, duration, completed laps, best lap, recording state and completeness. It SHALL filter by race, qualifying and practice and SHALL open the selected session in a recorded analytics view.

#### Scenario: Race and practice in one list
- **WHEN** the catalog contains a race and a practice session
- **THEN** each row states its type and the filter can show either one alone

#### Scenario: Empty season
- **WHEN** no session exists for a filter
- **THEN** History states what is missing and how to start a session

### Requirement: Recorded sessions never show live stream values

The system SHALL source every value in a recorded session view from that session's catalogue. It SHALL NOT show live metrics, live status words or the `Recorded · selection drives history views` copy.

#### Scenario: Historical session opened during a live race
- **WHEN** the user opens a recorded session while telemetry is live
- **THEN** the view shows the recorded session's drivers and laps, not the current live field

#### Scenario: Recorded header
- **WHEN** a recorded session is open
- **THEN** a persistent header identifies the session, type and date and offers a return to live

### Requirement: Recorded Pit wall is a dense analytics workspace

The recorded Pit wall SHALL show a session summary, a driver selector, and selectable views for lap times, sector times, speed traces, tyre wear and temperature, fuel and ERS, position and gap evolution, stint structure and corner performance. It SHALL allow two to four drivers on shared charts.

#### Scenario: Race review
- **WHEN** a race session is open
- **THEN** the Pit wall shows the classification, a lap-time trend, gap evolution and tyre/stint charts for the selected drivers

#### Scenario: Selected rival
- **WHEN** the user selects another driver
- **THEN** the charts replace their series with that driver and keep the chosen metric and lap range

#### Scenario: Teams view
- **WHEN** the user selects Teams
- **THEN** each team row states the contributing cars and the metrics that are unavailable

### Requirement: Analysis compares laps and channels with context

Analysis SHALL let the user choose a session, two to four laps and one or more recorded channels. It SHALL show distance-aligned traces, sector deltas, corner deltas, validity and condition differences, and a theoretical best from best sectors.

#### Scenario: Two laps of one driver
- **WHEN** the user selects two clean laps
- **THEN** Analysis shows the aligned trace, per-sector deltas and theoretical best

#### Scenario: Laps with different conditions
- **WHEN** the selected laps differ in compound or wet/dry conditions
- **THEN** the difference is stated without preventing the comparison

#### Scenario: Gap in a trace
- **WHEN** a distance interval has no samples within tolerance
- **THEN** the chart shows a gap rather than interpolating across it

### Requirement: Analysis can ask the post-race engineer

Analysis SHALL offer an engineer panel that sends a question with the selected session, branch and laps to the existing post-race profile, and SHALL show the answer, evidence identifiers and citation list. It SHALL not modify the authored prompt bundle.

#### Scenario: Configured provider
- **WHEN** the user asks why one lap is faster
- **THEN** the answer references the selected laps and evidence and is stored as an invocation

#### Scenario: No provider
- **WHEN** no model endpoint is configured
- **THEN** the panel states that live questions are unavailable and disables sending

#### Scenario: Invalid answer
- **WHEN** the model returns an output outside the agent schema
- **THEN** the panel shows the validation error and does not present the text as advice

### Requirement: Practice sessions expose run and consistency analysis

Practice SHALL show best and theoretical best laps, clean-lap consistency, run structure, tyre and fuel usage and per-corner performance. It SHALL offer setup recommendations grounded in inspected sources and recorded evidence, labeled as recommendations.

#### Scenario: Practice run review
- **WHEN** a practice session is open
- **THEN** runs are separated by pit visits and the best lap of each run is visible

#### Scenario: Setup suggestion
- **WHEN** the user asks for a setup direction
- **THEN** the answer cites the observed limitation and inspected sources, and states that no game control is performed

### Requirement: Qualifying sessions expose segment progression

Qualifying SHALL show Q1, Q2 and Q3 segments grouped from the weekend link, each segment's best lap and sector bests, the progression between segments, tyre usage and the margin to the next driver.

#### Scenario: Three qualifying segments
- **WHEN** Q1, Q2 and Q3 share a weekend link
- **THEN** the workspace shows one qualifying view with three segment tabs and the driver's progression

#### Scenario: Single segment
- **WHEN** only one qualifying segment was recorded
- **THEN** the workspace shows that segment without inventing the missing ones

### Requirement: Charts remain usable with a full field

The workspace SHALL render a 24-car session and a full race of laps without page-level horizontal overflow, and SHALL bound chart points and retained series. Loading, empty, error and stale-data states SHALL be explicit.

#### Scenario: Full race opened
- **WHEN** a two-hour, 24-car race is opened
- **THEN** the session summary and selected-driver charts render from stored analytics within the interaction budget

#### Scenario: Bounded chart points
- **WHEN** a lap trace exceeds the point cap
- **THEN** the chart uses the decimated series and labels the series as such
