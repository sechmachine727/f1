# scheduled-race-engineer evidence

## Commands

    rtk go test -race ./server/internal/engineer
    rtk go test ./server/internal/engineer -run 'TestSchedulerVectors|TestOutputAcceptance|TestInvocationLimits|TestPromptBundleMatchesManifest|TestToolRegistrationIsProfileScoped'

## Coverage

- All 16 vectors from docs/rework/prompts/v1/scheduler-cases.json pass, including cancelled Safety Car retention, new evidence during a run, branch-change supersession, budget blocking, ALL latches, question non-consumption and expired continuation.
- Dispatch and evidence cursors are separate; only acceptance advances evidence.
- Output acceptance rejects echoed-scope mismatch, unknown evidence IDs, notify=false with a message, over-long messages, critical urgency without critical evidence, invalid pit windows, malformed JSON and fenced JSON.
- Prompt bundle hashes match the authored manifest; base plus exactly one profile is assembled in order.
- Tool registration is profile-scoped: scheduled and continuous expose read + evaluate; live and post-race add compare; setup research exposes compare + search + fetch and never evaluate.

## Provider protocols

Both adapters are implemented on pinned Eino versions: Chat Completions through `eino-ext/components/model/openai`, Responses through `agenticopenai` with `schema.AgenticMessage` and a typed runner. `TestResponsesProtocol` drives a fake Responses endpoint: the first turn returns a `function_call`, the second request carries the same `call_id` as `function_call_output`, and the terminal message is the output JSON. Tool backends that are absent return a typed `unavailable` error instead of panicking.

## Context budget

`BoundObservation` caps 512 metrics and 64 recent plus 64 critical events, ranks the selected car first, and records truncation and omitted counts. `TestContextBounds` covers an oversized input; `TestBoundObservationKeepsSmallInputs` proves small inputs pass through unchanged.

## External blocker

No model provider or API key is configured on this machine. Real Chat Completions and Responses round trips, cancellation against a provider, and the semantic eval suite were not run. The fake-endpoint test proves message shapes, not provider compatibility.
