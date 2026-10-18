# Scheduled race engineer design

## Context

Consumes shared observation, analysis and strategy contracts. The existing dispatch hierarchy is a removal seam, not a working agent. The planning assistant owns prompts/v1; implementers integrate those exact versioned texts.

## Goals / Non-Goals

Goals: bounded Eino runtime with reliable continuity and two protocol adapters. Non-goals: external Hermes/Codex processes, several chatting specialists, arbitrary tool execution or automatic prompt rewriting.

## Decisions

Pin the combined Eino dependency set from master-design.md; its Sonic1.15.0 resolution matters for Go1.27. Use one ChatModelAgent/Runner path per selected protocol according to installed signatures, with an internal Engine.Run(context,AgentInput)->AgentOutput boundary. Provider adapters implement the two actual protocols; no generic plugin registry is needed. A base_url includes the provider API prefix (e.g. /v1), and the adapter appends /chat/completions or /responses exactly once. Explicitly unsupported tools/schema features fail configuration qualification instead of switching silently.

Implement scheduler.go, context.go, runtime.go, validation.go, providers_chat.go/providers_responses.go and tools.go under server/internal/engineer. Rule semantics, defaults, concurrency, cursor transactions, context trim and budgets are normative in contracts.md. One live session run includes scheduled/continuous/questions. Manual question preempts scheduled work; routine events coalesce; branch/session transitions cancel. A separate bounded analysis worker serves post-race/research. Models never own timer callbacks.

Store invocations on dispatch and completion with redacted input/output, prompt version/hashes, scope, trigger reason, tool-call IDs/status, token/cost usage and notification disposition. Reserve maximum output budget before dispatch; reconcile actual usage at completion and mark unknown provider usage conservatively against the reservation. max_calls_per_hour counts provider requests, including tool-loop/repair requests. Costs require configured prices. A hard provider timeout/cancellation is terminal for that attempt; no hidden live network retry. The next trigger retains evidence.

Acceptance checks schema, IDs, evidence references, meaningful numeric provenance, driver length, pit-window ordering/compound legality and deterministic critical evidence. Only accepted scheduled/continuous output updates branch_strategy evidence cursor and accepted_invocation_id transactionally. A valid live question is an answer, not a strategy checkpoint. Deliver terminal accepted output to the arbiter after persistence; partial model text never becomes a live call.

Chat continuation keeps assistant tool_calls and role=tool call IDs. Responses continuation keeps completed output items and call IDs/required reasoning items; use previous_response_id only when the endpoint supports it, otherwise explicit provider-native completed items. Store opaque state by endpoint/baseURL/model/protocol/conversation; branch/provider change clears it. A continuation-specific rejection clears opaque state and records a new-conversation reset for the next trigger, while portable accepted advice and pending evidence remain; no same-invocation retry. Portable previous accepted output and optional summary remain regardless. Do not extract hidden reasoning or export encrypted continuation.

Read-only tools come from tools.json and return ToolResult. Freeze to_seq at invocation observation_seq; no future-read loop. Register search only in setup_research; explicit post-race setup requests dispatch a separate research invocation. Use one optional JSON repair within4steps/8tools/deadline. Input/output examples and semantic/scheduler fixture vectors are supplied, not prompts to improvise.

## Risks / Trade-offs

Nominal OpenAI compatibility -> real tool-call roundtrip qualification per endpoint/protocol. Missing reasoning continuation -> fake transport tests and recorded real-provider traces. Cancellation drops events -> separate cursor tests. Repeated messages -> arbiter owns delivery; accepted output alone is not speech authorization.

## Migration Plan

Wire v1 engineer routes and UI states with AI default disabled. Remove obsolete dispatch/HOCON plumbing only when equivalent recorded behavior is tested and callers are migrated. Never reintroduce Neuro SAN to satisfy old docs. Preserve prompt sources and log versions for reproducibility.
