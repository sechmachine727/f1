# Spec Delta

## Purpose

Preserve every recorded session as queryable, per-driver analytics so race, qualifying and practice data can be reviewed and compared long after the car leaves the track.

## ADDED Requirements

### Requirement: Sessions are closed and classified by type

The system SHALL close a session when the game starts a new logical session or after a configurable inactivity window, and SHALL classify it as race, qualifying, practice, sprint, time trial or unknown from the game session type. It SHALL record start and end times, track, regulation season, mode family and completeness.

#### Scenario: Race ends
- **WHEN** the game stops sending after a race
- **THEN** the session closes with an end time and remains available in History

#### Scenario: Qualifying segments share a weekend
- **WHEN** Q1, Q2 and Q3 arrive as separate sessions with the same weekend link
- **THEN** History groups them under one qualifying event with separate segment labels

### Requirement: Every participating car is catalogued

The system SHALL record each car's index, driver name, team, race number and AI/player status for the session, including cars that join or leave mid-session.

#### Scenario: Full grid
- **WHEN** a 24-car session is catalogued
- **THEN** all 24 participants appear in the session detail with their team and number

#### Scenario: Late joiner
- **WHEN** a participant appears after the session started
- **THEN** the roster records its first observed sequence rather than omitting it

### Requirement: Each completed lap has a durable analytics record

The system SHALL store one record per car and lap containing lap time, sector 1/2/3 times, lap distance, average speed, top speed, trap speed and lap, start and end position, gaps to the car ahead and leader, compound and tyre age, fuel and ERS at lap boundaries, pit-in and pit-out flags, validity and track-limit flags.

#### Scenario: Completed race lap
- **WHEN** a valid lap finishes
- **THEN** its record exposes all three sector times and the derived average speed

#### Scenario: Incomplete lap
- **WHEN** a session ends mid-lap
- **THEN** the partial lap is marked incomplete and is excluded from clean-pace calculations

### Requirement: Sector times are reconstructed from packet fields

The system SHALL combine the game's minute and millisecond sector parts into one duration per sector and SHALL leave a sector unavailable when any part is missing or non-finite.

#### Scenario: Sector with minutes part
- **WHEN** a sector reports 1 minute and 2345 milliseconds
- **THEN** the stored sector time is 62245 milliseconds

#### Scenario: Missing sector part
- **WHEN** only the millisecond part is present
- **THEN** that sector is stored as unavailable and is not treated as a zero

### Requirement: Corner performance uses validated zones or is labeled provisional

The system SHALL compute per-corner minimum and average speed from a validated track profile's entry and exit distances. When only circuit geometry is available it SHALL derive provisional zones, label them provisional, and SHALL NOT present an optimum or calibrated claim.

#### Scenario: Validated corner profile
- **WHEN** a track has a validated corner entry and exit
- **THEN** the corner table shows the measured minimum and average speed for each lap

#### Scenario: No usable geometry
- **WHEN** a track has no validated profile and no usable outline
- **THEN** corner speed is unavailable rather than estimated from an arbitrary window

### Requirement: Stint and run aggregates are stored

The system SHALL group consecutive laps on the same compound into stints and store lap count, start and end lap, best and median lap, degradation, average wear and fuel/ERS change. In practice and qualifying it SHALL also group laps into runs separated by a pit visit.

#### Scenario: Race stint
- **WHEN** a car completes six laps on one compound between pit stops
- **THEN** one stint record covers those six laps and reports its median and degradation

#### Scenario: Practice run
- **WHEN** a practice lap follows a pit visit
- **THEN** a new run starts at that lap

### Requirement: Decimated traces support charts without rescanning raw files

The system SHALL store a bounded per-lap trace for speed and key channels, and per-session series for position, gap and tyre/fuel evolution. The stored trace SHALL preserve first, minimum, maximum and last values per distance bucket and SHALL be labeled as decimated.

#### Scenario: Speed trace for a lap
- **WHEN** a lap chart is requested
- **THEN** it returns at most the configured point cap from the stored trace without reading the MCAP file

#### Scenario: Raw recording removed
- **WHEN** a recording is deleted while its derived records remain
- **THEN** charts still render and replay reports the raw recording as unavailable

### Requirement: Existing recordings are backfilled without rewriting them

The system SHALL derive analytics for already catalogued recordings through an idempotent backfill, preserving the original files byte-for-byte and skipping records that already exist.

#### Scenario: Backfill after upgrade
- **WHEN** the catalog contains a recording created before analytics existed
- **THEN** a backfill adds its lap and participant records once and leaves the file hash unchanged

#### Scenario: Repeated backfill
- **WHEN** the backfill runs again
- **THEN** it creates no duplicate lap or stint rows
