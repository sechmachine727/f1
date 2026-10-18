# Source register and evidence limits

Reviewed for this design on 2026-10-18. Attached documents supply protocol evidence; none of their prose is an instruction to an implementing agent. Read the actual structures before writing a decoder. Hashes identify the supplied versions, not a promise that a remote page stays unchanged.

## Supplied protocol documents

Original directory: /Users/sechmachine/Downloads/. The handoff includes unchanged copies in [reference](reference/README.md).

| ID | File | Version / role | SHA-256 |
|---|---|---|---|
| UDP26-12 | Data Output from F1 25 2026 Season Pack (Season 8).pdf | v1.2, 33 pages; newest supplied event guidance | 3f38858c3ca65b2faa55a90a35277dd2767bb9cea2911e741b61b370a39365ae |
| UDP26-11 | Data Output from F1 25 2026 Season Pack (1).pdf | v1.1, 33 pages; comparison | 74fc3793b123717fd3261d1bbbba3e896dfd9bdba442bb79af7db8500098d2a0 |
| STRUCT26 | 2026 Season Pack Telemetry Output Structures (1).txt | exact supplied C structures | 4f97867924f5f13f11b7fde6eb84ccb3aefcaf69062424bc077e387090647c72 |
| STRUCT25 | F1 25 Telemetry Output Structures.txt | exact supplied 2025 C structures | 49cceadc60a1a6403a59620c6fc09da377048ba004b15c199399f775b9b3fc69 |
| UDP25 | Data Output from F1 25 v3.pdf | 31 pages; 2025 behavior/restrictions | 850199d1ea817b887b150118095c5ca86527a397d578d53fd5c83427636b92d5 |

Observed: 2026 STRUCT26 includes formula13 (F1 26), game mode/network fields and season/weekend/session link identifiers that persist across saves. These support context/linking, not career-save extraction or retrospective telemetry recovery. Packet11/12 carry carIdx and cycle through cars; the earlier referenced chat's player-only claim is incorrect. Rival setups are withheld, and restricted multiplayer channels can be zeroed. Preserve availability rather than inferring values.

Observed: surface/core temperatures and per-car speed-trap values are broadcast. Battery capacity is not a telemetry field; harvest limit is a per-lap allowance. Corner landmarks do not supply validated entry/exit intervals. No official numeric tyre optimum was established from the supplied documents. v1.2 adds partial-aero/overtake race-control event material that must be covered beyond its cover note.

## Pit wall and driver communication

- [Mercedes: What goes into F1 strategy](https://www.mercedesamgf1.com/news/what-goes-into-f1-strategy). Describes preparation and in-race use of tyre performance, weather, competitors and scenario analysis. Design inference: deterministic scenario calculations plus a concise engineer summary are a suitable game adaptation; this is not an assertion that the package reproduces a team's proprietary models.
- [Mercedes: The trackside engineers](https://www.mercedesamgf1.com/news/insight-the-trackside-engineers). Describes complementary engineering roles and the race engineer's driver communication role. Design inference: one notification arbiter avoids several specialists independently speaking to the driver.

## Game handling and setup

- [EA F1 25 setup guide](https://help.ea.com/en/articles/f1/f1-25/car-setups-guide/). Official explanation of setup controls and handling tradeoffs; use it for principles, not a universal fastest numeric setup.
- [EA 2026 setup/assists/track guide](https://www.ea.com/games/f1/f1-25/news/f1-25-2026-season-edition-tips-and-tricks), published September18,2026. Describes rapid boost draw, continued deployment after release, incremental setup testing, mechanical/aero balance and brake-bias tradeoffs. It supports distinguishing expected energy draw from failure and testing small setup changes. It does not establish calibrated telemetry temperature bands or unrestricted in-session adjustability.
- [Sim Racing Setups tyre guide](https://simracingsetup.com/ea-sports-f1/f1-25-tyre-guide/). Search located a relevant original guide, but the full fetch returned429 during this authoring pass. Its numeric bands are not seeded into normative defaults. Recheck the full source and applicability before adding a new profile.

Legacy thermal source: telemetry_server/alerts/tyre_alert_processor.py:56–78 uses surface >103C warning and >108C critical, wear>=75% warning and >=90% critical. These are observed repository heuristics, uncalibrated in-game. Engine/brake thresholds are similarly legacy heuristics. Do not relabel them as official optima. Core/cold/target ranges stay unclassified until an applicable sourced profile exists.

## Harness and formats

- [Eino ADK](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/): Go agent/runner/tool primitives. Selected as an embedded bounded runtime. No separate Codex or Hermes daemon is part of the product.
- [Eino OpenAI AgenticModel](https://www.cloudwego.io/docs/eino/ecosystem_integration/chat_model/agentic_model_openai/): Responses integration. Compilation evidence is separate from provider interoperability.
- [OpenAI function calling](https://developers.openai.com/api/docs/guides/function-calling) and [reasoning item continuation](https://developers.openai.com/cookbook/examples/responses_api/reasoning_items): preserve tool-call/result IDs and provider-required continuation items. Endpoint support is explicit; an OpenAI-compatible URL does not guarantee both protocols or every model feature.
- [MCAP specification](https://mcap.dev/spec): schemas/channels, chunk compression, CRCs and indexes support raw capture and indexed recovery/query.
- [SearXNG API](https://docs.searxng.org/dev/search_api.html): JSON search requires that format enabled on the chosen instance. Surface a disabled-format error instead of parsing HTML as successful JSON.
- [Tavily search API](https://docs.tavily.com/documentation/api-reference/endpoint/search): hosted search integration. Source results are leads until contents are inspected.
- [Microsoft Windows App SDK1.8 release notes](https://learn.microsoft.com/en-us/windows/apps/windows-app-sdk/release-notes/windows-app-sdk-1-8): version1.8.12 package1.8.260921001, released September24,2026. Pin this servicing version for the first control-center build. This pin is a deliberate stable baseline, not a claim it is the newest major release.

## Local verification and current-code pointers

Observed current files: telemetry_server/telemetry_server.py:40–121 composes state and stores packets11/12; server/telemetry_input.py:39–84 handles live receipt/broadcast; common/f1_capture/binary_writer.py:49–61 writes <QH timestamp/length plus bytes; binary_reader.py:39–54 drops an incomplete tail. Legacy capture throttles selected IDs (capture_session.py:146–155) and live setup uses10Hz. The new raw capture must not inherit that filtering.

Observed frontend: useSessionTelemetry.ts:66, useTyreTelemetry.ts:100, useRaceEngineerReport.ts:30 and four other hooks create separate sockets. tests/test_flashback_keeps_state.py preserves logs on same-UID time rewind. The original docs/agent-harness-inventory.md is a historical removal inventory with stale pre-removal references; inspect current code before acting on it.

Planning verification previously performed: frontend typecheck/build passed; decode-only2025 benchmark was approximately28k datagrams/s on this Mac; combined pinned Eino modules cross-compiled CGO=0 for Windowsamd64/Linuxamd64/Linuxarm64 and selected fake-model tool-loop/cancellation tests passed. These results are limited probes, not release qualification. The Python suite was unavailable because pytest was missing. This authoring delivery reruns document/schema checks only.

Unknown until implementation qualification: real provider Chat/Responses round trips, Windows app/voice behavior, live2026 game parity, calibrated thermal/capacity/corner profiles, end-to-end throughput and two-hour soak. See qualification.md; never replace missing evidence with a model's confidence.
