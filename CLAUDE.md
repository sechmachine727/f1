# CLAUDE.md

Guidance for coding agents working in this repository. The current runtime is Go plus React. The old Python bridge is retained only for rollback and compatibility tests; do not add new Python runtime code.

## Build and run

```bash
./scripts/build_web.sh                       # build the dashboard into the embedded copy
go build -o f1tele ./server/cmd/f1tele       # build the server
./f1tele serve --data-dir ./data             # dashboard on http://localhost:8081, UDP on 20777
docker compose up -d --build                 # container image
```

## Verification

```bash
GOCACHE=/tmp/f1tele-go-build go test ./server/...
cd race_engineer_hub && npm test && npx tsc --noEmit && npm run lint && npm run build
openspec validate --all --strict --no-interactive
```

Loopback HTTP tests need an unsandboxed shell on macOS. Run `gofmt` on Go files you change.

## Architecture

- `server/cmd/f1tele` — CLI entry point (`serve`, `auth`).
- `server/internal/protocol` — F1 25 and 2026 Season Pack UDP decode plus the channel catalog.
- `server/internal/state` — live reducer, session/fragment/branch lifecycle, snapshots and events.
- `server/internal/history` — SQLite catalog, MCAP recorder and replay, analytics tables, agent transcripts and logs.
- `server/internal/analytics` — per-lap samples, lap analytics, stints, session summaries and backfill.
- `server/internal/engineer` — scheduler, budget, prompt assembly, provider adapters, compaction and activity audit.
- `server/internal/httpapi` — versioned HTTP, WebSocket stream and SSE activity surface.
- `server/internal/server` — composition root wiring ingestion, recording, analysis, strategy, HTTP and the engineer.
- `server/internal/config`, `server/internal/auth` — configuration store, secret handling, web password and sessions.
- `race_engineer_hub` — Vite + React + TypeScript + Tailwind dashboard, built into the server with `scripts/build_web.sh`.

Data lives in `catalog.db` plus `recordings/<session>/<fragment>/*.mcap` under the data directory. Recordings are never rewritten. Legacy `.f1bin` files import through `POST /api/v1/imports` with the original preserved.

## Conventions

- Go: standard library first, `gofmt`, table-driven tests, errors wrapped with context.
- TypeScript strict mode, ESLint config in `race_engineer_hub/eslint.config.js`.
- Keep the OpenSpec changes under `openspec/changes/` authoritative; read the applicable `AGENTS.md` before editing.
- Do not add accounts, pairing, application tokens or mandatory certificates; the dashboard is a direct local/LAN surface protected by the web password.
