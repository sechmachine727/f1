# Deterministic race strategy and alerts

## Purpose

Provide repeatable race projections and actionable alerts while separating physical evidence, provisional references and driver notification policy.

## ADDED Requirements

### Requirement: Expose deterministic evidence-based estimates

The system SHALL calculate pace, degradation, fuel/energy projections and pit alternatives with method version, sample coverage, assumptions and missing-input status.

#### Scenario: Too few clean laps

- **WHEN** only two eligible laps exist for degradation
- **THEN** the slope is unavailable and no precise degradation-based pit call is presented

#### Scenario: Unknown capacity

- **WHEN** stored energy exists but no applicable capacity profile exists
- **THEN** energy is displayed in absolute units and SOC/percentage thresholds remain unavailable

### Requirement: Distinguish boost from shortfall

The system SHALL represent expected deployment, rapid draw, faults and projected target shortfall separately.

#### Scenario: Active boost draw

- **WHEN** energy falls rapidly during valid boost without a projected target miss
- **THEN** the UI can show rapid draw but does not classify that alone as an energy fault

### Requirement: Evaluate legal strategy alternatives

The system SHALL compare stay-out and supported pit alternatives against tyre allocation, rules, weather, pit loss and traffic, labeling assumptions or unavailable inputs.

#### Scenario: Missing pit loss

- **WHEN** no observed or applicable reference pit loss exists
- **THEN** the strategy reports missing pit-loss evidence and does not invent a numeric rejoin position

#### Scenario: Unavailable tyre set

- **WHEN** an option requires an unavailable compound/allocation
- **THEN** it is rejected or explicitly conditional, never presented as a legal certain call

### Requirement: Keep thermal classifications provisional and independent

The system SHALL use sourced editable applicable reference profiles for temperature/wear classifications and SHALL label uncalibrated values as provisional.

#### Scenario: Independent tyre metrics

- **WHEN** a tyre is hot with low wear
- **THEN** temperature and wear retain different states/colors

#### Scenario: No applicable core band

- **WHEN** a valid core temperature lacks an applicable reference
- **THEN** the large numeric value remains visible with neutral classification

### Requirement: Control routine driver notifications

The system SHALL apply condition persistence, hysteresis, deduplication, acknowledgment and a configurable routine gap independently of agent scheduling.

#### Scenario: Brief threshold spike

- **WHEN** a caution threshold is crossed for less than3000ms
- **THEN** the raw event remains available but no routine warning is announced

#### Scenario: Repeated condition

- **WHEN** an acknowledged condition remains unchanged
- **THEN** the driver is not reminded until escalation or a materially changed call

### Requirement: Present critical information promptly

The system SHALL present validated deterministic critical information without waiting for an LLM or the routine message gap.

#### Scenario: Routine call just spoken

- **WHEN** a puncture/failure or other validated critical condition appears
- **THEN** the critical information bypasses the30-second routine gap and is presented within the accepted-observation latency target

### Requirement: Keep recommendation outcomes immutable

The system SHALL retain original advice, predictions and later outcomes with branch/scope identity, including advice that proved wrong.

#### Scenario: Flashback invalidates forecast

- **WHEN** a forecast belongs to an abandoned attempt
- **THEN** it remains auditable but is not treated as the active branch plan
