"""
F1 25 UDP Telemetry — Lap Data Packet (1285 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Lap data about one car
LAP_DATA = {
    "m_lastLapTimeInMS": "I",                # uint32 - Last lap time in milliseconds
    "m_currentLapTimeInMS": "I",             # uint32 - Current time around the lap in milliseconds
    "m_sector1TimeMSPart": "H",              # uint16 - Sector 1 time milliseconds part
    "m_sector1TimeMinutesPart": "B",         # uint8  - Sector 1 whole minute part
    "m_sector2TimeMSPart": "H",              # uint16 - Sector 2 time milliseconds part
    "m_sector2TimeMinutesPart": "B",         # uint8  - Sector 2 whole minute part
    "m_deltaToCarInFrontMSPart": "H",        # uint16 - Time delta to car in front milliseconds part
    "m_deltaToCarInFrontMinutesPart": "B",   # uint8  - Time delta to car in front whole minute part
    "m_deltaToRaceLeaderMSPart": "H",        # uint16 - Time delta to race leader milliseconds part
    "m_deltaToRaceLeaderMinutesPart": "B",   # uint8  - Time delta to race leader whole minute part
    "m_lapDistance": "f",                    # float  - Distance vehicle is around current lap in metres (can be negative)
    "m_totalDistance": "f",                  # float  - Total distance travelled in session in metres (can be negative)
    "m_safetyCarDelta": "f",                # float  - Delta in seconds for safety car
    "m_carPosition": "B",                   # uint8  - Car race position
    "m_currentLapNum": "B",                 # uint8  - Current lap number
    "m_pitStatus": "B",                     # uint8  - 0 = none, 1 = pitting, 2 = in pit area
    "m_numPitStops": "B",                   # uint8  - Number of pit stops taken in this race
    "m_sector": "B",                        # uint8  - 0 = sector1, 1 = sector2, 2 = sector3
    "m_currentLapInvalid": "B",             # uint8  - Current lap invalid - 0 = valid, 1 = invalid
    "m_penalties": "B",                     # uint8  - Accumulated time penalties in seconds to be added
    "m_totalWarnings": "B",                 # uint8  - Accumulated number of warnings issued
    "m_cornerCuttingWarnings": "B",         # uint8  - Accumulated number of corner cutting warnings issued
    "m_numUnservedDriveThroughPens": "B",   # uint8  - Num drive through pens left to serve
    "m_numUnservedStopGoPens": "B",         # uint8  - Num stop go pens left to serve
    "m_gridPosition": "B",                  # uint8  - Grid position the vehicle started the race in
    "m_driverStatus": "B",                  # uint8  - 0 = in garage, 1 = flying lap, 2 = in lap, 3 = out lap, 4 = on track
    "m_resultStatus": "B",                  # uint8  - 0 = invalid, 1 = inactive, 2 = active, 3 = finished, 4 = DNF, 5 = DSQ, 6 = not classified, 7 = retired
    "m_pitLaneTimerActive": "B",            # uint8  - Pit lane timing, 0 = inactive, 1 = active
    "m_pitLaneTimeInLaneInMS": "H",         # uint16 - If active, the current time spent in the pit lane in ms
    "m_pitStopTimerInMS": "H",              # uint16 - Time of the actual pit stop in ms
    "m_pitStopShouldServePen": "B",         # uint8  - Whether the car should serve a penalty at this stop
    "m_speedTrapFastestSpeed": "f",         # float  - Fastest speed through speed trap for this car in kmph
    "m_speedTrapFastestLap": "B",           # uint8  - Lap no the fastest speed was achieved, 255 = not set
}

# Packet-level trailing fields (after LAP_DATA[22])
PACKET_LAP_DATA_TAIL = {
    "m_timeTrialPBCarIdx": "B",             # uint8  - Index of Personal Best car in time trial (255 if invalid)
    "m_timeTrialRivalCarIdx": "B",          # uint8  - Index of Rival car in time trial (255 if invalid)
}

# PacketLapData: PACKET_HEADER + LAP_DATA[22] + PACKET_LAP_DATA_TAIL
