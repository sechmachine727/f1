# Telemetry analysis and exports

## Why

Recorded races need comparable lap/stint views and portable data. Explicit validity, conditions and availability prevent misleading comparisons or LLM analysis of absent values.

## What Changes

- Add bounded indexed range queries, distance-aligned lap comparisons, stint summaries and recommendation outcome joins.
- Export selected/session data to CSV, JSONL and Parquet with one shared typed schema and provenance manifest.
- Preserve gaps, invalid laps, neutralizations and reference applicability through charts and exports.

## Capabilities

### New Capabilities

- `telemetry-analysis`: Telemetry analysis and exports with observable behavior and acceptance scenarios defined in the capability delta.

### Modified Capabilities

None. No canonical capability exists yet; do not archive this proposal before implementation and qualification.

## Impact

server/internal/analysis; analysis/export HTTP routes; SQLite summary indexes; Arrow/Parquet mapping; frontend analysis consumers.

Dependencies: `durable-race-history`. Shared authority: [master design](../../../docs/rework/master-design.md), [contracts](../../../docs/rework/contracts.md), [qualification](../../../docs/rework/qualification.md).

This artifact is a specification candidate. Application tasks remain unimplemented; authoring these files does not complete their checkboxes.
