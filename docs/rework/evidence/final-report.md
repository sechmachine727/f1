# F1 telemetry redesign final report

Status: implementation incomplete; qualification gates A and E are partial, B is partial, C and D are not passed. This report is evidence, not a completion claim.

## What was built

- Go root module `f1tele` with `server/cmd/f1tele` and packages: protocol, config, state, history, analysis, strategy, engineer, setup, httpapi, web.
- Protocol: generated layouts for all 33 supplied 2025/2026 packet layouts; exact documented sizes; 2025 real-capture parity and 2026 golden corpus; malformed/unknown rejection; event union decoding; channel catalog with wheel order and restrictions.
- State: single serialized reducer, per packet/car ordering, uint32 wrap, same-frame distinct events, flashbacks and branches, restriction as null, freshness, season conflict retention.
- History: SQLite catalog with WAL/foreign keys, durable sequence blocks, MCAP Zstandard chunks with CRC/indexes, rotation, checkpoints, legacy .f1bin import, recovery from damaged files without modifying originals, replay isolation.
- Analysis: bounded range queries with 30s deadline and decimation, distance-aligned comparisons with gaps, CSV/JSONL/Parquet exports plus manifest, formula-injection escaping.
- Strategy: deterministic pace/degradation/pit candidates with explicit missing inputs, provisional thermal/wear profiles, notification arbiter with persistence, acknowledgment and 30s routine gap.
- Engineer: exact prompts/v1 embedded byte-for-byte and hash-verified; tools.json schemas; Eino ADK chat-completions invoker; scheduler/evidence cursor state machine; all 16 supplied scheduler vectors pass.
- Setup: Tavily/SearXNG search, bounded HTTP(S) inspection, provenance and experiment records.
- Frontend: six workspaces, one WebSocket store, Drive layout using circular wear rings, 56px surface/core values, fixed call band, neutral unknown/restricted states.
- Packaging: multi-stage Dockerfile, compose with 8081/tcp and 20777/udp, persistent /data.

## Executed validation

| Command | Result |
|---|---|
| `rtk go test -race ./server/...` | pass |
| `rtk go vet ./server/...` | pass |
| `rtk go build ./server/cmd/f1tele` | pass |
| CGO=0 linux/amd64, linux/arm64, windows/amd64 builds | pass |
| `rtk openspec validate --all --strict --no-interactive` | 9/9 pass |
| `python3 docs/rework/validate_artifacts.py` | PASS |
| `npx tsc --noEmit`; `npm run lint`; `npm run test -- --run`; `npm run build` | pass (lint 0 errors, 9 warnings) |
| Local UDP->MCAP->SQLite->HTTP smoke | pass (200 datagrams) |

## Artifacts

- linux/amd64: 27e5004be2a6b70fbc70879f1357aa54d69645ceb8327660678d756f6056e486
- linux/arm64: 5ce6b453ebf7240b46ba8073149f2858ce2a385bfac6b3e21365479a43c83b5c
- windows/amd64: ea5b8be3de3d448f97d5749bcbe06264e3ea58e2cab27a138f8849027e76c84b

## Requirement-to-evidence mapping (selected)

- Decode both formats: protocol golden/parity tests.
- Availability/null restrictions: state TestRestrictedRivalIsNullNotZero.
- Flashback continuity: state TestSameUIDRewindCreatesBranchKeepsSession; history branch segments test.
- Raw recording: history TestRecordingRoundTrip; recovery test.
- Bounded analysis/exports: analysis validation and export code; not end-to-end exercised in this report.
- Scheduling cursors: engineer scheduler vector suite.
- Prompts: manifest hash verification in engineer.VerifyBundle.
- UI hierarchy: Drive component; rendered screenshots not captured.

## Pending / failed gates

- Gate C: 2-hour soak and latency/memory/drop oracle not implemented or run.
- Gate D: real provider, Windows host, microphone and real 2026 capture unavailable; Responses adapter not wired.
- Gate B: no rendered 1080p/1440p/zoom/keyboard validation; scheduler and import wiring lack full integration tests.
- Windows control center (change 8) absent.

## Rollback

Revert to branch `codex/f1-telemetry-redesign-specs` / commit c95b78c. Existing recordings were not modified or deleted. New runtime uses data/catalog.db and data/recordings under the selected data dir; no downgrade migration was applied.
