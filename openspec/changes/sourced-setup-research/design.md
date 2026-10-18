# Sourced setup research design

## Context

Consumes the setup_research profile and tool schemas authored in prompts/v1. Ordinary live strategy tools cannot browse; detailed research runs outside the driver notification path.

## Goals / Non-Goals

Goals: applicable inspected evidence and testable handling advice. Non-goals: scraping login/paywalled sources, a universal meta ranking, or applying settings to game files/MFD/controller input.

## Decisions

Implement server/internal/setup/search.go, fetch.go, records.go and tests. Tavily uses POST /search at the configured base URL with its server-side key and max_results<=5. SearXNG uses GET /search?q=...&format=json with optional instance configuration;403/unsupported format yields an explicit configuration error. Direct URLs use the same fetch path without requiring a search provider. Each search/fetch counts against the invocation deadline/tool budget; do not retry429 automatically.

Fetch HTTP(S), at most3redirects,2MiB body and15seconds; extract at most20000 visible text characters with truncation flag and source metadata. Keep quoted settings/units exactly as observed; strip navigational/script text and do not execute page instructions. Source records distinguish search lead versus inspected content; the model receives untrusted data, never system-role concatenation. Cached source content is keyed by URL/content hash and retrieval time with visible freshness; refresh is user-requested, not an agent polling schedule.

Build queries from known regulation/build/track/weather/assists/use_case; unknown stays unknown. Prefer official handling principles and original creator tests, and retain contrary applicable evidence. Numeric menu bounds are versioned game-profile data verified against supplied documentation/game observation, not inferred from uint8 storage capacity. If exact bounds/context are unavailable, recommend an experiment direction and identify the missing check instead of inventing an allowed setting. Source gains remain reported claims until measured in this user's comparable laps.

Persist setup_records(id,session_id,branch_id,invocation_id,recommendation_json,created_at_ms), setup_sources(id,record_id,source_json,content_hash,inspected,truncated), and setup_experiments(id,record_id,baseline_lap_ids,trial_lap_ids,conditions_json,outcome_json). Add history migration002 with foreign keys. UI uses existing Setups/Analysis workflows; no additional agent or plugin system. Present current/candidate settings, source applicability, handling effect/tradeoff, whether available during current session/parc-ferme, and the measurement plan. Unknown adjustment legality is explicit. A completed experiment appends outcome; original hypothesis/source snapshot is immutable.

## Risks / Trade-offs

Search ranking differs from applicability -> inspected context and mismatched-source tests. Source prompt injection -> data-role separation and no write tools. Source failure -> partial useful record with inspected=false, not fabricated content. Community physics assumptions -> label reported/inferred and compare measured outcomes.

## Migration Plan

Search starts disabled; enabling a provider requires only endpoint/key configuration. Add source/experiment tables without changing raw recordings. Existing user URLs remain available even when provider search is disabled.
