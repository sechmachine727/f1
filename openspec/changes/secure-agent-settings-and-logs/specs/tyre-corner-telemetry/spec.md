# Spec Delta

## Purpose

Expose each wheel independently and make tyre wear readable as a continuous green-to-red scale.

## ADDED Requirements

### Requirement: Wheel arrays decode by corner

The system SHALL map each wheel channel to its own element in the UDP wheel array. Surface, core, pressure, wear, damage, blisters and brake channels SHALL NOT reuse the first element for all four corners.

#### Scenario: Four different wear values
- **WHEN** packet 10 reports RL10%, RR20%, FL30% and FR40% wear
- **THEN** the four canonical corner channels report those four values independently

#### Scenario: Real golden packet
- **WHEN** the checked-in 2026 golden packet 10 is decoded
- **THEN** every corner read matches the corresponding element of the decoded wheel array

### Requirement: Wear color is a continuous scale

The wear ring and the tyre-wear graph SHALL use a continuous color scale with green at 0% and red at 100%. The numeric percentage SHALL remain visible.

#### Scenario: Mid-range wear
- **WHEN** wear is 50%
- **THEN** the ring and graph use the mid-point of the green-to-red scale
