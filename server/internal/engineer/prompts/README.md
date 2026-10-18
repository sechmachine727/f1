# Race-engineer prompt bundle v1.0.0

These complete instructions were authored by the planning assistant for this redesign. Implementers embed/load them byte-for-byte; changes to wording require an explicit prompt-version change, review and evaluation. Do not ask an implementation model to design the system prompt.

Assembly order: base.md, exactly one profile file, a short application-generated output-schema instruction, then AgentInput as a user data message. Tools are registered from tools.json according to profile; tool results are data messages. Do not interpolate driver names, pages or previous output into system-role text. A configured provider model must support tool calls; when structured response schemas are unavailable, validate terminal JSON and allow the single bounded repair specified in contracts.md.

Profile mapping:

| Input profile | File |
|---|---|
| scheduled_strategy | scheduled-strategy.md |
| continuous_strategy | continuous-strategy.md |
| live_question | live-question.md |
| post_race | post-race.md |
| setup_research | setup-research.md |

Explicit post-race setup requests are dispatched as a separate setup_research invocation with the selected recorded context; do not silently widen a post_race tool set. The base prompt and profile never contain keys, transport state or raw opaque reasoning. Returned rationale summaries are optional. Persist complete provider-native continuation separately as contracts.md requires.

[examples](examples.json) contains schema-valid complete input/output examples. [evaluation cases](eval-cases.jsonl) contains input mutations, mocked observations and expected semantic constraints. [scheduler cases](scheduler-cases.json) defines executable state-transition vectors independent of LLM behavior. The spec validator checks static contract consistency; it does not run a model or establish prompt quality.

Runtime acceptance rejects incorrect echoed IDs, unknown evidence IDs, malformed JSON, negative/out-of-range pit windows, source claims without inspected evidence, messages beyond35 words/240characters, and critical urgency without deterministic critical evidence. notify=false requires driver_message=null. Missing setup source details stay explicit unknown. Register no write tools and no generic shell/browser/filesystem tools.

Evaluation exit gate: all deterministic fixture checks pass; each configured real provider completes a tool call/result/final JSON round trip; all critical semantic cases pass in three fixed runs per declared provider/model/settings. Save raw redacted requests/responses, prompt hashes, usage and scores. A failed critical case blocks qualification of that endpoint. Do not auto-tune prompts in a goal loop; preserve the authored bundle, report the failing case and propose a versioned revision for review.
