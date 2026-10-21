# Setup

This package runs a local telemetry server and dashboard for F1 25. It records every datagram the game sends, catalogues sessions, and serves the web workspace on port 8081.

## Choose how to run it

- Docker Compose runs the same Linux image as the published package. Use this on a machine that already has Docker.
- The native binary runs on macOS, Linux or Windows when you have Go and Node available to build it.
- The Windows portable ZIP adds the control center and tray. Configure that build in its own Settings page.

All three need the game to send UDP telemetry to the machine running the server.

## Docker

Prerequisites: Docker Engine or Docker Desktop with the Compose plugin.

1. Clone the repository and enter it.

   ```bash
   git clone https://github.com/sechmachine727/f1
   cd f1
   ```

2. Build and start the service.

   ```bash
   docker compose up -d --build
   ```

3. Find the generated dashboard password in the service log. It is printed once.

   ```bash
   docker compose logs f1tele | grep "web password"
   ```

4. Open [http://localhost:8081](http://localhost:8081) and sign in.

The Compose file binds `8081/tcp` and `20777/udp`, and keeps data in the `f1tele-data` volume. The process runs as a non-root user on a distroless image.

To check the service:

```bash
docker compose ps
curl -fsS http://localhost:8081/api/v1/health
```

To set a known password instead, export `F1TELE_AUTH_PASSWORD` before the first start. To replace an existing password later:

```bash
docker compose exec f1tele /f1tele auth reset
```

If the image predates the `F1TELE_DATA_DIR=/data` default, add `--data-dir /data`.

The reset asks for the new password twice. Piping it to `auth reset --stdin` reads it once and skips confirmation.

When the game runs on another machine, set the address the dashboard should show for the game:

```bash
F1TELE_UDP_ADVERTISE_HOST=192.168.1.20 docker compose up -d
```

Use the Docker host's LAN address. The Settings workspace shows the resulting `host:20777` value.

## Native binary

Prerequisites: Go 1.27.1 or newer, and Node 24 or newer for the dashboard build.

```bash
./scripts/build_web.sh
go build -o f1tele ./server/cmd/f1tele
./f1tele serve --data-dir ./data
```

The first start prints `F1Telemetry web password (shown once): ...`. Sign in with it, or start with `F1TELE_AUTH_PASSWORD` set. Reset a stored password with:

```bash
./f1tele auth reset --data-dir ./data
```

`build_web.sh` builds the dashboard into the server's embedded copy. A plain `go build` without it serves the previously embedded bundle. Run it after frontend changes.

Without `--data-dir`, data goes under the platform data directory. The server defaults to `0.0.0.0:8081` for HTTP and `0.0.0.0:20777` for UDP. Override with `--http`, `--udp`, `--data-dir` or `--config`, or with the `F1TELE_*` variables in [Settings](settings.md).

## Windows portable ZIP

On Windows 11 x64 with .NET, Go and Node installed:

```powershell
pwsh ./windows/package.ps1
```

The script writes `artifacts/F1Telemetry-1.0.0-win-x64.zip`. Extract it and run the control center. The first launch opens Settings before the server starts. Set the dashboard password and connection fields there; a generated password is shown once with a copy button. The control center then starts and supervises the bundled server and keeps it in the tray.

Later **Settings** saves stop the owned server, write `config.json` and `secrets.json`, and restart it. The reset password button accepts a new password and confirmation without the old password. Agent and harness settings, prompt revisions and activity logs are in the web Settings after login.

## Connect the game

In F1 25, open **Settings**, then **Telemetry Settings**, and set:

| Setting | Value |
|---|---|
| UDP Telemetry | On |
| UDP Broadcast Mode | Off |
| UDP IP Address | The server's address, or `127.0.0.1` on the same machine |
| UDP Port | `20777` |
| UDP Send Rate | 60 Hz |
| UDP Format | 2026 |
| Your Telemetry | Public |

Restricted telemetry still records, but rival ERS, fuel and tyre channels become unavailable. The dashboard labels them rather than showing zero.

## First run

`GET /api/v1/health` reports `awaiting_telemetry` until the first datagram arrives. That state is normal with the game closed. Once packets arrive, the state becomes `live` and the first session appears in History.

To exercise the dashboard without the game, build and run the synthetic sender:

```bash
go run ./server/cmd/f1demo -seconds 120 -rate 512
```

It sends the same 2026 payloads as the rated workload and is a development helper for UI checks.

## Data and upgrades

The data directory holds `catalog.db` and `recordings/<session>/<fragment>/*.mcap`. Recordings are never rewritten on upgrade. Back up the whole directory before moving the install.

With Compose, the data lives in the `f1tele-data` volume. Inspect it with `docker volume inspect f1tele-data`. To rebuild after a source update, run `docker compose up -d --build` again; the volume survives.

Recording limits default to a 100 GiB cap with 5 GiB kept free, and no automatic deletion. You can stop recording explicitly and delete recordings from History. Deletion requires confirmation and refuses active recordings.

## Troubleshooting

- **Health stays `awaiting_telemetry`:** check the game's IP and port, and confirm broadcast mode is off.
- **The dashboard trails the game by seconds:** check that UDP Broadcast Mode is off and the game targets this machine's IP directly. Frames sent to `255.255.255.255` can sit in an access point's multicast buffer; a measured Wi-Fi capture delivered them about 5 seconds late.
- **`bind udp ... address already in use`:** another process owns 20777. Stop it or pass a different `--udp` value.
- **Recording state `stopped_limit`:** the cap or free-disk reserve was reached. Delete recordings you no longer need, then start a new recording.
- **Port 8081 already in use:** stop the other service or set `F1TELE_HTTP_ADDR` and the Compose port mapping together.

