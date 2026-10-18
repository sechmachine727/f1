# Go telemetry core design

## Context

See proposal.md and the current paths in docs/rework/sources.md. Python remains a runnable comparison baseline during this change; do not switch Docker or delete it yet.

## Goals / Non-Goals

Goals: one complete typed ingestion/state/API spine. Non-goals: persistence internals, model orchestration and a second legacy compatibility API.

## Decisions

Use Go1.27.1 with a root module named f1tele and packages under server/internal. The server composition root owns UDP receive, a recording sink boundary, a single serialized live reducer, immutable snapshots and HTTP. Pass read-only snapshots into downstream packages. Use net/http plus coder/websocket, JSON config and the normative schema. Start with AI/history consumers absent until their dependent change; health explicitly distinguishes available subsystems.

ReadMsgUDP uses a65535-byte buffer; assign receive sequence and monotonic/epoch receive times immediately. Forward unchanged bytes to the recorder boundary before parsing or gate decisions. Decoder dispatch is keyed by packetFormat and packetVersion, with exact lengths, enums and signed/scaled fields from supplied structures/PDFs. Keep unknown raw field codes; show human-readable Unknown in UI. Build a channel catalog mapping every decoded field, wheel order and restriction rule.

For current state, key ordering by session/packetID and carIdx where a packet cycles cars, not one global newest frame. Use uint32 serial arithmetic for wraparound. Exact duplicates are suppressed by frame/key/payload fingerprint; distinct event payloads in the same frame survive. Accept unseen events within a128-frame reorder window, timestamped with their game time; older events remain raw/auditable and count as late. Never reset alert state after an arbitrary rejection count. A valid flashback event supplies the anchor; absent that, require a newer accepted overall frame with confirmed game-time regression. History owns branch persistence in change2.

Use config precedence explicit CLI > F1TELE_HTTP_ADDR/F1TELE_UDP_ADDR/F1TELE_DATA_DIR/F1TELE_CONFIG > config.json > defaults.json. CLI is f1tele serve --config <path> --data-dir <path>, with optional --http/--udp address overrides. Environment secret names are referenced by endpoint config. Persist user-supplied secret values to a separate user-readable-only secrets file; never serialize them in GET config. Bind changes require restart and show desired versus active addresses. Defaults: Windows LOCALAPPDATA/F1Telemetry, container /data; native Unix XDG_DATA_HOME/f1tele or user local data directory.

Implement exact HTTP/stream contracts from contracts.md. Stream telemetry defaults20Hz, selected telemetry cap60Hz, standings5Hz, immediate events. State mutation and delivery queues remain separate; a failed socket never returns an empty successful frame. Context/availability state is sent on connect and after changes. v1 readiness means services running, even while waiting for game packets.

## Risks / Trade-offs

Binary compatibility mistakes -> packet-size/golden and negative tests before reducer wiring. Shared mutable maps -> single reducer ownership and copied snapshots checked with race detector. Raw recorder backlog -> history supplies a bounded failure signal; it must never block the live reducer indefinitely. Same-frame events -> explicit multi-event fixture prevents packet-ID-only deduplication.

## Migration Plan

Keep Python commands/fixtures unchanged. Build Go independently; stage5 adopts v1 in the frontend and stage9 switches Docker/docs atomically. Roll back to the baseline runtime without rewriting recordings.
