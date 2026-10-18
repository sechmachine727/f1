# Packaging and release qualification design

## Context

All earlier changes must have implementation evidence before this release gate. Existing Docker publishes native amd64/arm64 images; preserve native runners and do not substitute QEMU. The scope authorizes local artifacts, not pushing a release or changing external systems.

## Goals / Non-Goals

Goals: finite reproducible qualification, supported artifacts and a practical rollback. Non-goals: accepting synthetic checks as live-game proof, publishing without authorization or chasing unrelated cleanup.

## Decisions

Rewrite Dockerfile as Node frontend build + Go CGO=0 build + small runtime image. Copy embedded UI and exact prompt manifest, expose8081tcp/20777udp, mount /data and support config/endpoint secrets through documented inputs. Replace the Python static-server entrypoint and legacy8765 exposure coherently with compose/frontend/docs. Preserve healthcheck and native amd64/arm64 workflow builds; update smoke scripts/CI for Go, frontend and Windows builds. Keep image package/runtime versions locked in implementation and record digests. No automatic updater or service installer.

The canonical gate matrix, fixture seeds, exact commands and limits are in docs/rework/qualification.md. Implement server/qualification integration tests plus a deterministic UDP generator/record-count oracle. The soak runs separately from ordinary unit tests and writes evidence artifacts, not a huge committed capture. Duration2hours,2048datagrams/second using the fixed docs/rework/contracts/workload.json schedule and a24-slot2026 corpus, three browser clients (Drive60Hz, Pit wall5Hz, Analysis range activity). Include reorder/duplicates/flashback/disconnect/cancel/storage failure phases without obscuring expected input counts. Compare raw payload count and SHA-256 stream digest; report rejected live frames separately.

Measure accepted-observation-to-render latency with a local clock-offset handshake and event IDs, saving raw sample CSV and percentile calculation. Server RSS<=512MiB after warmup and growth<=64MiB over the last90minutes; each browser<=512MiB with no unbounded log accumulation. Critical<=500ms, normal p95<250ms, no application-induced recording drops. Exclude provider/network latency only where declared; do not exclude local queue delays. Record OS/socket loss counters as measured or unavailable, never zero by assumption. Test slow-client and slow-disk isolation separately.

Real game validation uses a new2026 capture including packet16/v1.2 events, restricted-opponent behavior where available, car/formula context, flashback and race finish. Synthetic field coverage remains required when rare events cannot be induced; label the live coverage limitation. Thermal/capacity/corner references remain provisional or unavailable until separately calibrated; release can ship explicit provisional/neutral behavior but must not claim calibrated physics.

Provider qualification exercises both configured protocols, complete tool turns and critical semantic fixtures in3fixed runs with the authored prompts. Windows qualification requires native WinUI build/clean launch/tray/flush and actual game-focus PTT/microphone. Missing external inputs are explicit gates. The goal terminates blocked when independent work is exhausted and the blocking threshold is met; do not manufacture a smaller success claim.

## Risks / Trade-offs

Two-hour runs cost time/storage -> run once after narrow suites and rerun only on relevant changes/failures. Hardware variance -> declare hardware/OS/settings in evidence and compare against these fixed limits. Incomplete source coverage -> retain unavailable/provisional labels and distinguish implementation from release qualification.

## Migration Plan

Build and smoke replacement artifacts before changing published tags. Verify import/replay/export of baseline fixtures and compatibility of existing recordings. Document old commit/image rollback and API break. Do not delete baseline assets or force users to convert recordings. Publishing/pushing/merging needs the user's separate instruction.
