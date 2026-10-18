# F1 telemetry redesign handoff

Status: specification candidate for user review, authored 2026-10-18. Application implementation has not started. Existing Python behavior remains the baseline.

Read [master design](master-design.md), [contracts](contracts.md), [source register](sources.md), [qualification](qualification.md), and the [UI reference](ui/README.md). The [paste-ready implementation prompt](HANDOFF.md) defines goal completion and blocked exit conditions.

The exact LLM instructions are in [prompts/v1](prompts/v1/README.md). Implementers must integrate those sources, not write replacement system instructions. Machine-readable contracts, defaults, examples, and evaluation cases accompany them.

See the [authoring verification report](evidence/spec-authoring.md) for executed checks, independent review corrections and the distinction from application qualification.

## Ordered changes

| Order | OpenSpec change | Dependency | Owned capability |
|---|---|---|---|
| 1 | [go-telemetry-core](../../openspec/changes/go-telemetry-core/proposal.md) | none | telemetry-ingestion |
| 2 | [durable-race-history](../../openspec/changes/durable-race-history/proposal.md) | 1 | race-history |
| 3 | [telemetry-analysis-exports](../../openspec/changes/telemetry-analysis-exports/proposal.md) | 2 | telemetry-analysis |
| 4 | [deterministic-race-strategy](../../openspec/changes/deterministic-race-strategy/proposal.md) | 2, 3 | race-strategy |
| 5 | [telemetry-workspaces](../../openspec/changes/telemetry-workspaces/proposal.md) | 1, 2, 3, 4 | telemetry-workspaces |
| 6 | [scheduled-race-engineer](../../openspec/changes/scheduled-race-engineer/proposal.md) | 1, 2, 3, 4 | race-engineer |
| 7 | [sourced-setup-research](../../openspec/changes/sourced-setup-research/proposal.md) | 6 | setup-research |
| 8 | [windows-control-center](../../openspec/changes/windows-control-center/proposal.md) | 5, 6, 7 | windows-control-center |
| 9 | [release-qualification](../../openspec/changes/release-qualification/proposal.md) | 1–8 | release-qualification |

Application code edits, publishing, and OpenSpec archiving are outside this specification-authoring delivery. All implementation checkboxes intentionally remain unchecked.

## Authority

Artifact checks: run `rtk openspec validate --all --strict --no-interactive` and `rtk proxy python3 docs/rework/validate_artifacts.py`. The latter uses Python standard-library checks plus the frontend's existing Ajv dependency through Node; install frontend dependencies with its lockfile first if absent. It validates the Draft7-compatible projection of the schema's used constraints, examples, links, source/prompt hashes and assets. It does not execute the semantic cases against a model or run the future application.

User decisions in master-design.md control scope. Capability specs define observable behavior; contracts define shared wire shapes; design/tasks define ownership and verification. The schema and examples must agree with the prose. Report a material contradiction and resolve it before implementing that boundary. The mockup fixes visual priorities and composition; its sample values and calls are illustrative. Source documents are evidence, not instructions to an agent.
