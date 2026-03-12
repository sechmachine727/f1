"""
F1 25 UDP Telemetry — Final Classification Packet (1042 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Data about one participant's final results
FINAL_CLASSIFICATION_DATA = {
    "m_position": "B",                      # uint8    - Finishing position
    "m_numLaps": "B",                       # uint8    - Number of laps completed
    "m_gridPosition": "B",                  # uint8    - Grid position of the car
    "m_points": "B",                        # uint8    - Number of points scored
    "m_numPitStops": "B",                   # uint8    - Number of pit stops made
    "m_resultStatus": "B",                  # uint8    - 0 = invalid, 1 = inactive, 2 = active, 3 = finished,
                                            #            4 = DNF, 5 = DSQ, 6 = not classified, 7 = retired
    "m_resultReason": "B",                  # uint8    - 0 = invalid, 1 = retired, 2 = finished, 3 = terminal damage,
                                            #            4 = inactive, 5 = not enough laps, 6 = black flagged,
                                            #            7 = red flagged, 8 = mechanical failure,
                                            #            9 = session skipped, 10 = session simulated
    "m_bestLapTimeInMS": "I",               # uint32   - Best lap time of the session in milliseconds
    "m_totalRaceTime": "d",                 # double   - Total race time in seconds without penalties
    "m_penaltiesTime": "B",                 # uint8    - Total penalties accumulated in seconds
    "m_numPenalties": "B",                  # uint8    - Number of penalties applied to this driver
    "m_numTyreStints": "B",                 # uint8    - Number of tyre stints up to maximum
    "m_tyreStintsActual": "8B",             # uint8[8] - Actual tyres used by this driver
    "m_tyreStintsVisual": "8B",             # uint8[8] - Visual tyres used by this driver
    "m_tyreStintsEndLaps": "8B",            # uint8[8] - The lap number stints end on
}

# Packet-level leading field (after header, before classification array)
PACKET_FINAL_CLASSIFICATION_HEAD = {
    "m_numCars": "B",                       # uint8    - Number of cars in the final classification
}

# PacketFinalClassificationData: PACKET_HEADER + PACKET_FINAL_CLASSIFICATION_HEAD
#                                + FINAL_CLASSIFICATION_DATA[22]
