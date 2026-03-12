"""
F1 25 UDP Telemetry — Tyre Sets Packet (231 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Data about one tyre set
TYRE_SET_DATA = {
    "m_actualTyreCompound": "B",             # uint8  - Actual tyre compound used
    "m_visualTyreCompound": "B",             # uint8  - Visual tyre compound used
    "m_wear": "B",                           # uint8  - Tyre wear (percentage)
    "m_available": "B",                      # uint8  - Whether this set is currently available
    "m_recommendedSession": "B",             # uint8  - Recommended session for tyre set, see appendix
    "m_lifeSpan": "B",                       # uint8  - Laps left in this tyre set
    "m_usableLife": "B",                     # uint8  - Max number of laps recommended for this compound
    "m_lapDeltaTime": "h",                   # int16  - Lap delta time in milliseconds compared to fitted set
    "m_fitted": "B",                         # uint8  - Whether the set is fitted or not
}

# Packet-level fields
PACKET_TYRE_SETS_HEAD = {
    "m_carIdx": "B",                         # uint8  - Index of the car this data relates to
}

PACKET_TYRE_SETS_TAIL = {
    "m_fittedIdx": "B",                      # uint8  - Index into array of fitted tyre
}

# PacketTyreSetsData: PACKET_HEADER + PACKET_TYRE_SETS_HEAD + TYRE_SET_DATA[20] + PACKET_TYRE_SETS_TAIL
