# Spec Delta

## Purpose

Give the race engineer durable conversation continuity that stays within provider limits and preserves accepted strategy when older telemetry is compacted.

## ADDED Requirements

### Requirement: Conversations are scoped and durable

The system SHALL maintain one live-strategy transcript per session and branch shared by scheduled, continuous and manual live questions, and separate post-race and setup-research transcripts. It SHALL persist model-visible messages, tool calls and results, provider continuation and usage.

#### Scenario: Live question after a scheduled call
- **WHEN** the driver asks a question after an accepted scheduled strategy call
- **THEN** the question uses the same session transcript and can reference the prior advice

#### Scenario: Post-race analysis
- **WHEN** post-race analysis runs for a session
- **THEN** it uses its own transcript without consuming live-strategy evidence

### Requirement: Context is compacted before overflow

The system SHALL compact when estimated model-visible input reaches the configured percentage of the effective context limit. It SHALL use provider-reported input tokens where available and a deterministic estimate for new content.

#### Scenario: Threshold reached
- **WHEN** the next request would cross the compaction threshold
- **THEN** compaction runs before the normal provider call

#### Scenario: Provider rejects oversized input
- **WHEN** the provider rejects a request as exceeding its context window
- **THEN** the system compacts and retries the logical turn within the bounded recovery policy

### Requirement: Compaction preserves strategy evidence

Compaction SHALL preserve current plan, decision changes, accepted advice, evidence identifiers, assumptions, predictions, measured outcomes, safety, weather and flag changes, and coverage gaps. It SHALL drop old raw telemetry and repeated tool payloads first and SHALL retain a recent verbatim tail.

#### Scenario: Old telemetry removed
- **WHEN** a long race transcript is compacted
- **THEN** the summary retains the accepted strategy and evidence identifiers while detailed old samples are removed

#### Scenario: Recent tail retained
- **WHEN** compaction completes
- **THEN** recent turns remain verbatim up to the configured tail budget

### Requirement: Compaction is auditable

The system SHALL record every compaction trigger, focus instruction, before and after token estimate, summary, model and usage in activity logs and SHALL count the summarization request against budgets.

#### Scenario: Manual compact
- **WHEN** the operator selects Compact now with a focus instruction
- **THEN** the resulting summary and focus appear in the activity timeline

#### Scenario: Continuation boundary
- **WHEN** provider continuation exists before compaction
- **THEN** it is cleared across the summary boundary and the reset is recorded

### Requirement: Accepted strategy is never rewritten

Compaction SHALL NOT modify accepted strategy records or prior decision evidence.

#### Scenario: Compaction after accepted advice
- **WHEN** compaction summarizes a conversation containing accepted advice
- **THEN** the original accepted record remains byte-for-byte unchanged
