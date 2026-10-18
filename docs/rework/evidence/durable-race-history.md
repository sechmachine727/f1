# durable-race-history evidence

## Commands

    rtk go test ./server/internal/history -run 'TestCatalog|TestMigration'
    rtk go test ./server/internal/history -run TestRecordingRoundTrip
    rtk go test ./server/internal/history -run 'TestStorage|TestWarn'
    rtk go test ./server/internal/history -run TestSurvivingBranchTimeline
    rtk go test ./server/internal/history -run TestLegacyImportAndRecovery
    rtk go test ./server/internal/server -run TestReplayIsolation

All pass.

## Coverage

- Migration creates every specified table with foreign keys, WAL, busy timeout and user_version.
- Sequence blocks of 4096 are reserved durably before use; reopening continues after the reserved block, so values are never reused.
- MCAP round trip writes 62 datagrams (60 valid, one duplicate, one invalid 3-byte payload) with Zstandard chunks, CRCs and indexes. After reopen all payloads and receive timestamps match; the invalid datagram is preserved byte-for-byte.
- Storage limits: queue exhaustion stops recording without blocking live state; the cap and the free reserve both produce stopped_limit with a reason. No file is deleted.
- Surviving branch resolution returns parent prefix strictly before the anchor, then child data from the anchor.
- Legacy import preserves the original file byte-for-byte, reports 13 records, and flags a truncated tail. Import writes a new MCAP artifact and verifies record count.
- Recovery from a truncated multi-chunk file copies readable messages into a new file, sets lost-tail, and leaves the damaged source in place.
- Replay reads a recording into its own reducer; seeking and speed changes leave the recording byte-identical and the replay does not claim the live source.

## Limits

A CRC-corrupted chunk fixture and repeated concurrent recovery are not covered. Cleanup-protected-file behaviour is covered at the HTTP layer with a not_found result; a real active-recording deletion refusal still needs a dedicated fixture.
