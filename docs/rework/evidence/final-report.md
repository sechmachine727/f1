# F1 telemetry redesign final report

Status: implementation substantially complete; qualification incomplete. Gates A and E are partly met, B is partial, C is running, D is externally blocked.

Read this with docs/rework/evidence/implementation-status.md, which carries the current step.

## What was built

Go module f1tele:

- server/internal/protocol: generated layouts for all 33 supplied 2025/2026 packet layouts, exact declared sizes, 2025 real-capture parity, 2026 golden corpus, event union decoding, channel catalog with wheel order and restriction rules.
- server/internal/config: documented precedence, validation, redacted reads, atomic failed PATCH, secrets file at 0600.
- server/internal/state: one serialized reducer, per-key ordering, uint32 wrap, duplicate suppression with same-frame distinct events, flashback branches, null restrictions, freshness, season conflict retention, published game-mode mapping.
- server/internal/history: SQLite catalog (WAL, foreign keys), durable sequence blocks, MCAP Zstandard/CRC chunks with rotation and checkpoints, legacy .f1bin import, damaged-file recovery, replay isolation, storage limits.
- server/internal/analysis: bounded range queries with a 30-second deadline, min/max decimation, distance-aligned comparison with gaps, CSV/JSONL/Parquet export plus manifest.
- server/internal/strategy: pace, per-stint degradation, fuel margin, ten-second energy draw, stay-out/one-stop/two-stop candidates with pit-loss provenance, provisional reference profiles, notification arbiter with persistence, clearing, grouping, acknowledgment and the 30-second gap.
- server/internal/engineer: exact prompts/v1 bundle with hash verification, tools.json schemas, Eino ADK chat-completions invoker, scheduler vectors, output acceptance, rolling budget.
- server/internal/setup: Tavily and SearXNG search, bounded source inspection, provenance, setup records and experiments.
- server/internal/httpapi: the v1 routes, one-socket streaming with snapshot/delta/resync, job and download handling, cleanup protection.
- server/internal/web: embedded dashboard with a compilable placeholder for fresh checkouts.
- server/qualification: rated 2048 dk/s generator with independent sender, receiver-intake and MCAP oracles, RSS and latency sampling.

Frontend (race_engineer_hub): six workspaces, one stream store with a tested reducer, Drive composition with circular wear rings, 56px-equivalent temperatures, independent semantic states, MJ fallback and a fixed call band.

Windows (windows/): control-center core with supervisor identity/adoption, flush-then-stop, force-stop confirmation, voice turn cap and WAV validation; WinUI app, tray controller, native capture and packaging script.

Packaging: multi-stage Dockerfile (distroless, nonroot, writable /data), compose on 8081/tcp and 20777/udp, CI workflow for Go, frontend, artifacts and Windows.

## Validation executed

| Command | Result |
|---|---|
| go test -race ./server/... | pass |
| go vet ./server/... | pass |
| go build ./server/cmd/f1tele | pass |
| CGO=0 linux/amd64, linux/arm64, windows/amd64 | pass |
| fresh clone go build ./server/... | pass |
| rtk openspec validate --all --strict --no-interactive | 9/9 pass |
| python3 docs/rework/validate_artifacts.py | PASS |
| npm run test -- --run | 8/8 pass |
| npx tsc --noEmit, npm run lint, npm run build | pass, 0 errors |
| dotnet test windows/F1Telemetry.ControlCenter.Tests | 9/9 pass |
| docker build | pass, 48.3 MB |
| container health, UDP, restart persistence | pass |
| 30 s rated smoke (2048 dk/s, 3 clients) | exact count and digest match, 0 drops, p95 1.1 ms |
| 10-minute rated pre-check | 1,228,800/1,228,800/1,228,800 sent/received/recorded, identical digests, 0 drops, peak RSS 67.6 MiB, p95 0.53 ms |
| rendered Drive at 1920x1080 and 2560x1440 | no page scroll; screenshots in evidence/ui |

## Requirement mapping

- Decode both formats: protocol golden and parity tests.
- Availability and restrictions: state tests; stale renders as a state word, never a number.
- Context dimensions: state mode/season tests, published game-mode table.
- Flashback continuity: state rewind test, history branch segments, scheduler branch-reset vector.
- Raw recording and recovery: history round-trip, import, recovery tests.
- Bounded analysis and exports: analysis tests including Parquet reopen and cancellation isolation.
- Deterministic estimates: strategy estimate and candidate tests.
- Notifications: notification policy test.
- Scheduling: all 16 supplied vectors.
- Prompts: byte-identical bundle with manifest verification.
- UI: store tests and rendered screenshots.
- Windows core: nine unit tests.

## Artifacts

| Target | SHA-256 |
|---|---|
| linux/amd64 | 27e5004be2a6b70fbc70879f1357aa54d69645ceb8327660678d756f6056e486 |
| linux/arm64 | 5ce6b453ebf7240b46ba8073149f2858ce2a385bfac6b3e21365479a43c83b5c |
| windows/amd64 (cross-compiled) | ea5b8be3de3d448f97d5749bcbe06264e3ea58e2cab27a138f8849027e76c84b |
| Docker image f1tele:local | 991158b3ada7 |

The Windows binary is a compile check produced on macOS, not Windows native qualification.

## Pending and externally blocked

- Gate C: the clean two-hour soak started at 16:11 and ends at 18:11. Numbers land in docs/rework/evidence/release-qualification/rated-workload.json.
- Gate D: no configured model provider or key, no Windows 11 host with microphone, no real 2026 capture. The Responses adapter is not wired; it fails explicitly rather than falling back.
- Gate B: keyboard traversal, focus return, contrast measurement, reduced motion, 200% browser zoom, and the remaining workspace states have no captured evidence. Pit wall, History, Analysis, Setups and Settings were not screenshotted.
- Windows: WinUI build, portable ZIP, clean-machine launch, tray, voice and push-to-talk under game focus are not verified.
- Docker arm64 and amd64 image builds on native runners are not verified locally.

## Rollback

The previous runtime remains at branch codex/f1-telemetry-redesign-specs (commit c95b78c). No recording was rewritten or deleted. New data lives in catalog.db and recordings/ under the chosen data directory; a downgrade does not migrate them.
