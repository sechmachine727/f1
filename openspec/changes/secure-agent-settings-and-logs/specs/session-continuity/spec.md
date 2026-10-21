# Spec Delta

## Purpose

Keep the live reducer and stream aligned when the game moves between qualifying segments or restarts a session.

## ADDED Requirements

### Requirement: New sessions start with clean live state

The system SHALL start a new logical fragment when the game session UID changes and SHALL clear pause, samples, frame ordering, participants, laps, events and season evidence before reducing packets from the new session.

#### Scenario: Qualifying segment changes
- **WHEN** the game changes from Q1 to Q2 with a new session UID
- **THEN** the next stream state has no inherited pause and accepts Q2 telemetry even when frame numbers restart lower

### Requirement: Same-UID restart starts a new logical session

The system SHALL treat a return to the session-clock origin as a new logical session even when the session UID is unchanged. It SHALL clear the same live state as a UID change.

#### Scenario: Restart the session
- **WHEN** the player restarts a session and the game clock returns near zero with the same UID
- **THEN** the dashboard leaves the paused state and shows the restarted session instead of waiting for a container restart

### Requirement: Refreshed state reaches the dashboard

After a session transition the system SHALL stream the new context and changed metrics without requiring a browser or server restart.

#### Scenario: Live stream after transition
- **WHEN** a client is subscribed before the transition
- **THEN** subsequent deltas carry the new session context and current telemetry
