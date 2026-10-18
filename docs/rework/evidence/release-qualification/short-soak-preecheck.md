# 10-minute soak pre-check

Command (run once, in parallel with the 2-hour run, on the same code):

    go test ./server/qualification -run '^TestRatedWorkload$' -count=1 -timeout 30m \
      -args -duration=10m -udp-rate=2048 -cars=24 -seed=20261018 -clients=3 \
      -evidence-dir=/tmp/qual10

Result: pass in 605 s.

| Measure | Value |
|---|---|
| Transmitted | 1,228,800 datagrams (2048.0/s) |
| Receiver intake | 1,228,800, same bytes |
| MCAP recorded | 1,228,800 |
| Payload SHA-256 | 2ff1ed195af984a4e3281735ff7780b182abb4fe3582bec4b737769969fa36dd, identical for sender and receiver |
| Application drops | 0 |
| Server peak RSS | 70,926,336 B (67.6 MiB) |
| Accepted-observation to stream | p50 0.317 ms, p95 0.534 ms, max 34.05 ms (47,998 samples) |
| Ingest lag | 5 ms |

This is a pre-check, not Gate C. The 2-hour run with three browser-analogue clients and the documented phases is separate evidence.

## Failure found and fixed during Gate C

The first clean 2-hour attempt received all 14,745,600 datagrams but recorded only 1,681,897, then stopped silently around minute 14. Intake and recording are independent, so the digest comparison still matched.

Cause: `Recorder.write` compared the cumulative file size against `chunk_bytes`, so after the first chunk every datagram triggered `file.Sync()`. As the file grew, fsync cost rose until the writer could not keep up and the bounded queue stopped recording.

Fix: checkpoint only when new bytes since the last checkpoint reach `chunk_bytes` and at least `chunk_flush_ms` has elapsed. Regression: `TestStorageFailureIsolation/checkpoint_cadence` holds a frozen clock and asserts no second checkpoint.

Verification after the fix:

| Duration | Sent | Received | Recorded | Recorder state | Peak RSS | p95 |
|---|---|---|---|---|---|---|
| 3 min | 368,640 | 368,640 | 368,640 | recording | 64.4 MiB | 0.52 ms |
| 20 min | 2,457,600 | 2,457,600 | 2,457,600 | recording | 75.4 MiB | 0.53 ms |

The 20-minute run passes the point where the earlier attempt stopped. Raw evidence: verification-soak-20m.json.
