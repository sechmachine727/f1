# Spec Delta

## ADDED Requirements

### Requirement: Drive graph slots are configurable

Drive SHALL render three graph slots. Each slot SHALL offer a metric selector and a time-range selector, and changing one slot SHALL NOT change another. The default slots SHALL show lap time, worst tyre wear and gap ahead.

#### Scenario: Switch a slot to ERS
- **WHEN** the driver selects ERS store in the first slot
- **THEN** only that slot changes, and the other two keep their metrics

#### Scenario: Defaults
- **WHEN** Drive opens with no previous selections
- **THEN** the slots show lap time, worst tyre wear and gap ahead

### Requirement: Graphs label both axes

Each Drive graph SHALL label the horizontal axis as time and the vertical axis with the selected metric's unit. It SHALL show at least three time ticks ending at the newest sample and, when samples exist, at least three value ticks, with text readable at 1920x1080 and 2560x1440.

#### Scenario: ERS graph
- **WHEN** ERS store is selected
- **THEN** the vertical axis is labelled MJ and the horizontal ticks show the selected time range

#### Scenario: Range change
- **WHEN** the range changes from 2 minutes to 30 seconds
- **THEN** the time ticks and the visible window update without changing the vertical metric

### Requirement: Every supported metric is sampled continuously

Drive SHALL sample every continuously available metric at 1 Hz whether or not a slot selects it, and SHALL record completed lap times when the catalogue reports them. It SHALL retain the most recent ten minutes per series. The supported set SHALL include lap time, worst tyre wear, gap ahead, gap to leader, ERS store, fuel mass, engine temperature, tyre surface temperature, tyre core temperature, speed, throttle, brake and brake temperature. An unavailable sample SHALL be stored as a gap.

#### Scenario: Metric selected mid-stint
- **WHEN** the driver switches a slot to fuel mass after several minutes
- **THEN** the graph shows the fuel history already sampled, not an empty series

#### Scenario: Unavailable channel
- **WHEN** a selected metric has no available value for part of the window
- **THEN** the line breaks at the gap and the header states the channel is unavailable

### Requirement: Range changes do not reset data

Selecting a range SHALL only change the visible window. It SHALL NOT clear samples, restart a series or shorten retention. Samples SHALL be removed only when older than the ten-minute maximum. The range options SHALL be 30 seconds, 1 minute, 2 minutes, 5 minutes and 10 minutes, and the default SHALL be 2 minutes.

#### Scenario: Short range then long range
- **WHEN** the driver views 30 seconds and then selects 10 minutes
- **THEN** the graph shows the older retained samples instead of an empty or restarted series

#### Scenario: Metric round trip
- **WHEN** the driver switches away from a metric and back
- **THEN** the previously sampled history is still present

#### Scenario: Retention boundary
- **WHEN** samples pass ten minutes of age
- **THEN** only those older samples are discarded

### Requirement: Graphs follow the newest sample

The visible window SHALL end at the newest sample and advance as new samples arrive. When telemetry stops, the window SHALL hold at the last sample and the graph SHALL keep showing the frozen values.

#### Scenario: Live growth
- **WHEN** new samples arrive during a stint
- **THEN** the window advances and the newest point is at the right edge

#### Scenario: Feed stops
- **WHEN** the game pauses and no packets arrive
- **THEN** the graph stops advancing and stays at the last sample

