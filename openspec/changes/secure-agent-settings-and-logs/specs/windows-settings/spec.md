# Spec Delta

## Purpose

Give the portable Windows package a native first-run and settings experience for connection, security and password configuration while leaving agent tuning and logs in the web dashboard.

## ADDED Requirements

### Requirement: First launch opens Settings

On first launch the Windows control center SHALL show its Settings window before starting the bundled server. Password configuration SHALL appear at the top.

#### Scenario: Fresh portable install
- **WHEN** the app launches without completed first-run state
- **THEN** Settings is the first window and the server has not started

#### Scenario: Later launch
- **WHEN** first-run state is complete
- **THEN** the app starts the server with the normal control-center window

### Requirement: Generated password is shown once

When the server creates a password, the native app SHALL show it once in a code box with a copy button. It SHALL NOT show that password again on later launches.

#### Scenario: Generated password
- **WHEN** authentication has no stored or environment password
- **THEN** the app displays the generated password once during first-run Settings

### Requirement: Native Settings writes portable configuration

The native Settings window SHALL configure Origin validation, listeners, game address, data directory, recording limits, agent endpoint URL, model, protocol and API key, search and voice. Environment-owned values SHALL be read-only. Saving SHALL stop the running bundled server, write configuration and secrets, and restart it.

#### Scenario: Save while running
- **WHEN** the operator changes a listener while the server is running
- **THEN** the app stops the server, writes the change, restarts it and reports the result

#### Scenario: Environment-owned listener
- **WHEN** a listener is set in the process environment
- **THEN** the native field is read-only and identifies the environment source

### Requirement: Password reset is available without the old password

The Settings window SHALL provide a reset button that accepts a new password and confirmation without the previous password and applies it through the server authentication command.

#### Scenario: Native reset
- **WHEN** the operator confirms a new password
- **THEN** the stored password and signing key are replaced and existing web sessions are invalidated

### Requirement: Agent tuning and logs stay in web Settings

The native app SHALL link to the password-protected web Settings for agent, harness and activity-log management rather than duplicating those controls.

#### Scenario: Open agent settings
- **WHEN** the operator selects Agent settings in the native app
- **THEN** the web Settings workspace opens and requires the web login
