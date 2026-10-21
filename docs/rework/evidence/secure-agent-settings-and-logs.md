# Secure agent settings and activity evidence

## Passed on 2026-10-21

- `GOCACHE=/tmp/f1tele-go-build go test ./server/...` passed all packages.
- `GOCACHE=/tmp/f1tele-go-build go test -race ./server/...` passed all packages.
- `go test ./server/internal/auth` covers generation, expiry, tampering, environment ownership and reset invalidation.
- `go test ./server/internal/history` covers event ordering, compaction lookup, message ordering and prompt-revision deletion protection.
- `go test ./server/internal/engineer` covers a fake OpenAI-compatible compaction call, provider usage, compaction events and the compaction invocation row.
- `go test ./server/internal/state` covers a new UID session transition and a same-UID restart returning to the session-clock origin. Both clear the paused flag, old samples and old frame ordering.
- `go test ./server/internal/server -run TestStreamClearsPausedOnNewSession` passes: a subscribed WebSocket sees Q1 paused at 300 km/h, then Q2 at 100 km/h with `session.paused=false` after the UID and frame numbers restart.
- `npm test` in `race_engineer_hub` passed 79 tests.
- `npm run lint` passed with warnings only; the repository already carries warnings in shared UI files.
- `npm run build` passed. Monaco is split into the lazy `AgentSettings` chunk.
- `npx tsc --noEmit` passed under the existing root project-references configuration.
- `openspec validate --all --strict --no-interactive` passed all 14 changes.
- Windows Core `dotnet build` passed with no warnings or errors.
- Windows Core tests passed 15/15.
- The contract validator passed 79 definitions, 74 fixture assertions and 19 semantic reference cases. Its artifact text scan still fails on the pre-existing em dash in `docs/rework/evidence/telemetry-traces.md`.

## Browser checks

A temporary server on `127.0.0.1:8099` with a fresh data directory and `F1TELE_AUTH_PASSWORD=test-password`:

- Unauthenticated data routes returned 401; health and auth status remained public.
- Login set a session cookie and loaded Drive.
- Logs listed a seeded accepted invocation with input/output tokens, latency, event count and decision.
- Logs detail rendered Overview, Input, Output, Tools, Context and Raw tabs. Input showed the exact system prompt snapshot and current input.
- Settings rendered Agent, Prompt, Compaction, Connections and Security.
- Prompt loaded the authored text in Monaco, listed custom revisions, disabled deletion of the current revision, showed an inline Delete? confirmation for an older revision, and rendered a side-by-side red/green diff before saving.
- Saving the reviewed diff selected the new revision and kept the older revision.
- Compaction rendered the 80/25 defaults and the manual compact action. It correctly reported that no active live conversation was available.
- The final container was rebuilt and started with `F1TELE_UDP_ADVERTISE_HOST=192.168.69.42`. Its first start printed the generated password once; a restart printed no password.
- Container checks: login 200, authenticated config 200, cross-origin config 403, same-origin config 200, authenticated SSE returned a real `retry: 3000` frame, and an agent config PATCH round trip incremented revisions and restored the original value.
- The container now sets `F1TELE_DATA_DIR=/data`. `docker exec -i f1-25-tele-f1tele-1 /f1tele auth reset --stdin` succeeded without `--data-dir`, the temporary password logged in, and resetting back to the generated password restored login.
- Corner decoding now reads each wheel-array element independently. A synthetic packet 10 produced FL30%, FR40%, RL10% and RR20% wear, with distinct surface, core, pressure, brake and damage values.
- The tyre-wear graph uses the worst wheel value and a continuous green-to-red scale. Unit tests pin the scale at 0% green, 50% yellow and 100% red; the synthetic 40% graph rendered yellow-green.

## Not yet qualified

- WinUI XAML compilation and a real Windows first-run/reset launch require a Windows host. The macOS build stops at the Windows XamlCompiler executable.
- Provider continuation replay is not persisted by the existing Responses adapter. Compaction writes a continuation-reset marker, but full opaque-item replay remains open.
- Pit wall live activity and History deep links compile and their data paths are tested through the activity API; a real game session was not available for this evidence pass.
- No 2560-width screenshot pass was taken in this session.
