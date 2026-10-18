# Telemetry analysis and exports

## Purpose

Make recorded laps and stints comparable and export portable telemetry with explicit units, validity, gaps and provenance for people and other analysis tools.

## ADDED Requirements

### Requirement: Query bounded historical ranges

The system SHALL return named-session/branch/cars/channels over explicit half-open ranges with bounded points, coverage and cancellation.

#### Scenario: Oversized query

- **WHEN** a request exceeds24 cars,16 channels or20000 points per series
- **THEN** the request fails explicitly without starting an unbounded raw scan

#### Scenario: Cancelled query

- **WHEN** the client cancels a range scan
- **THEN** work and file handles are released while recording continues

### Requirement: Compare eligible laps

The system SHALL align two to four selected laps by distance and expose invalidity, missing samples and condition differences without silently treating unequal conditions as equivalent.

#### Scenario: Invalid or wet lap

- **WHEN** a comparison includes a cut lap or different weather/compound
- **THEN** the series retains invalid/gap markers and comparison differences

#### Scenario: Missing distance

- **WHEN** a lap lacks reliable distance samples
- **THEN** alignment is unavailable for that portion instead of inventing a trace

### Requirement: Summarize stints and race trends

The system SHALL provide branch-aware pace, degradation, gap and stint timelines with neutralizations/pit laps identified and estimate methods disclosed.

#### Scenario: Flashback race summary

- **WHEN** the selected surviving branch inherits earlier laps
- **THEN** race/stint counts use the surviving prefix once and exclude abandoned continuation

### Requirement: Define reliable speed metrics

The system SHALL distinguish observed top speed, per-car session speed-trap best and last clean-lap average; corner averages SHALL require validated boundaries.

#### Scenario: Lap average

- **WHEN** a clean completed lap has known track distance and lap time
- **THEN** average is distance/time with lap/source context

#### Scenario: Only corner landmarks

- **WHEN** a track has point landmarks but no validated entry/exit zones
- **THEN** corner average is null/unsupported and no precise corner-speed figure is displayed

### Requirement: Export consistent typed data

The system SHALL export CSV, JSONL and Parquet using ExportRow plus a provenance manifest, preserving unavailable values, branches, conditions, units and gap information.

#### Scenario: Restricted metric export

- **WHEN** a restricted opponent fuel metric is selected
- **THEN** all typed value columns are null and availability is restricted in every format

#### Scenario: CSV text payload

- **WHEN** a driver label begins with a spreadsheet formula character
- **THEN** CSV escapes that text, records its escaping policy, and preserves numeric signs

### Requirement: Preserve originals and export failures

The system SHALL run exports as cancellable bounded jobs that never overwrite recordings and visibly report storage/serialization failures.

#### Scenario: Low space

- **WHEN** a requested export would breach the free reserve
- **THEN** the job refuses or stops with a clear error and existing history remains intact

### Requirement: Review advice against outcomes

The system SHALL associate recommendations, assumptions and predicted horizons with branch-aware observed outcomes for post-race review.

#### Scenario: Incorrect prediction

- **WHEN** a predicted pit rejoin differs from the observed result
- **THEN** review retains both values and error/context instead of rewriting the original advice
