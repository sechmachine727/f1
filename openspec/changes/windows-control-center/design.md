# Windows control center and optional voice design

## Context

WinUI controls server lifecycle; the browser remains the full telemetry UI. Use native microphone/audio to avoid browser secure-context setup on a LAN HTTP dashboard. No Windows service is needed.

## Goals / Non-Goals

Goals: extract-and-launch operation, tray continuation, accessible diagnostics and optional voice. Non-goals: Docker management, accounts/pairing, automatic updates or game input automation.

## Decisions

Use .NET10LTS, target net10.0-windows10.0.26100.0, minimum Windows10.0.22000.0 (Windows11), WinUI3 Microsoft.WindowsAppSDK1.8.260921001. Pin NuGet versions/lockfile and record the actual .NET10 SDK patch used. Create windows/F1Telemetry.ControlCenter with App/MainWindow, ServerSupervisor, TrayController, VoiceController, SettingsStore and focused tests. Unpackaged self-contained win-x64 output includes runtime dependencies and sibling f1tele.exe; use a ZIP, no installer/service/autostart.

Single instance uses AppInstance activation redirection. ServerSupervisor launches the bundled absolute executable with explicit data/config paths under LOCALAPPDATA/F1Telemetry, tracks PID/executable path/start time and health instance_id. Save enough ownership metadata to reconnect after window reopening. Never adopt a server based on port alone. Readiness polling is bounded to10seconds then reports error; do not repeatedly restart a failing server. On success open the system default browser at127.0.0.1:8081 and display LAN addresses. Connection diagnostics distinguish UDP silence, wrong target and port bind error; any OS firewall prompt is handled with plain instructions, no automatic firewall reconfiguration.

Use Win32 Shell_NotifyIcon/NotifyIcon interaction through a narrow native boundary for Open dashboard, Show control center, Start/Stop server and Exit. Closing hides to tray and announces that status once. Explicit Stop POSTs control/stop and waits for process exit/flush acknowledgment.10second grace offers Retry or explicit Force stop for the owned child. A server crash preserves its exit code/log path and recording recovery status; restarting is a user action.

VoiceController uses Windows.Media.Capture.MediaCapture to capture WAV PCM16-bit mono16kHz; set/validate the audio encoding and surface unsupported device errors. RegisterHotKey binds Ctrl+Alt+Space by default; while held, a short-lived16ms key-state check detects release, and a30second cap always stops capture. No persistent polling outside an utterance. Confirm game-focus behavior on the supported Windows/game environment; conflicts offer rebinding and text fallback. Send bounded WAV to the local server transcription route; the server calls the configured OpenAI-compatible audio/transcriptions endpoint using multipart model/file. The transcript becomes a live_question request only after capture ends. MediaPlayer plays server-returned WAV from audio/speech (model/input/voice/response_format=wav). Provider keys stay server-side.

One voice controller owns microphone/playback. PTT interrupts prior speech; mute suppresses playback only. UI exposes Idle/Recording/Transcribing/Thinking/Speaking/Error. Raw audio is transient and deleted after the request by default; transcript/invocation remains in the audit. Voice disabled or unavailable never blocks text/telemetry. Browser microphone support, if exposed, follows browser restrictions and remains optional.

## Risks / Trade-offs

Portable dependencies may be incomplete -> clean-VM launch test. Hotkey in exclusive games -> actual game-focus test and explicit binding limitation. Flush races -> supervisor fake-child tests and crash/recovery tests. Audio driver variation -> format/permission/failure tests plus real microphone evidence.

## Migration Plan

Package alongside the Go distribution only after browser/v1 parity. Preserve user data on app replacement; no silent schema downgrade. Keep previous ZIP/config snapshot for rollback without deleting recordings.
