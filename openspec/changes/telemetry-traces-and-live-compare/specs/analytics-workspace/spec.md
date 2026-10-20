# Spec Delta

## ADDED Requirements

### Requirement: Recorded Pit wall includes input traces

The recorded Pit wall SHALL add a telemetry view showing throttle, brake, gear and ERS store energy over session time for the selected cars and laps, with the deployment or overtake band and the shared zoom and pan behaviour.

#### Scenario: Two drivers compared over a stint
- **WHEN** two cars and a lap range are selected
- **THEN** both cars appear on the trace charts in their team colours and the ERS band follows the session's season

#### Scenario: Lap range narrowed
- **WHEN** the operator selects a subset of laps
- **THEN** the charts show only those laps and the 100 percent range fits them

#### Scenario: Session opened from History
- **WHEN** the operator selects a recorded session in History
- **THEN** the recorded Pit wall opens with the telemetry view available for that session

### Requirement: Analysis adds time-aligned traces

Analysis SHALL show the selected laps on a time axis from lap start, in addition to the existing distance-aligned comparison, including throttle, brake, gear and ERS for each selected lap.

#### Scenario: Two laps compared
- **WHEN** two laps of the same car are compared
- **THEN** the time-aligned traces start at zero for each lap and the distance comparison remains available

#### Scenario: Laps of different lengths
- **WHEN** the compared laps differ in duration
- **THEN** each trace ends at its own lap time and the difference is not hidden by stretching

### Requirement: Recorded traces use recorded evidence

Every trace in a recorded Pit wall or Analysis view SHALL come from the selected session's stored samples, and the view SHALL state when samples are still being derived or were never captured.

#### Scenario: Backfill pending
- **WHEN** the selected recording has no samples yet
- **THEN** the view states that samples are being derived and keeps the lap aggregates available

#### Scenario: Live session running
- **WHEN** a recorded session is open while another session is live
- **THEN** no live sample appears in the recorded traces

### Requirement: History identifies the circuit and timing

History rows SHALL show the circuit name when the track identifier maps to a known circuit, the session date, duration and type, and SHALL fall back to the track identifier with an unknown label when no name is known.

#### Scenario: Known circuit
- **WHEN** a recorded race at a mapped track is listed
- **THEN** the row shows the circuit name, date, duration, type and lap summary

#### Scenario: Unknown track
- **WHEN** the track identifier has no circuit name
- **THEN** the row shows the identifier and states the name is unknown without dropping the session

