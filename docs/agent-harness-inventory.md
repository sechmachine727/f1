# Agent harness inventory

Scope: everything the telemetry bridge uses to produce agent output, and everything
that exists only to support the upstream Neuro SAN demo. Written before replacing the
harness, so the new implementation starts from a known surface.

Status: the removal below is applied on `codex/reimplement-agent-harness`. The four
specialist prompt HOCONs remain in `registries/` as reference text for the new harness,
and the dashboard alert panels are now titled "Tyre Alerts", "Power Unit Alerts",
"Damage Alerts", and "Race Engineer".

## What the bridge actually uses

`telemetry_server` imports Neuro SAN in exactly four places
(`telemetry_server/agents/*_agent.py:7-8`). Each wrapper opens one direct session:

| Network | Session name | Called from |
|---|---|---|
| `damage_engineer` | `DamageAgent` | `dispatch/damage_dispatcher.py` |
| `tyres_engineer` | `TyresAgent` | `dispatch/tyres_dispatcher.py` |
| `power_unit_engineer` | `PowerUnitAgent` | `dispatch/pu_dispatcher.py` |
| `race_engineer` | `RaceEngineerAgent` | `dispatch/race_engineer_dispatcher.py` |

The only environment variable the bridge sets is `AGENT_MANIFEST_FILE`
(`telemetry_server/telemetry_server.py:129`). It never sets
`AGENT_TOOLBOX_INFO_FILE` or `MCP_SERVERS_INFO_FILE`, so `toolbox/` and `mcp/`
are not read at runtime. The other files in `registries/` exist for the studio
network in `formula_1_racing_team.hocon`.

## Delete: upstream studio and demo layer

These have no reference from the telemetry server. The repo has left the fork, so the
upstream demo is dead weight.

| Path | Lines | Why |
|---|---:|---|
| `run.py` | 441 | Launches the Neuro SAN server plus nsflow on 8080/4173. Includes a `lsof` + `kill -9` port cleanup helper (`run.py:333-362`). |
| `registries/formula_1_racing_team.hocon` | 232 | 20-agent AAOSA demo network, never instantiated by the bridge. |
| `registries/aaosa.hocon` | 82 | Substitution block only used by the demo network. |
| `registries/manifest.hocon` | 26 | Registers the demo network. Replace with whatever config the new harness needs. |
| `f1_team.md` | 138 | The prompt/spec that produced the demo network. |
| `mcp/mcp_info.hocon` | 60 | Never read by the bridge. Also currently unparseable without `GITHUB_TOKEN` and `GOOGLE_API_KEY` because the "commented" sections are uncommented. |
| `toolbox/toolbox_info.hocon` | 711 | LangChain toolkits (Gmail, Jira, etc.). Never read by the bridge. |

Keeping the four specialist prompt HOCONs is worthwhile even if the format changes:
their instruction text is tuned and sits at
`registries/{damage,tyres,power_unit,race}_engineer.hocon`.

## Replace: harness adapters

The wrapper and dispatch layers are shaped around Neuro SAN sessions. The queue/flush
behavior is worth keeping; the session plumbing is not.

| Path | Lines | Disposition |
|---|---:|---|
| `telemetry_server/agents/damage_agent.py` | 96 | Clone of the other three. Differs only in `TEST_INPUT`, class name, network name, log name. |
| `telemetry_server/agents/tyres_agent.py` | 95 | Same clone. |
| `telemetry_server/agents/power_unit_agent.py` | 95 | Same clone. |
| `telemetry_server/agents/race_engineer_agent.py` | 95 | Same clone, plus a session-context instruction append. |
| `telemetry_server/dispatch/base_agent_dispatcher.py` | 101 | Keep the batching, re-entrancy guard, and `response`/`response_time` fields. Agent call site at line 69 changes. |
| `telemetry_server/dispatch/{damage,tyres,pu}_dispatcher.py` | 35 each | Thin subclasses that only forward to the race engineer. Collapse into the base or a single specialist class. |
| `telemetry_server/dispatch/race_engineer_dispatcher.py` | 186 | Keeps prompt-specific prefix parsing (`To Damage Engineer:` etc.) in Python (`lines 112-180`). Move routing into the new harness or delete if the new design does not forward between agents. |
| `telemetry_server/dispatch/agent_dispatch_manager.py` | 110 | Facade, plus `_agent_defs()`. Becomes the new harness entry point. |

The four wrapper files are 381 lines of copy-paste. Their `__main__` blocks import
`timedinput`, which is not in `requirements.txt`; that path already fails.

## Keep: contracts the UI depends on

Rewire these, do not remove them.

| Contract | Location |
|---|---|
| Report payload shape: `damageReport`, `tyresReport`, `puReport` = `{response, time}` | `telemetry_server/server/message_builder.py:185-196` |
| Race engineer payload: `raceEngineerReport` = `{responses, alerts}` | `telemetry_server/server/message_builder.py:197-200` |
| Driver radio inbound message: `{driverMessage: string}` | `telemetry_server/server/websocket_server.py:62-68` |
| Alert dispatch at packet 6, and session re-init hooks | `telemetry_server/server/telemetry_input.py:78-88, 135-143` |
| UI panels and hooks: `RaceEngineerPanel`, `DriverRadioInput`, `useRaceEngineerReport`, `engineerResponses` props | `race_engineer_hub/src/pages/Index.tsx:30-36, 121-136` |

The frontend surface is thin: one hook and one panel for the race engineer, plus an
`engineerResponses` prop on the three specialist alert panels. A new harness only
needs to fill the four JSON keys above.

## Delete: dead frontend

| Path | Why |
|---|---|
| `race_engineer_hub/src/components/EngineerReport.tsx` | Never imported. |
| `AlertBox` component export in `race_engineer_hub/src/components/AlertBox.tsx` | Only the `Alert` type is imported anywhere. Keep the type, drop or fold the component. |
| Root `package-lock.json` | 119-byte empty lockfile for the repo root. The frontend has its own. |

## Dependencies and image

`requirements.txt` shrinks substantially when Neuro SAN goes:

| Line | Package | Note |
|---|---|---|
| 2 | `neuro-san` | Harness. Removed. |
| 3 | `nsflow` | Studio client for `run.py`. Removed. |
| 5-7 | `leaf-common==1.2.51` | Pin exists only because Neuro SAN 0.6.47 imports the old module path. Removed with it. |
| 10 | `langchain_anthropic` | Only for the default `claude-sonnet` config. Drop unless the new client speaks Anthropic. |
| 16 | `PySide6` | Unused by tracked code. Docker filters it out at `Dockerfile:26-27`; remove both. |
| 13 | `python-dotenv` | Used by `telemetry_server.py:151`. Keep. |

Neuro SAN also brings LangChain and `pyhocon` transitively. If the new harness keeps
HOCON config, add `pyhocon` explicitly; do not rely on a transitive pin.

The Docker image should drop from roughly 1.5 GB to well under 500 MB once Neuro SAN,
nsflow, and the filtered Qt wheel are gone. The arm64 wheel problem documented in
`Dockerfile:23-24` disappears with the filter.

## Config, docs, and runtime surface

| Path | Cleanup |
|---|---|
| `.env.example` | Replace LLM provider keys with the new client's variables. Keep the endpoint variables as the compatibility target if the new client keeps Chat Completions semantics. |
| `docker-compose.yml:10-23` | Drop `registries/`, `mcp/`, `toolbox/` mounts unless the new harness reads them. `./logs` mount depends on where the new harness writes. |
| `docker/entrypoint.sh:3` | `--no-agents` becomes meaningless once agents are the harness. Decide whether the new flag exists. |
| `.github/workflows/docker-publish.yml:139` | Smoke test passes `--no-agents`; update with the flag. |
| `README.md:188-267` | Agent network, LLM configuration, custom endpoints, and observability sections describe Neuro SAN. Rewrite after the harness lands. |
| `CLAUDE.md:22, 48-79, 102, 106-113` | Same: agent hierarchy and environment variables are Neuro SAN-specific. |

## Tests tied to the old harness

These verify Neuro SAN's config loader and provider wiring, not telemetry behavior:

- `tests/registry_config.py`
- `tests/stub_openai_server.py`
- `tests/test_custom_openai_endpoint.py`
- `tests/test_llm_config_endpoint.py`

Keep the stub server and the endpoint assertions if the new client is OpenAI-compatible;
they are a useful specification. The registry loader tests go with the HOCON files.

## Removal order

1. Delete the studio layer (`run.py`, demo network, `aaosa`, `f1_team.md`, `mcp/`, `toolbox/`).
2. Collapse the four agent wrappers into the new harness client; keep `BaseAgentDispatcher`.
3. Collapse the three specialist dispatcher subclasses; keep or rewrite the race engineer routing.
4. Trim `requirements.txt` and the Dockerfile filter; rebuild and run the suite.
5. Delete the Neuro SAN config tests and rewrite the docs.
6. Remove the dead frontend files and the root lockfile.

## Unrelated defects found while mapping

- `mcp/mcp_info.hocon:33-51` has two sections documented as commented but active. Any
  Neuro SAN startup with MCP enabled fails unless both keys are set.
- `telemetry_server/agents/*_agent.py` `__main__` blocks need `timedinput`, which is
  not declared. They are dev-only and go away with the wrappers.
- `lap_logger.py` duplicates the 2025 lap packet layout with a hard-coded 22-car count.
  It is superseded by `--capture` and is a candidate for deletion, but it is independent
  of the agent harness.
