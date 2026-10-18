# Durable race history design

## Context

Depends on go-telemetry-core. Legacy format is raw little-endian uint64 nanosecond epoch timestamp, uint16 payload length, then payload bytes. The old10Hz capture policy means existing files are not complete race recordings.

## Goals / Non-Goals

Goals: durable all-channel raw capture, recoverable catalog, surviving branches and independent replay. Non-goals: deleting/sampling automatically or duplicating all normalized data.

## Decisions

Use modernc SQLite (CGO=0), WAL, foreign_keys=ON, busy_timeout=5000 and schema version PRAGMA user_version. Define SQL migrations in server/internal/history/migrations; migrate a catalog copy first in tests. Tables: source_contexts(id,recorder_install_id,sender_ip,next_reserved_seq); sessions(id,regulation_season,career_id,mode,track,context_json,started_at_ms); careers(id,name); fragments(id,session_id,source_context_id,game_uid,link_ids_json,source,started_at_ms); branches(id,session_id,parent_id,anchor_seq,anchor_time_ms,is_surviving); recordings(id,fragment_id,path,bytes,state,durable_offset,last_seq,sha256); laps(id,session_id,branch_id,car_idx,lap,start_seq,end_seq,time_ms,validity_json,summary_json); stints(id,session_id,branch_id,car_idx,compound,start_lap,end_lap,summary_json); events(id,session_id,branch_id,seq,type,severity,payload_json); gaps(id,session_id,branch_id,from_seq,to_seq,reason); invocations(id,session_id,branch_id,status,input_json,output_json,usage_json,provider_state_blob); branch_strategy(branch_id,dispatch_cursor_json,evidence_cursor,accepted_invocation_id). Use IDs and foreign keys, uniqueness(session,branch,car,lap) for final lap summaries, and indexes(session,branch,seq)/(session,branch,car,lap). A branch ancestry resolver supplies inherited-prefix queries instead of copying samples.

MCAP message encoding f1udp, schema name F1UDP/<format>/<packetVersion>, channel per protocol/packet ID; message data is original UDP payload. log_time/publish_time are receive epoch ns. Companion metadata/checkpoint records map receive sequences, session time, packet identity and validation outcome to file/chunk offsets; raw per-message sequence uses MCAP sequence for low32 bits with segment sequence base metadata for uint64 reconstruction. Unknown formats use a raw/unknown channel. Zstd chunks1MiB or1s with CRC/index; rotate256MiB, session fragment change and explicit stop. Close writes summary/footer and commits size/hash in catalog. Partial files retain .partial suffix and recover into a separate .mcap file. File creation uses unique IDs and exclusive create; source files are never overwritten.

Keep64MiB bounded writer queue and explicit record admission/cap checks. MCAP fsync checkpoint before SQLite checkpoint. On startup compare catalog/file offsets, verify CRCs and reindex later durable chunks. A failed checkpoint stops recording visibly, preserves files and live processing; recovery is idempotent. Querying/import/export runs at bounded concurrency and cannot hold the writer transaction while scanning raw files.

Cap includes raw recordings/partial imports, reserve check includes catalog/temp/export writes. Warn at80%. Stop before exceeding cap using admitted bytes plus current chunk reservation. No automatic purge. Cleanup resolves only catalog-owned IDs, refuses open/active references, removes each file and commits corresponding deletion only when successful; partial cleanup returns per-ID errors. A new recording after Stop/limit requires explicit start or next application launch; do not restart into a full disk loop.

Persist branches at actual flashback anchors. Inherit strictly earlier prefix; child owns anchor inclusively. Resume fragments use nonzero game link identifiers plus persisted source_context_id, never UID alone to merge across installs. Packet11/12 history/tyre allocation updates are keyed by carIdx without player-only filtering. Rebuild derived laps/stints after rewind and exclude incomplete lap halves. Replay uses its own reducer, default1x, pauses preserved, allowed0.25–8x and explicit Skip long pauses option; no live events escape it.

## Risks / Trade-offs

If storage failure also prevents writing a gap/error to SQLite, retain the visible error in memory and do not claim that audit row is durable. On recovery, mark the interval after the last durable checkpoint as uncertain through the first new captured point. Recovery cannot reconstruct datagrams never durably written.

Disk stalls -> bounded visible stop instead of unbounded RAM. Metadata crash -> checkpoint and raw reindex tests. Wrong association -> keep unassigned and allow manual grouping. Large scans -> indexed ranges and request cancellation.

## Migration Plan

Create a fresh catalog in the chosen data directory; import .f1bin explicitly and retain originals. SQLite migration failures leave original catalog and previous app usable; no automatic destructive rollback of recorded history.
