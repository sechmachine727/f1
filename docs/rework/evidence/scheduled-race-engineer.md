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

## External blocker

No model provider or API key is configured on this machine. Real Chat Completions and Responses round trips, cancellation against a provider, and the semantic eval suite were not run. The Responses adapter is not wired in this build; it fails explicitly rather than falling back.
