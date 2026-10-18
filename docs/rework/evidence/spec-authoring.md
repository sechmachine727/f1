# Specification-authoring verification

Date: 2026-10-18. Baseline application commit: 1e099b0 on codex/reimplement-agent-harness. This report covers the documentation/contract/prompt/mockup delivery only. Application code, Go runtime, Windows app and release artifacts remain unimplemented.

## Delivered

- Nine OpenSpec changes, each with proposal, capability delta, design and tasks;55 implementation tasks remain unchecked.
- Master design, exact defaults,79 shared contract definitions and a method/path operation catalog.
- Complete assistant-authored base system instructions plus five profiles, exact read-only tool schemas, prompt hashes,19 semantic reference cases and16 scheduler vectors.
- Standalone approved Drive/Pit wall HTML,1080p/1440p Drive screenshots,1080p Pit wall screenshot, editable two-page draw.io layout and its first-page PNG export.
- Unchanged copies and SHA-256 verification for all five supplied UDP documents; source register and qualification limits.
- A paste-ready goal prompt with suggested skills, ordered implementation, completion gate and bounded external-blocker exit.

## Executed checks

| Command / action | Result |
|---|---|
| rtk openspec validate --all --strict --no-interactive |9 changes passed,0 failed |
| rtk proxy python3 docs/rework/validate_artifacts.py |Passed; schema examples, scope/evidence references, hashes, local links, screenshot dimensions, drawing and fixed-workload arithmetic |
| rtk proxy node docs/rework/validate_contracts.cjs |79 definitions compiled using existing Ajv;74 fixture assertions;19 semantic reference cases, no model calls |
| Standalone mockup browser interaction |Drive -> Pit wall -> Drive verified by visible view and pressed state |
| Mockup viewport inspection |Drive content fits1920x1080 and2560x1440; saved screenshots inspected |
| draw.io native export and XML parse |Editable file parsed; first page exported successfully |

The Python artifact checker invokes the Node checker; these are complementary document checks, not two independent application test suites. It accepts valid checked/unchecked task syntax so it remains usable during later implementation; it does not certify task completion.

## Independent review and corrections

A separate read-only review identified11 concrete contract/exit risks. Corrections were made before final validation:

| Finding | Correction |
|---|---|
| Unspecified HTTP bodies |Added route-specific definitions and operation catalog |
| Missing config revision/patch |ConfigView/ConfigPatch and explicit If-Match behavior |
| Ambiguous export archive/range |One selected data file plus manifest; inclusive receive sequences, separate time-query units |
| Zero links/source identity |Zero maps to null; nonzero schema; persisted automatic source_context_id |
| Branch/session evidence reset |New-branch null accepted/evidence state, reset latches and surviving-prefix re-evaluation; explicit new-session reset |
| Missing scheduler failure vectors |Added failed/timeout/invalid/superseded/budget/manual/session/continuation cases |
| Request/token/throttle ambiguity |Provider-request counters, total-token reservation, latest limiter wins, visible continuous budget pause |
| Inadequate tool payloads |Typed series/candidates/source inspection, frozen strategy_evaluation and candidate binding |
| Expired provider continuation |Clear opaque state for next invocation without same-attempt retry |
| Self-selected soak load |Checked-in per-ID workload,2,141,822payload bytes/s and14,745,600healthy datagrams over2hours |
| Failed check mislabeled blocked |Available-hardware failures require fixes; blocked exit is limited to named unavailable external inputs |

Parent review also added explicit Time Trial behavior, same-frame event preservation, unavailable calibration handling, and source/recording sequence distinction. The reference HTML's original in-memory handlers were restored for standalone use; all sample values remain illustrative.

## Remaining qualification

No application test/build, real provider call, native Windows voice run or live2026 qualification was performed for this documentation delivery. Those are explicit implementation gates in qualification.md. Provisional thermal profiles remain uncalibrated; unknown core/cold/2026/capacity/corner references remain neutral or unavailable. Passing these artifact checks does not qualify a release.

Existing untracked .agents/ and OpenSpec setup files were preserved. Only the new redesign documents/assets and nine change directories belong to this delivery. No publishing or application implementation is authorized by completing this report.

Whitespace review excludes the two byte-for-byte official structure copies, whose original CRLF/trailing whitespace is preserved to match their published source hashes. Authored text is checked separately with git diff --cached --check and explicit path exclusions for those two source files.
