# Scheduled race engineer

## Purpose

Invoke a bounded race engineer from configurable software triggers while preserving unaccepted evidence, prior advice and provider-specific continuation correctly.

## ADDED Requirements

### Requirement: Schedule without default model polling

The system SHALL invoke AI on configured lap/event/time/manual rules, with periodic and continuous modes off by default and typed ANY/ALL filters, cooldowns and budgets.

#### Scenario: Default configured endpoint

- **WHEN** the endpoint is enabled with unchanged defaults
- **THEN** completed laps/significant events can invoke AI; no interval/continuous polling runs

#### Scenario: ALL rule

- **WHEN** a two-lap condition and weather event occur between dispatches
- **THEN** the rule dispatches once after both are satisfied and resets its scheduling latches

### Requirement: Retain evidence through failures

The system SHALL advance scheduling checkpoints on dispatch but advance strategy evidence only after accepting a strategy result, carrying pending events through failed/cancelled/superseded runs.

#### Scenario: Cancelled Safety Car call

- **WHEN** the call carrying a Safety Car event is cancelled
- **THEN** the next strategy input includes that event and previous attempt status

#### Scenario: New event during execution

- **WHEN** a weather event arrives after the frozen observation upper bound
- **THEN** accepting the current result does not consume the later event

### Requirement: Carry useful strategy continuity

The system SHALL include previous accepted output, available rationale, assumptions, predictions, observed outcomes and attempt status, with explicit scope/reset/truncation metadata.

#### Scenario: Old advice disproved

- **WHEN** observed outcomes contradict the prior plan
- **THEN** the next input retains both and requires reassessment

#### Scenario: Branch changes

- **WHEN** a flashback creates a child attempt
- **THEN** old in-flight output is rejected and a new conversation uses the surviving prefix

### Requirement: Support both explicit model protocols

The system SHALL support selected Chat Completions or Responses endpoints with correct tool-result IDs and scoped opaque continuation, without silent protocol fallback.

#### Scenario: Provider switch

- **WHEN** endpoint/model/protocol/conversation changes
- **THEN** opaque state is cleared while portable accepted advice remains available

#### Scenario: No reasoning summary

- **WHEN** a provider returns no rationale summary
- **THEN** the call still succeeds using validated output and observed evidence

### Requirement: Bound continuous conversations

The system SHALL make continuous mode an explicitly enabled logical conversation with configurable cadence, context/time/cost limits and visible pause/stop controls.

#### Scenario: Default continuous session ends

- **WHEN** the enabled default15-minute period elapses
- **THEN** continuous mode stops without automatic restart or an agent-owned polling loop

### Requirement: Validate and audit outputs

The system SHALL reject malformed, stale-scope or unsupported-evidence outputs, record failures and suppress partial responses, using the exact authored prompt bundle.

#### Scenario: Wrong branch response

- **WHEN** a completed response names an abandoned branch
- **THEN** it is marked superseded and does not change strategy or evidence cursor

#### Scenario: Invalid terminal JSON

- **WHEN** one bounded repair still fails schema validation
- **THEN** the invocation ends invalid with evidence pending and no driver call

### Requirement: Separate calls from notifications

The system SHALL persist valid advice independently of whether the notification arbiter speaks it, and deterministic critical information SHALL remain available without a model.

#### Scenario: Unchanged plan

- **WHEN** a valid keep result supplies no useful new call
- **THEN** it is recorded without repeated speech or UI interruption
