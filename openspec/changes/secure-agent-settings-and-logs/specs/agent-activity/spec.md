# Spec Delta

## Purpose

Give operators a durable, searchable and live record of what the race engineer saw, requested, decided and changed during a session.

## ADDED Requirements

### Requirement: Invocation stages are append-only evidence

The system SHALL persist an append-only event for every invocation stage, including queued, context built, compaction started and completed, provider request and response, tool call and result, validation, acceptance, rejection, notification, cancellation and error. Events SHALL NOT be modified or deleted through the API.

#### Scenario: Provider failure
- **WHEN** a provider request fails after context was built
- **THEN** the stored timeline retains the trigger, context, request, failure and final invocation status

#### Scenario: Process interruption
- **WHEN** the server starts and finds an invocation still marked running
- **THEN** it appends an interrupted event and marks the invocation interrupted

### Requirement: Model-visible payloads are auditable

The system SHALL store the exact system prompt snapshot, model-visible transcript, current input, output, tool calls and tool results associated with each invocation. It SHALL omit API keys, authorization headers, opaque provider continuation and hidden reasoning.

#### Scenario: Prompt revision deleted later
- **WHEN** an operator deletes the prompt revision used by an old invocation
- **THEN** the invocation detail still shows the recorded prompt snapshot and its hash

#### Scenario: Oversized tool result
- **WHEN** a tool result exceeds the event payload cap
- **THEN** the event records a truncation marker and artifact reference instead of silently overflowing storage

### Requirement: Logs workspace exposes list and detail views

The system SHALL provide a Logs workspace listing time, session, profile, status, trigger, model, prompt revision, input and output tokens, estimated cost, latency, compaction count and notification outcome, with filters and cursor pagination. Selecting a row SHALL show overview, input, output, tools, context and raw views.

#### Scenario: Recorded race review
- **WHEN** an operator opens Logs for a recorded session
- **THEN** the list shows that session's invocations and compaction events in time order

#### Scenario: Legacy invocation
- **WHEN** an invocation predates activity capture
- **THEN** the detail shows the available output and usage with an explicit input-not-captured state

### Requirement: Live activity streams in real time

The system SHALL stream authenticated activity events over SSE with replay from the last event identifier and periodic heartbeats. Events SHALL be persisted before broadcast.

#### Scenario: Reconnect after missed events
- **WHEN** a client reconnects with its last event identifier
- **THEN** the server replays later stored events before continuing live updates

#### Scenario: Cross-origin stream
- **WHEN** a browser opens the activity stream from another origin
- **THEN** the server rejects it before opening the stream

### Requirement: Pit wall and History link to session activity

The live Pit wall SHALL provide an activity button with a running-count badge and a live drawer. Each History session and recorded Pit wall SHALL provide a link to that session's full activity in Logs.

#### Scenario: Agent running during a race
- **WHEN** an invocation is queued or running during a live session
- **THEN** the Pit wall drawer shows its current stage, elapsed time, recent tools, compaction state and latest decision

#### Scenario: Recorded session
- **WHEN** the operator selects Agent activity from a History row
- **THEN** Logs opens filtered to that session
