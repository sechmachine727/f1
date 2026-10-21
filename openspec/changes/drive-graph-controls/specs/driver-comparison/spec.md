# Spec Delta

## ADDED Requirements

### Requirement: Live Pit wall charts follow the active session

While telemetry is live, the Pit wall SHALL refresh the active branch's catalogued laps at least every five seconds and update its pace, stint and lap-based charts without a reload.

#### Scenario: Lap completes
- **WHEN** the player crosses the line and the lap is catalogued
- **THEN** the pace and stint charts include it within five seconds

#### Scenario: Recorded session open
- **WHEN** a recorded session is open
- **THEN** the recorded laps remain the only source and no live lap is added

### Requirement: Pace shows the in-progress lap

The live pace chart SHALL show the current lap as a provisional point from the current lap time, updating as the lap progresses, and SHALL replace it with the catalogued lap time when the lap completes.

#### Scenario: Current lap grows
- **WHEN** the player is partway through a lap
- **THEN** the provisional point grows with the current lap time

#### Scenario: Lap catalogued
- **WHEN** the lap completes and appears in the catalogue
- **THEN** the provisional point is replaced by the final lap time without duplicating the lap

### Requirement: Live traces append the current lap

The live trace stack SHALL plot the current lap's samples before the lap completes and SHALL refresh the completed-lap series after each lap increment.

#### Scenario: Mid-lap trace
- **WHEN** the player has driven part of a lap
- **THEN** throttle, brake, gear and ERS already show that part of the lap

#### Scenario: Lap closes
- **WHEN** the current lap becomes a completed lap
- **THEN** the trace continues from the stored lap series without a gap or duplicated time

