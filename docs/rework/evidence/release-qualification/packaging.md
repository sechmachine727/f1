# Packaging evidence

Date: 2026-10-18.

## Cross-builds (CGO_ENABLED=0, Go 1.27.1)

| Target | Command | Result |
|---|---|---|
| linux/amd64 | go build -o /tmp/f1tele-linux-amd64 ./server/cmd/f1tele | pass |
| linux/arm64 | go build -o /tmp/f1tele-linux-arm64 ./server/cmd/f1tele | pass |
| windows/amd64 | go build -o /tmp/f1tele.exe ./server/cmd/f1tele | pass |

The mac-built windows/amd64 binary is a compile check. It is not Windows native qualification.

## Docker

    docker build -t f1tele:local .

Image: f1tele:local, 48.3 MB, distroless static base, nonroot user, writable /data owned by uid 65532.

Smoke:

    docker run -d --name f1smoke -p 18091:8081 -p 20778:20777/udp -v f1smoke-data:/data f1tele:local
    curl http://127.0.0.1:18091/api/v1/health

- Health returned awaiting_telemetry, then live after 120 fixture datagrams on 20778.
- After docker restart, the session from the previous run was still listed with its catalog row, and /data contains catalog.db plus recordings for that session.

## Not run

Native arm64 and amd64 image builds inside Docker on their own runners, and the CI workflow that builds them, are unverified locally; only the local host architecture image was built.
