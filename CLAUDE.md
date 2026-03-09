# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Setup

```bash
python -m venv venv
source venv/bin/activate && export PYTHONPATH=`pwd`   # Mac/Linux
pip install -r requirements.txt
```

For the React frontend:
```bash
cd race_engineer_hub && npm install
```

## Running

| What | Command | Port |
|------|---------|------|
| Agent network + nsflow UI | `python -m run` | 8080 (server), 4173 (nsflow) |
| Telemetry WebSocket bridge | `python -m telemetry_server` | UDP 20777 in, WS 8765 out |
| Race Engineer Hub (React UI) | `cd race_engineer_hub && npm run dev` | 8081 |
| Replay captured telemetry | `python -m telemetry_server --replay data/<file>.csv --speed 10x` | |
| Capture live telemetry to CSV | `python -m telemetry_server --capture` | |
| Download test CSV data | `./scripts/download_test_data.sh` | |

## Verification

```bash
# Python syntax check (from repo root)
python3 -m py_compile telemetry_server/telemetry_server.py

# TypeScript type check (from race_engineer_hub/)
cd race_engineer_hub && npx tsc --noEmit
```

Python commands require the venv activated and `PYTHONPATH` set to the repo root.

## Architecture

Three main systems communicate at runtime:

1. **Neuro SAN agent network** (`registries/`, `run.py`) — HOCON-defined multi-agent orchestration. Agents are declared in `registries/*.hocon` and registered in `registries/manifest.hocon`. LLM config in `registries/llm_config.hocon` (default: OpenAI gpt-5.2, requires `OPENAI_API_KEY` in `.env`).

2. **Telemetry server** (`telemetry_server/`) — Async Python WebSocket bridge. Receives F1 25 UDP packets, parses them via `F1TelemetryParser` (`f1_packet_parser.py`), generates alerts, dispatches to specialist agents, and broadcasts JSON over WebSocket.

3. **Race Engineer Hub** (`race_engineer_hub/`) — Vite + React 18 + TypeScript + Tailwind + shadcn/ui dashboard. Each hook (`src/hooks/`) opens its own WebSocket connection to the telemetry server and accumulates state.

### Data flow

```
F1 game → UDP:20777 → telemetry_server.py → parse packets → generate alerts
                                           → dispatch to specialist agents (async)
                                           → broadcast JSON over WS:8765
                                           ↓
                              race_engineer_hub (React) ← WS hooks
                                           ↓
                              Driver Radio input → WS → race engineer agent
```

### Agent hierarchy

- **Race Engineer Agent** — coordinator that reads specialist reports, filters "Copy" acknowledgments, routes follow-ups to specialists, and communicates to the driver
- **Damage Agent** — analyzes aero/brake alerts (wing damage, DRS faults, brake temps)
- **Tires Agent** — analyzes tire alerts (surface temp, wear, damage, blistering)
- **Power Unit Agent** — analyzes PU alerts (engine temp, fuel, ERS, gearbox)

Each specialist agent (`telemetry_server/*_agent.py`) uses `AgentSessionFactory` + `StreamingInputProcessor` from neuro-san. Alerts are batched with a debounce timer (1-2s) before dispatch. Responses are forwarded to the race engineer agent.

### Telemetry parsing

`F1TelemetryParser` in `f1_packet_parser.py` is the shared parser used by both `tyre_logger.py` and `telemetry_server.py`. It parses 7 packet types (Session, LapData, CarStatus, CarDamage, CarTelemetry, CarSetups, MotionEx) into state dicts. `PACKET_ID_CAR_TELEMETRY` is the trigger packet — when received, the server builds and broadcasts the full JSON message.

### Frontend patterns

- WebSocket hooks auto-reconnect every 2s on close. Each hook deduplicates responses via `useRef` tracking the last value.
- Combined alert panels (`TireAlertPanel`, `PuAlertPanel`, `AeroAlertPanel`) merge alert items and engineer responses chronologically using a merge-sort by timestamp.
- Session reset is detected when `sessionTime` drops below the previous value, clearing all accumulated alerts and agent responses.
- Panels are expandable via double-click on the header (fixed overlay pattern).
- Path alias: `@/*` → `./src/*`.

## Code style

- Python 3.12+. Line length 119 chars. Ruff + pylint for linting. Config in `pyproject.toml`.
- Python 3.12+ target. Line length: 119 characters.
- Linting: ruff (format + isort + pycodestyle + pyflakes) then pylint. Config in `pyproject.toml`.
- Imports: ES-style single-line imports, sorted by isort (`force-single-line = true`).
- Naming: Google Python Style Guide conventions -- `PascalCase` classes, `snake_case` functions/methods/variables, `UPPER_CASE` constants.
- Docstrings required on functions and classes.
- TypeScript strict mode. ESLint configured in `race_engineer_hub/eslint.config.js`.
- HOCON agent configs include a `metadata` section with `description`, `tags`, and `sample_queries`.

## Environment variables

- `OPENAI_API_KEY` — required for LLM calls
- `AGENT_MANIFEST_FILE` — defaults to `registries/manifest.hocon`
