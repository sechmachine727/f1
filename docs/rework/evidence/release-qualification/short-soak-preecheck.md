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
