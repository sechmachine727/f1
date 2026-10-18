# Go telemetry core

## Purpose

Deliver validated, contextual live telemetry for every available car without UI, storage or model work blocking UDP collection.

## ADDED Requirements

### Requirement: Decode both supplied formats

The system SHALL decode complete supported 2025 and 2026 packets, preserve format-specific events and car counts, and report invalid or unsupported datagrams without terminating intake.

#### Scenario: 2026 maximum field

- **WHEN** a valid2026 packet contains24 car slots and packet16 telemetry
- **THEN** all available slots and fields are decoded with correct signedness, scales and units

#### Scenario: Malformed packet

- **WHEN** a packet is truncated or has an unknown format/version
- **THEN** raw intake retains its bytes and diagnostics count it; live state is not modified and intake continues

### Requirement: Preserve measurement availability

The system SHALL publish nullable values with availability, units, as-of time, basis, source and confidence; restricted zeros and protocol sentinels SHALL remain unavailable.

#### Scenario: Restricted opponent

- **WHEN** a rival has restricted fuel/ERS/wear channels containing zeros
- **THEN** the corresponding values are null/restricted and never imply an empty tank or fresh tyres

#### Scenario: Stale channel

- **WHEN** no new high-rate value arrives for1000ms in live mode
- **THEN** that value becomes null/stale without announcing a recovered fault

### Requirement: Separate context dimensions

The system SHALL distinguish UDP format, software game year, regulation season, mode, online status, source and saved-session link IDs, retaining unknown or conflicting classifications.

#### Scenario: 2026 cars with2025 software year

- **WHEN** formula13 identifies2026 cars but gameYear is25
- **THEN** the UI context records2026 regulation season and2025 game year independently

#### Scenario: Ambiguous career

- **WHEN** zero or conflicting saved-session identifiers are received
- **THEN** history association remains unassigned and the evidence remains inspectable

### Requirement: Order live updates without losing events

The system SHALL prevent duplicate or older data from regressing current channel state while preserving distinct same-frame events and all raw received datagrams.

#### Scenario: Several events in one frame

- **WHEN** two different race-control events share an overall frame ID
- **THEN** both events appear once in live event history

#### Scenario: Late packet type

- **WHEN** a low-rate packet arrives after a newer frame of a different packet type
- **THEN** its fields update if newer for their own packet/car key; unrelated frame ordering does not discard it

### Requirement: Preserve flashback session continuity

The system SHALL preserve session identity and existing alert/audit data on same-UID game-time rewind, while exposing a branch transition for surviving-timeline processing.

#### Scenario: Same UID rewind

- **WHEN** game time decreases while overall frame ID advances
- **THEN** the session is retained and a branch transition is emitted; active logs are not globally cleared

#### Scenario: Duplicate rewind

- **WHEN** an older duplicate datagram has a lower session time
- **THEN** no new branch or repeated condition notification is created

### Requirement: Serve direct local and LAN clients

The system SHALL serve the UI and versioned API on8081 and receive UDP20777 by default, without login, pairing, application tokens, Origin validation/allowlists or mandatory TLS.

#### Scenario: LAN first launch

- **WHEN** a user opens the displayed LAN address from another reachable browser
- **THEN** dashboard/API/stream access works without an application access handshake

#### Scenario: Provider disabled

- **WHEN** no AI endpoint is configured
- **THEN** health, collection, telemetry and deterministic functionality remain usable

### Requirement: Bound browser streaming

The system SHALL provide subscription snapshots/deltas on one socket per browser, preserve delta continuity and isolate slow clients from collection.

#### Scenario: Lost delta

- **WHEN** a client receives a base_seq different from its applied sequence
- **THEN** it requests/resets from a snapshot instead of merging an inconsistent state

#### Scenario: Slow browser

- **WHEN** a client exceeds the bounded send queue
- **THEN** telemetry coalesces or resynchronizes for that client while other clients and intake continue

### Requirement: Keep credentials server-side

The system SHALL keep provider/search/voice credentials out of read APIs, browser state, raw exports and diagnostic logs.

#### Scenario: Save endpoint

- **WHEN** a key is saved through configuration
- **THEN** the response reports only whether a key is configured and never echoes it
