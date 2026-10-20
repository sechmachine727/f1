# Spec Delta

## Purpose

Gives every analysis chart one consistent time-based inspection model, so a player can read within-lap inputs and energy use across a full race instead of only a compressed overview.

## ADDED Requirements

### Requirement: Time-axis input traces

The recorded Pit wall and Analysis SHALL show a shared time axis for throttle, brake, gear and ERS store energy for the selected cars and laps. Samples SHALL use the session clock, and channels that did not arrive SHALL leave a gap rather than a zero line.

#### Scenario: One lap selected
- **WHEN** a lap with samples is selected
- **THEN** throttle and brake render from 0 to 100 percent, gear renders as discrete steps, and ERS renders as stored energy

#### Scenario: Channel never arrived
- **WHEN** a selected car has no ERS samples for the lap
- **THEN** the ERS panel states the channel is unavailable and no line is drawn

### Requirement: Energy state is shown as a labelled band

The ERS chart SHALL mark deployment state over time with a band labelled in the legend and tooltip. 2025 sessions SHALL distinguish none, medium, hotlap and overtake. 2026 sessions SHALL distinguish none, medium, hotlap and boost, and SHALL show overtake unavailable, available and active as a separate state. Unknown seasons and restricted cars SHALL use a neutral band labelled as such.

#### Scenario: 2025 push phase
- **WHEN** a 2025 lap includes the highest deployment mode
- **THEN** the band shows an overtake segment and the legend names it

#### Scenario: 2026 boost and overtake availability
- **WHEN** a 2026 car is on boost and later has overtake available without activating it
- **THEN** the band shows the boost segment and the availability state changes separately

#### Scenario: Unknown regulation season
- **WHEN** the session's regulation season is unknown
- **THEN** the band stays neutral and states that the state is unclassified

### Requirement: Charts zoom and pan

Every analysis chart SHALL start at 100 percent, showing the complete selected range. It SHALL support zooming with the pointer wheel or pinch, zoom buttons, and keyboard keys, and panning by drag, scroll and arrow keys. It SHALL show the visible range and current zoom, and one action SHALL restore the full range.

#### Scenario: Race zoomed to a lap
- **WHEN** the operator zooms into the middle of a race
- **THEN** the visible range narrows around the pointer or centre, the range readout updates, and the chart does not reorder or lose samples

#### Scenario: Reset
- **WHEN** the operator resets the chart
- **THEN** the full range is shown at 100 percent

#### Scenario: Empty range
- **WHEN** the selected data is shorter than the visible window
- **THEN** the chart shows the available range without inventing padding samples

### Requirement: Live charts follow the latest sample

A live chart SHALL follow incoming samples while its window is at the end. Panning away SHALL turn following off and keep the inspected window still. A follow-latest control SHALL return to the newest sample and resume following.

#### Scenario: Operator pans back
- **WHEN** the operator pans to an earlier moment during a live session
- **THEN** new samples do not move the window and the follow control is offered

#### Scenario: Follow restored
- **WHEN** the operator activates follow
- **THEN** the window jumps to the latest sample and then tracks new samples

### Requirement: Charts stay readable at race length

A chart SHALL reduce the rendered points to the visible plot width while preserving local extremes, so brake applications and gear changes remain visible at full-race scale. Zooming in SHALL reveal the stored detail rather than a smoothed approximation.

#### Scenario: Full race view
- **WHEN** a two-hour race is shown at 100 percent
- **THEN** the chart renders a bounded number of points and still shows every braking zone as a visible dip or rise

#### Scenario: Zoomed detail
- **WHEN** the operator zooms to a few seconds
- **THEN** individual samples are plotted at their stored times

### Requirement: Pointer inspection states exact values

The chart SHALL show a crosshair and a tooltip with time, throttle, brake, gear, ERS energy and the deployment or overtake state for the nearest sample, identifying each selected car.

#### Scenario: Hover between samples
- **WHEN** the pointer is between two samples
- **THEN** the tooltip uses the nearer sample and states its time

### Requirement: Chart states are explicit

Charts SHALL distinguish waiting for telemetry, no completed laps, loading, unavailable channel, restricted channel and an unsupported season, and SHALL keep the surrounding layout stable while doing so.

#### Scenario: Session just started
- **WHEN** the live session has no completed lap
- **THEN** the charts state that the first completed lap will populate them

#### Scenario: Recorded session without samples
- **WHEN** a recording predates sample capture and backfill has not finished
- **THEN** the chart states that samples are being derived and the lap aggregates remain available

### Requirement: Chart interaction is keyboard accessible

Zoom, pan, reset and follow SHALL be operable from the keyboard with visible focus, and zoom state SHALL not rely on colour alone. Reduced-motion preferences SHALL suppress animated transitions of the window.

#### Scenario: Keyboard zoom
- **WHEN** the chart has focus and the operator presses the zoom key
- **THEN** the range changes, the readout updates and focus stays on the chart

