# Paste-ready implementation goal

The prompt below starts application implementation after the user has reviewed the specification bundle. It does not authorize publishing, external messages or automatic deletion of history. Paste it into a new chat attached to this repository. For a smaller goal, replace the stated full-program scope with one named change before starting; do not let the implementation agent silently choose a smaller goal.

> Superseded: the web password, signed sessions and default-on Origin check added by `openspec/changes/secure-agent-settings-and-logs` replace the historical "no accounts/login/Origin checks" decision in the prompt below. Endpoint connection fields and secrets remain environment or native-Windows owned; the web surface is protected by the dashboard password.

~~~text
Implement the approved F1 telemetry redesign as a finite goal in /Users/sechmachine/Documents/Clanka/f1-25-tele. The specification bundle is docs/rework/README.md. Preserve the existing working tree and all user changes. Inspect git status and current instructions before edits; never reset/overwrite foreign work. I authorize application implementation of the nine ordered OpenSpec changes, local builds/tests and local packaging. Do not publish, push, merge, archive OpenSpec changes, send external messages, install services/autostart, delete existing recordings, or add unrelated features.

Read docs/rework/master-design.md, contracts.md, defaults.json, qualification.md, sources.md, and ui/README.md. Open ui/approved-workspaces.html and inspect the saved Drive/Pit wall screenshots and editable layout. These are required implementation references. Match their hierarchy/readability and the written state/availability rules; their numbers are illustrative. Protocol references are copied into docs/rework/reference/ with hashes in sources.md.

Use the existing OpenSpec changes in this order: go-telemetry-core; durable-race-history; telemetry-analysis-exports; deterministic-race-strategy; telemetry-workspaces; scheduled-race-engineer; sourced-setup-research; windows-control-center; release-qualification. Before each change, use the repository's openspec-apply-change skill and CLI status/instructions; read that change's proposal, design, specs and tasks. Do not replace this design with a new brainstorm. Resolve routine reversible choices yourself. Stop only for a material contract contradiction or missing external requirement, after completing independent work.

Create a goal with the concrete objective below if goal tools are available. Do not invent a token budget. Keep one compact checkpoint file at docs/rework/evidence/implementation-status.md: current change/task, passing evidence, failures, next step and blocker. Update it at meaningful checkpoints, not every tool call. Each task checkbox is marked complete only when its stated verification passed. Record baseline failures separately with reproduction evidence.

Objective: implement all nine changes, produce the supported native/Docker artifacts, meet all required A–E gates in docs/rework/qualification.md, and produce docs/rework/evidence/final-report.md with exact commands/results, requirement-to-evidence mapping, artifact hashes, remaining limitations and rollback. A report or green unit suite alone is not completion.

Use relevant installed skills, reading each exact SKILL.md before applying it:
- .agents/skills/openspec-apply-change/SKILL.md for artifact-driven tasks; openspec-explore only for a concrete unresolved question.
- ~/.agents/skills/ponytail/SKILL.md for the smallest complete implementation, while preserving the approved UX.
- Superpowers using-git-worktrees, test-driven-development, systematic-debugging, receiving-code-review and verification-before-completion for isolation, focused regression checks and evidence. Inspect actual installed paths; do not invent tool/API names.
- ~/.agents/skills/golang-patterns/SKILL.md and golang-testing/SKILL.md for the server; database-design/database-migrations for catalog changes.
- ~/.agents/skills/agent-harness-construction/SKILL.md for tool/observation integration; agent-architecture-audit for cursor/continuity/cancellation review.
- ~/.agents/skills/impeccable/SKILL.md, react-patterns/react-testing, and the installed frontend-testing-debugging skill for UI composition and rendered validation.
- Installed WinUI design/dev-workflow/packaging skills and windows-desktop-e2e for the native control center.
- ~/.agents/skills/docker-patterns/SKILL.md and github-actions-efficiency for packaging/workflow changes; unslop for documentation.
Use only skills that fit the current task. If delegation is available and authorized by applicable instructions, give bounded disjoint file ownership, exact contracts and expected verification; never delegate writing replacement LLM instructions.

Important fixed decisions: Go with Eino ADK; current React foundation; MCAP raw + SQLite; direct local/LAN dashboard on8081 and UDP20777. There are no accounts/login/pairing/application tokens/Origin checks/allowlists/mandatory certificates. Do not add those mechanisms. Keep input validation, server-side credentials/redaction and recording recovery. Windows11 x64 bundles the native server, starts/opens automatically and continues in tray. Optional voice uses native Windows capture. Setup work is recommendations only, with no game control.

Integrate docs/rework/prompts/v1 byte-for-byte with its manifest, exact tools and schemas. Those system instructions were already authored for this product. Do not generate/rewrite them, invent private chain-of-thought storage, or replace Eino with Codex/Hermes. If a prompt evaluation fails, preserve the authored version and report a concrete proposed revision for review. Software owns scheduling and notifications. Separate dispatch and accepted-evidence cursors; cancellation must not consume pending Safety Car/weather evidence.

Completion gate: all implementation tasks and required qualification gates A–E have passing evidence, schemas/prompts/mockup requirements match implementation, supported artifacts are built, final-report.md is present, and a skeptical final review has no unresolved material findings. Then mark the goal complete and end the turn. Do not add work after this gate, rerun unchanged passing suites, refactor unrelated code, or keep polishing.

Blocked exit gate: if a real provider/key, Windows/game/microphone host, real2026 capture or required human decision is unavailable, complete all independent implementation/checks, record the exact failed/unavailable gate and smallest next action, and stop claiming further progress on that gate. Never mark it passed with a fake model/cross-build. If the goal tool requires three consecutive blocked turns, use one concise unchanged-blocker audit per continuation and mark the goal blocked at the third. Do not repeat network calls/builds or sleep/poll to manufacture progress. A difficult bug is not an external blocker: reproduce, fix and verify it; if a budget ends first, leave an accurate incomplete checkpoint without marking complete.

Final response: state implementation status, link final-report.md and artifacts, list executed validation and explicit pending gates. Stop for my review. No automatic publishing or OpenSpec archiving.
~~~
