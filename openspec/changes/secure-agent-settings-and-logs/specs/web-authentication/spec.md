# Spec Delta

## Purpose

Protect the local telemetry dashboard, agent controls and recorded data with a password session while preserving a public health probe for the native supervisor.

## ADDED Requirements

### Requirement: Dashboard data requires authentication

The system SHALL require an authenticated session for every dashboard workspace, API route, SSE stream and WebSocket except the health probe and the login and authentication-status routes.

#### Scenario: Unauthenticated data request
- **WHEN** a client requests a dashboard data route without a valid session
- **THEN** the server returns 401 with a typed error and the UI shows the login screen

#### Scenario: Authenticated data request
- **WHEN** a client presents a valid session cookie
- **THEN** the requested workspace, API route or stream behaves as before

### Requirement: Origin validation is enabled by default

The system SHALL reject requests whose Origin host differs from the request Host when Origin validation is enabled. Requests without Origin SHALL remain usable by non-browser clients. The setting SHALL be enabled by default.

#### Scenario: Cross-origin API request
- **WHEN** a browser sends an API request with an Origin host that differs from Host
- **THEN** the server returns 403 without reading or changing application data

#### Scenario: Same-origin request
- **WHEN** the Origin host equals the request Host
- **THEN** the request proceeds to authentication and routing

### Requirement: First-run password lifecycle

The system SHALL use an environment password when present. Otherwise, when no stored password exists, it SHALL generate a random password, store only its salted hash, and print the plaintext once during that first start. Later starts SHALL NOT print a password.

#### Scenario: Fresh Docker data directory
- **WHEN** the server starts with an empty data directory and no environment password
- **THEN** it writes the password hash and prints the generated password once to the process log

#### Scenario: Existing password
- **WHEN** the server restarts with an existing password file
- **THEN** it prints no password and accepts only the stored or environment password

### Requirement: Sessions expire and invalidate on reset

The system SHALL issue a signed HttpOnly SameSite=Strict session valid for 30 days. Logout SHALL clear it. Password reset SHALL rotate the session-signing key and invalidate every previously issued session.

#### Scenario: Password reset with an active session
- **WHEN** an operator resets the password while a browser is logged in
- **THEN** the next protected request from that browser returns 401

#### Scenario: Logout
- **WHEN** the user selects Log out
- **THEN** the cookie is cleared and the login screen appears

### Requirement: Password replacement requires no previous password

The system SHALL provide a command that sets a new password interactively with confirmation or from standard input without confirmation. It SHALL refuse the reset while an environment password owns authentication.

#### Scenario: Piped reset
- **WHEN** a new password is supplied on standard input with the stdin flag
- **THEN** the stored password is replaced, the signing key rotates, and no re-entry is required

#### Scenario: Environment-owned password
- **WHEN** the authentication environment variable is set
- **THEN** the reset command exits non-zero and explains that the environment value must be removed first

### Requirement: Login attempts are throttled

The system SHALL limit repeated failed login attempts per source address and SHALL return a generic authentication error without revealing whether a password is configured.

#### Scenario: Repeated wrong password
- **WHEN** one source exceeds the configured failure rate
- **THEN** further attempts receive a rate-limit response until the window expires

### Requirement: Health remains available

The public health probe SHALL remain available without a session for the native supervisor and SHALL be subject to Origin validation when enabled.

#### Scenario: Native supervisor probe
- **WHEN** the native control center requests health without Origin or cookies
- **THEN** the server returns health without requiring login
