# Spec Delta

## ADDED Requirements

### Requirement: Completed laps keep input and energy samples

The archive SHALL store, for each completed lap and car, a bounded series of samples carrying session time, throttle, brake, gear, ERS store energy, and the deployment or overtake state in effect. Samples SHALL be derived from the recorded datagrams and SHALL NOT alter or replace the raw recording.

#### Scenario: Lap completed
- **WHEN** a car crosses the line and its lap is catalogued
- **THEN** one sample series is stored for that lap and car

#### Scenario: Raw file unchanged
- **WHEN** samples are derived for an existing recording
- **THEN** the recording file bytes and digest are unchanged

### Requirement: Sample resolution is bounded per lap

A stored lap series SHALL contain at most 900 samples. Decimation SHALL retain the first and last sample of the lap and SHALL keep changes in gear and brake state rather than dropping them.

#### Scenario: Long lap
- **WHEN** a lap has more raw samples than the bound
- **THEN** the stored series is decimated to the bound and both ends are present

#### Scenario: Short lap
- **WHEN** a lap has fewer raw samples than the bound
- **THEN** every raw sample is stored without interpolation

### Requirement: Restricted channels are stored as unavailable

For a car whose participant telemetry setting restricts other cars, the archive SHALL record ERS energy and deployment as unavailable unless the car is the player. It SHALL NOT store zero as a substitute. Throttle, brake and gear SHALL remain stored because the sender broadcasts them.

#### Scenario: Restricted rival
- **WHEN** a rival restricts telemetry during a session
- **THEN** that rival's stored lap series marks ERS and deployment unavailable and keeps throttle, brake and gear

#### Scenario: Player car
- **WHEN** the player's own car is recorded in the same session
- **THEN** its ERS and deployment samples are stored as available

### Requirement: Sample timestamps follow the session clock

Samples SHALL use the session clock reported in the packet header and SHALL be ordered within a lap. A session-time rewind SHALL not reorder or append samples to a lap that has already been catalogued.

#### Scenario: Flashback during a lap
- **WHEN** the session clock rewinds before the lap completes
- **THEN** the stored series remains ordered and the completed lap reflects the timeline that produced it

### Requirement: Completed laps are readable while recording continues

The archive SHALL expose stored samples for completed laps of an active session without waiting for the session to end.

#### Scenario: Mid-race request
- **WHEN** the operator requests samples during a live race
- **THEN** completed laps are returned and the in-progress lap is reported as not yet available

### Requirement: Session telemetry read route is bounded

The system SHALL expose a read-only route returning stored samples for a session, branch, a bounded set of cars and an optional lap range. It SHALL reject requests above the car bound, return not found for an unknown session, and distinguish a session with no samples from a failed read.

#### Scenario: Two cars requested
- **WHEN** the route is called for two cars of a catalogued session
- **THEN** the response names the session, branch, cars and per-lap series with availability

#### Scenario: Too many cars
- **WHEN** the request names more cars than the supported bound
- **THEN** the route rejects it with a stated limit and returns no partial series

### Requirement: Backfill derives samples for existing recordings

Backfill SHALL derive samples for recordings that predate this capability, SHALL resume from stored progress after interruption, SHALL be idempotent for an unchanged recording, and SHALL leave the original files in place.

#### Scenario: Interrupted backfill
- **WHEN** backfill stops partway through a session
- **THEN** rerunning it completes the session without duplicating stored samples

#### Scenario: Second run
- **WHEN** backfill runs again on a fully derived recording
- **THEN** stored samples and hashes are unchanged


