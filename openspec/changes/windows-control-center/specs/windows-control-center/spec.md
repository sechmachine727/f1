# Windows control center and optional voice

## Purpose

Let Windows11 x64 users start and operate the telemetry package without Docker, with reliable tray lifecycle and optional native push-to-talk voice.

## ADDED Requirements

### Requirement: Start without technical setup

The system SHALL distribute an unpackaged self-contained Windows11 x64 control center bundling the native server/UI and SHALL start/open the dashboard automatically.

#### Scenario: Fresh supported Windows machine

- **WHEN** the user extracts the portable ZIP and launches the app
- **THEN** no Docker/.NET installation, account, pairing or certificate workflow is required

#### Scenario: Port conflict

- **WHEN** another unrelated process owns the selected port
- **THEN** the app reports the conflict and offers configuration without terminating that process

### Requirement: Continue in tray and flush deliberately

The system SHALL continue server/recording operation when the control-center window closes, and explicit Stop/Exit SHALL flush recordings before shutdown.

#### Scenario: Close window

- **WHEN** the user closes the main window during recording
- **THEN** the tray remains available and recording continues

#### Scenario: Flush timeout

- **WHEN** the owned server does not complete within10seconds
- **THEN** the app offers retry or an explicit force action with possible data-loss explanation; it does not silently kill

### Requirement: Supervise only the owned server

The system SHALL avoid duplicate owned instances and SHALL identify the managed child before adopting, stopping or reporting it.

#### Scenario: Second launch

- **WHEN** a matching control center is already running
- **THEN** the existing instance is activated instead of starting another server

#### Scenario: Unrelated health endpoint

- **WHEN** an unrelated service returns on the configured port
- **THEN** it is not adopted or stopped as the telemetry child

### Requirement: Capture optional native voice

The system SHALL support optional native microphone push-to-talk while the game has focus, with configurable transcription/speech endpoints and text fallback.

#### Scenario: LAN dashboard on HTTP

- **WHEN** voice is enabled in the native control center
- **THEN** capture/playback operates without browser microphone or TLS setup

#### Scenario: Key/microphone unavailable

- **WHEN** the binding conflicts or microphone access fails
- **THEN** the app shows the actionable error and text requests remain usable

### Requirement: Bound voice turns

The system SHALL expose recording/transcription/thinking/speaking states, cap utterances and permit interruption without retaining default raw audio history.

#### Scenario: PTT while speaking

- **WHEN** the user starts another utterance
- **THEN** old playback stops and the new recording is visibly active

#### Scenario: Utterance exceeds30seconds

- **WHEN** the user keeps holding PTT
- **THEN** capture stops at the limit, reports it and creates no endless audio stream

### Requirement: Keep operation understandable

The system SHALL show actual server/telemetry/recording/storage states, local/LAN dashboard addresses and accessible basic controls with advanced settings separated.

#### Scenario: No game data

- **WHEN** the bundled server is running but waiting for UDP
- **THEN** status says waiting and shows the exact game host/port settings rather than failure or nominal telemetry
