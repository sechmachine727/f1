# Spec Delta

## Purpose

Lets an operator inspect one or two cars during a live session with the same evidence the driver sees, without losing the standing table or the status colour meanings.

## ADDED Requirements

### Requirement: Every driver row opens live detail

Selecting a row in the live Pit wall SHALL open a detail view for that car, including the player's row. The detail SHALL show the selected car's live speed, gear, throttle and brake, ERS energy and deployment state, fuel margin, tyre wear and temperatures, damage, position, gaps, current stint and recent laps.

#### Scenario: Rival selected
- **WHEN** the operator selects a rival row while telemetry is live
- **THEN** the detail shows that car's values and names the driver, team and compound

#### Scenario: Player selected
- **WHEN** the operator selects the row marked You
- **THEN** the detail shows values matching the Drive workspace for the same car and moment

### Requirement: Missing and restricted values are never zero

The detail SHALL label stale, restricted, unsupported and not-received values with the contract state instead of rendering zero, and SHALL show the unavailable state for a channel that has never arrived.

#### Scenario: Restricted rival channel
- **WHEN** the sender restricts a rival's ERS or fuel channel
- **THEN** the detail shows the channel as Restricted and renders no numeric zero

#### Scenario: Feed stops
- **WHEN** the car's packets stop arriving
- **THEN** the detail marks the affected values No update and keeps the panel present

### Requirement: Two drivers can be compared

The live Pit wall SHALL allow at most two cars to be selected for comparison, including the player. Selecting a third car SHALL replace the earliest selection. The table SHALL show which rows are selected.

#### Scenario: Second car added
- **WHEN** the operator selects a second car
- **THEN** both cars appear in the comparison with their own identity colour

#### Scenario: Third car selected
- **WHEN** two cars are already selected and the operator selects another
- **THEN** the oldest selection is replaced and two cars remain selected

### Requirement: Team colours identify cars, status colours describe state

Each selected car SHALL carry an identity colour derived from its team, adjusted for contrast on the dark surface, with a neutral fallback for unknown teams and custom entries. Identity colour SHALL be used only for names, markers, borders and series lines. Cold, normal, caution and critical values SHALL keep the existing blue, green, orange and red treatment with a word or symbol.

#### Scenario: Two teams compared
- **WHEN** cars from different teams are selected
- **THEN** their headers, markers and trace lines use their team colours and remain distinguishable

#### Scenario: Unknown team
- **WHEN** a car reports no usable team identity
- **THEN** it uses the neutral fallback without borrowing a status colour

### Requirement: Compared values show deltas

For each numeric value both selected cars report as available, the comparison SHALL show the difference with the direction stated for that metric. It SHALL omit the delta and name the unavailable car when either value is unavailable.

#### Scenario: Speed delta
- **WHEN** both cars report speed
- **THEN** the comparison shows the difference and which car is faster

#### Scenario: One value unavailable
- **WHEN** one car's tyre wear is restricted
- **THEN** the comparison shows no delta and names the unavailable side

### Requirement: Recorded selection never mixes live data

When a recorded session is open, selecting a row SHALL keep every value and chart sourced from that session's catalogue, and SHALL NOT show live stream values.

#### Scenario: Recorded row selected during a live session
- **WHEN** the operator selects a row in a recorded Pit wall while telemetry is live
- **THEN** the detail and charts show recorded evidence only

