# Spec Delta

## Purpose

Let authenticated operators tune race-engineer scheduling, budgets, context and prompts from the web while keeping connection secrets and native deployment settings outside the browser.

## ADDED Requirements

### Requirement: Web Settings edits agent behavior

The web Settings workspace SHALL edit enabled state, trigger rules and cadence, minimum dispatch gap, continuous mode, call, token and cost budgets, deadlines, context and output tokens, steps, tool calls, temperature, top-p and parallel tool calls. It SHALL validate values on the server and save with the current configuration revision.

#### Scenario: Cadence change
- **WHEN** an operator changes the lap trigger cadence and saves
- **THEN** the next scheduled invocation uses the new cadence without restarting the server

#### Scenario: Stale save
- **WHEN** a save uses an outdated configuration revision
- **THEN** the server rejects it with a revision conflict and the UI keeps the draft

### Requirement: Environment-owned values are read-only

The system SHALL display the source of each agent value and SHALL disable or reject edits to values owned by the environment.

#### Scenario: Environment-enabled agent
- **WHEN** the agent enabled state comes from an environment variable
- **THEN** the toggle is disabled and labeled as environment-owned

### Requirement: Connection fields stay outside web editing

The web Settings workspace SHALL show endpoint, model, protocol and API-key presence read-only. It SHALL NOT write endpoint connection fields or secrets.

#### Scenario: Missing API key
- **WHEN** the selected endpoint has no configured key
- **THEN** Settings shows the missing variable name and offers no web secret editor

### Requirement: Prompt revisions are immutable and reviewed

The system SHALL store full effective system prompts per profile as immutable revisions with a current selection or authored default. Saving SHALL first show a side-by-side diff. Selecting an older revision SHALL load it as a pending change. Reset SHALL select the authored default. Deletion SHALL require inline confirmation and SHALL be refused for the current revision.

#### Scenario: Save with changes
- **WHEN** the operator saves an edited prompt
- **THEN** a red/green side-by-side diff is shown before the new revision becomes current

#### Scenario: Delete current revision
- **WHEN** the operator attempts to delete the current revision
- **THEN** the action is refused until another revision or the authored default is selected

### Requirement: Custom prompts remain auditable

The system SHALL record the prompt revision and SHA-256 hash with each invocation, label custom prompts as unqualified, and continue appending the application output-schema instruction and tool set.

#### Scenario: Custom prompt invocation
- **WHEN** the current prompt is a custom revision
- **THEN** the invocation log identifies that revision and hash
