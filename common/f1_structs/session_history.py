"""
F1 25 UDP Telemetry — Session History Packet (1460 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Lap history data for one lap
LAP_HISTORY_DATA = {
    "m_lapTimeInMS": "I",                    # uint32 - Lap time in milliseconds
    "m_sector1TimeMSPart": "H",              # uint16 - Sector 1 milliseconds part
    "m_sector1TimeMinutesPart": "B",         # uint8  - Sector 1 whole minute part
    "m_sector2TimeMSPart": "H",              # uint16 - Sector 2 time milliseconds part
    "m_sector2TimeMinutesPart": "B",         # uint8  - Sector 2 whole minute part
    "m_sector3TimeMSPart": "H",              # uint16 - Sector 3 time milliseconds part
    "m_sector3TimeMinutesPart": "B",         # uint8  - Sector 3 whole minute part
    "m_lapValidBitFlags": "B",               # uint8  - 0x01 = lap valid, 0x02 = sector 1 valid,
                                             #          0x04 = sector 2 valid, 0x08 = sector 3 valid
}

# Tyre stint history data
TYRE_STINT_HISTORY_DATA = {
    "m_endLap": "B",                         # uint8  - Lap the tyre usage ends on (255 if current tyre)
    "m_tyreActualCompound": "B",             # uint8  - Actual tyres used by this driver
    "m_tyreVisualCompound": "B",             # uint8  - Visual tyres used by this driver
}

# Packet-level leading fields (after header, before lap history array)
PACKET_SESSION_HISTORY_HEAD = {
    "m_carIdx": "B",                         # uint8  - Index of the car this lap data relates to
    "m_numLaps": "B",                        # uint8  - Num laps in the data (including current partial lap)
    "m_numTyreStints": "B",                  # uint8  - Number of tyre stints in the data
    "m_bestLapTimeLapNum": "B",              # uint8  - Lap the best lap time was achieved on
    "m_bestSector1LapNum": "B",              # uint8  - Lap the best Sector 1 time was achieved on
    "m_bestSector2LapNum": "B",              # uint8  - Lap the best Sector 2 time was achieved on
    "m_bestSector3LapNum": "B",              # uint8  - Lap the best Sector 3 time was achieved on
}

# PacketSessionHistoryData: PACKET_HEADER + PACKET_SESSION_HISTORY_HEAD
#                           + LAP_HISTORY_DATA[100] + TYRE_STINT_HISTORY_DATA[8]
