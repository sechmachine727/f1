# F1 25 Telemetry Capture & Replay

Captures live F1 25 UDP telemetry to `.f1bin` binary files and replays them with a terminal dashboard.

## Prerequisites

```bash
source .venv/bin/activate
export PYTHONPATH=$(pwd)
```

Ensure the F1 25 game is configured to send UDP telemetry on port 20777 (Settings > Telemetry).

## Live Capture

```bash
# Capture at 10 Hz (default) with live terminal viewer
python -m common.f1_capture --live

# Capture at 20 Hz
python -m common.f1_capture --hz 20 --live

# Capture at full 60 Hz (matches game frame rate)
python -m common.f1_capture --hz 60 --live

# Capture without the terminal viewer (headless)
python -m common.f1_capture --hz 20

# View-only mode — terminal viewer without writing a .f1bin file
python -m common.f1_capture --live --no-capture
```

Capture files are written to `data/` by default. Use `--output <dir>` to change.

## Replay

```bash
# Replay a captured session at real-time speed
python -m common.f1_capture --replay data/<file>.f1bin

# Replay at 5x speed
python -m common.f1_capture --replay data/<file>.f1bin --speed 5

# Replay at 10x speed
python -m common.f1_capture --replay data/<file>.f1bin --speed 10
```

## All Options

| Flag | Default | Description |
|------|---------|-------------|
| `--hz N` | 10 | Capture frequency for high-rate packets (Hz) |
| `--port N` | 20777 | UDP port to listen on |
| `--output DIR` | `data/` | Output directory for `.f1bin` files |
| `--live` | off | Show live terminal viewer |
| `--no-capture` | off | Disable `.f1bin` output (view-only mode) |
| `--replay FILE` | — | Replay a `.f1bin` file instead of live capture |
| `--speed N` | 1.0 | Replay speed multiplier (e.g. 5.0 = 5x faster) |

Press **Ctrl+C** to stop capture or replay.

## Capture Frequency

The `--hz` flag controls how often high-frequency packets are written to disk. The game sends Motion, LapData, Telemetry, CarStatus, and MotionEx at up to 60 Hz. Each packet type is gated independently:

- `--hz 10` — ~10 frames/sec per type (~50 packets/sec total)
- `--hz 20` — ~20 frames/sec per type (~100 packets/sec total)
- `--hz 60` — full rate, no gating

Low-frequency packets (Session, Event, Participants, Setups, Damage, SessionHistory, TyreSets, LapPositions) are always captured regardless of the Hz setting, since they carry infrequent state changes.

## Terminal Viewer

The live viewer renders a 4-quadrant ANSI dashboard:

| Quadrant | Content |
|----------|---------|
| Top-left | **TYRES** — compound, wear, temps, pressures, brakes |
| Top-right | **POWER UNIT** — speed, RPM, gear, fuel, ERS, component wear |
| Bottom-left | **AERODYNAMICS** — wing damage, floor, diffuser, DRS, ride height |
| Bottom-right | **TRACK MAP** — ASCII track outline with car positions |

## Binary Format (.f1bin)

Each frame: `[uint64 timestamp_ns][uint16 payload_length][bytes payload]`

The payload is the raw UDP datagram as received from the game, preserving the original F1 25 packet structure for lossless replay.
