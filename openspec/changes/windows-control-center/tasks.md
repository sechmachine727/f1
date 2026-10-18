# Windows control center and optional voice implementation tasks

Dependencies: telemetry-workspaces, scheduled-race-engineer, sourced-setup-research. Read docs/rework/{master-design,contracts,qualification}.md and this change's specs/design before editing. Commands below describe future implementation checks, not checks already run. Use the shared schema instead of inventing new wire shapes.

## 1. Native lifecycle

- [ ] 1.1 Create windows/F1Telemetry.ControlCenter project/solution with pinned WinUI/.NET target, self-contained win-x64 settings and ServerSupervisor tests for startup, unrelated-port conflict, second activation, owned-child adoption and failure. Verify on Windows: dotnet test and dotnet publish -c Release -r win-x64 --self-contained true.
- [ ] 1.2 Implement tray hide/show and explicit Stop/Exit/timeout paths with fake-child tests; run the real bundled server through recording/flush/restart. Verify a close continues recording and an unrelated process is never killed. Document the portable first-run workflow.

## 2. Native voice and accessibility

- [ ] 2.1 Implement VoiceController/Hotkey binding and server voice proxy handlers; test bounded WAV, denied microphone, missing endpoint, timeout, empty/ambiguous transcript and interrupted speech. Verify provider credentials never reach the browser or WAV audit.
- [ ] 2.2 Exercise push-to-talk while F1 has focus, release/cap behavior, playback/mute and rebinding on Windows11 x64 with a real microphone; inspect keyboard/focus/text scaling/high contrast and waiting/error states. Save video/screenshots plus exact hardware/OS/API evidence, or record the concrete external gate.

## 3. Change gate

- [ ] 3.1 Run Windows dotnet test/publish plus rtk go test -race ./server/... for voice handlers, then launch the portable ZIP on a clean Windows11 x64 VM without Docker/.NET installed. Save results in docs/rework/evidence/windows-control-center.md. A Mac cross-build of the Go child does not satisfy the native app/voice gate.

## Workflow follow-up

- Stop at this change's gate when it is the chosen handoff scope. Follow docs/rework/HANDOFF.md for the whole-program goal.
- Archive only after implementation evidence and requested review; publishing requires separate user authorization.
