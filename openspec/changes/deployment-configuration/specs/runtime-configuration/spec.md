# Spec Delta

## Purpose

Let operators configure the telemetry package from environment variables or files, and let users see the effective configuration and its source inside the web and native applications.

## ADDED Requirements

### Requirement: Environment overrides file and default configuration

The system SHALL resolve each configurable value in the order command line, environment, configuration file, embedded default. An environment value SHALL take effect without editing files inside a container or application bundle.

#### Scenario: UDP listener set from the environment
- **WHEN** `F1TELE_UDP_ADDR=0.0.0.0:30777` is set and the container starts
- **THEN** health and Settings report UDP port 30777 and the source as environment

#### Scenario: Command line still wins
- **WHEN** both `F1TELE_HTTP_ADDR` and `--http` are supplied
- **THEN** the command-line value is effective and its source is reported as command line

### Requirement: Deployment values are available as documented environment variables

The system SHALL expose environment variables for HTTP and UDP listeners, data directory, recording enabled, cap, free reserve, chunk size, flush interval, rotation size, agent endpoint URL, model, protocol and key, search provider and key, and voice endpoint and key. Names SHALL be stable and documented in one operator reference.

#### Scenario: Dashboard and game on different machines
- **WHEN** the package runs on one machine and the game sends UDP from another
- **THEN** the operator can bind UDP to a reachable interface and the dashboard reports the LAN address the game should use

#### Scenario: Recording cap changed in Docker
- **WHEN** `F1TELE_RECORDING_CAP_BYTES` is set
- **THEN** existing recordings are preserved and new writes stop at the configured cap

### Requirement: Invalid environment values fail before serving

The system SHALL validate environment values with the same rules as file values and SHALL refuse to start on an invalid value, naming the variable and the expected form. It SHALL NOT silently substitute a default.

#### Scenario: Malformed listener
- **WHEN** `F1TELE_UDP_ADDR` has no port
- **THEN** startup fails with an error naming the variable and the required `host:port` form

#### Scenario: Negative cap
- **WHEN** a recording cap is negative
- **THEN** startup fails instead of disabling or clamping recording silently

### Requirement: Secrets are write-only in every surface

The system SHALL read a secret from its configured environment variable or the local secret store, SHALL never return it through the configuration API or render it in the web or native UI, and SHALL report only whether it is configured. Logs SHALL redact secret values.

#### Scenario: Configured API key
- **WHEN** Settings is opened with an agent key present in the environment
- **THEN** the agent row shows configured, its source, and never the key value

#### Scenario: Missing key
- **WHEN** an endpoint is enabled but its key variable is empty
- **THEN** the dependent feature reports unavailable with the missing variable name

### Requirement: Web Settings reports effective values and restart requirements

The web Settings workspace SHALL be a read-only report of every effective value, its source, and whether changing it requires a restart. It SHALL show listeners, data directory, recording limits, agent endpoint and model, search, voice, and feature availability. It SHALL NOT be the configuration surface for the Windows portable package.

#### Scenario: Live-reloadable value
- **WHEN** an agent endpoint changes through the configuration API
- **THEN** Settings marks it live and the next invocation uses it without a restart

#### Scenario: Restart-only value
- **WHEN** the UDP bind address changes
- **THEN** Settings marks it restart-required and keeps reporting the running value until restart

### Requirement: Native settings surface mirrors effective configuration

The Windows control center's Settings page SHALL be the configuration surface for the portable Windows shipment. It SHALL read and write the app's local configuration, show the running server address, UDP listener, data directory, recording limits, agent configuration state, and configuration file location, and keep secrets out of the UI. The web Settings workspace SHALL only report these effective values.

#### Scenario: Native settings opened
- **WHEN** the user opens Settings in the control center
- **THEN** the app shows the running server address, UDP listener, data directory, recording limits, agent configuration state, and configuration file path

#### Scenario: Windows configuration is edited natively
- **WHEN** the user changes a local setting in the Windows control center
- **THEN** the app updates its own configuration and the web Settings report reflects the new effective value without becoming the editing surface

#### Scenario: Change requires restart
- **WHEN** a displayed value is restart-only
- **THEN** the surface states that the server must be restarted and does not imply the change is active

### Requirement: Defaults remain usable with no environment set

The system SHALL start with the embedded defaults and no environment variables, preserving the direct local dashboard on 8081 and UDP 20777. Environment configuration SHALL be an override path, not a requirement.

#### Scenario: First run without configuration
- **WHEN** the package starts with an empty environment and no configuration file
- **THEN** it serves the dashboard on 8081, listens on UDP 20777, and reports default sources in Settings
