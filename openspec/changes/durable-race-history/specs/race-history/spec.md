# Durable race history

## Purpose

Retain complete received telemetry and distinguish saved sessions, capture fragments and flashback attempts for reliable history, recovery and replay.

## ADDED Requirements

### Requirement: Record received telemetry at incoming rate

The system SHALL retain every received datagram while recording is healthy, independently of live frame rejection, car selection, UI cadence and model configuration.

#### Scenario: Duplicate and restricted data

- **WHEN** a duplicate or restricted rival datagram is received
- **THEN** unchanged bytes are recorded with receive identity even if the live reducer suppresses or nulls its fields

#### Scenario: No session packet yet

- **WHEN** telemetry arrives before identity
- **THEN** an unassigned fragment records it and associates it when context is available

### Requirement: Catalog sessions and optional careers

The system SHALL organize history by regulation season and optional named career/weekend/session using observed link evidence, retaining ambiguous associations as unassigned.

#### Scenario: Resume saved race

- **WHEN** nonzero link identifiers match an existing session in the same source context
- **THEN** a new fragment joins that logical session without overwriting existing data

#### Scenario: Missing context

- **WHEN** season or career association cannot be established
- **THEN** the session is visible in Unassigned with editable naming/grouping

### Requirement: Preserve flashback attempts

The system SHALL retain branch ancestry and abandoned attempts while default analysis follows the surviving prefix plus replacement child data.

#### Scenario: Rewind mid-lap

- **WHEN** a child attempt starts at a flashback anchor
- **THEN** parent samples before the anchor remain inherited and parent samples at/after it are excluded from default analysis

#### Scenario: Compare attempts

- **WHEN** a user selects an abandoned branch
- **THEN** its original data is available and labeled as an alternate attempt

### Requirement: Recover interrupted recordings

The system SHALL recover durable recording/catalog data after interruption without overwriting the original damaged file and SHALL expose uncovered intervals.

#### Scenario: Crash after chunk flush

- **WHEN** MCAP is durable but the SQLite checkpoint is older
- **THEN** restart reindexes durable chunks and reports any incomplete tail

#### Scenario: Corrupt chunk

- **WHEN** a CRC or truncated record is found
- **THEN** valid recoverable content is copied to a new artifact and corruption is reported

### Requirement: Enforce manual storage limits

The system SHALL warn at the configured threshold and visibly stop recording on cap, free-reserve, queue or disk failure while live telemetry continues; cleanup SHALL require explicit user selection.

#### Scenario: Default cap reached

- **WHEN** recording reaches100GiB or free space approaches5GiB
- **THEN** recording stops with reason, the gap is recorded, and existing files remain

#### Scenario: Cleanup active data

- **WHEN** cleanup includes an active recording or job source
- **THEN** the request fails with409 and deletes nothing from that protected source

### Requirement: Import legacy data faithfully

The system SHALL import timestamped .f1bin records without assuming historical captures were full-rate and SHALL preserve original files and incomplete-tail diagnostics.

#### Scenario: Legacy sampled file

- **WHEN** a capture contains <QH timestamp/length records with10Hz high-rate samples
- **THEN** import preserves those timestamps/bytes and labels actual coverage rather than claiming lost frames were recovered

### Requirement: Isolate replay from live operation

The system SHALL support seek, speed and pause for a selected session/branch without replacing live state, changing recording or triggering live agent calls.

#### Scenario: Replay during live race

- **WHEN** a browser seeks a recorded race while a live source records
- **THEN** only that replay playhead changes and the live recording/call schedule continues

#### Scenario: Paused replay

- **WHEN** wall time advances during pause
- **THEN** replay values remain at the playhead with a Paused label rather than false live-stale expiry
