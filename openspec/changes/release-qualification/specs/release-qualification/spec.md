# Packaging and release qualification

## Purpose

Release the replacement only with reproducible platform, recording, provider, usability and live-game evidence and a finite completion or blocked exit.

## ADDED Requirements

### Requirement: Package supported targets

The system SHALL provide Linuxamd64/arm64 Docker images and a self-contained Windows11 x64 ZIP with the same versioned UI/contracts/prompts and persistent data paths.

#### Scenario: Fresh installation

- **WHEN** a supported artifact starts without existing configuration
- **THEN** the documented ports/defaults/data directory and AI-disabled functionality work

### Requirement: Demonstrate rated recording workload

The system SHALL qualify a declared two-hour24-car workload with three browser clients, bounded memory, no application-induced recording drops and reported source/OS loss counters.

#### Scenario: Full race soak

- **WHEN** the qualification generator sends the declared2048datagrams/second workload for two hours
- **THEN** input count/hash matches raw recordings, memory remains within the gate, clients stay responsive and gaps/drop counters are reported

### Requirement: Meet local presentation latency

The system SHALL achieve local p95 telemetry presentation below250ms and critical presentation within500ms after an accepted observation under the declared workload.

#### Scenario: Critical event during load

- **WHEN** a deterministic critical event arrives during the rated soak
- **THEN** its rendered state meets500ms without waiting for an LLM or routine throttle

### Requirement: Qualify actual provider and platform paths

The system SHALL distinguish fake-model/compilation checks from real provider tool round trips, native Windows launch/voice and live2026 game evidence.

#### Scenario: Unavailable provider or Windows host

- **WHEN** the required endpoint/host is unavailable
- **THEN** qualification is explicitly pending/blocked with the exact missing input; no synthetic pass is substituted

### Requirement: Preserve rollback and existing history

The system SHALL document and verify the runtime/API cutover and rollback without overwriting recorded history or deleting legacy import support.

#### Scenario: Cutover fails

- **WHEN** the replacement cannot pass smoke tests
- **THEN** the baseline artifact can be restored while new recordings remain preserved for the compatible reader

### Requirement: Stop at a concrete gate

The implementation workflow SHALL stop after required evidence passes or a concrete external blocker prevents remaining work, without open-ended polishing or repeated unchanged checks.

#### Scenario: All checks pass

- **WHEN** all required tasks/gates have recorded passing evidence
- **THEN** the agent records the final report and stops without inventing more work

#### Scenario: External gate remains unavailable

- **WHEN** independent work is complete and the same missing external input persists
- **THEN** the agent records the blocker/next action and ends according to the bounded goal-blocking rules
