"""
F1 25 UDP Telemetry — Packet Header (29 bytes).
F1 25 UDP Telemetry — Global constants.

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

CONSTANTS = {
    "cs_maxNumCarsInUDPData": 22,                      # Maximum number of cars in UDP data arrays
    "cs_maxParticipantNameLen": 32,                    # Max participant name length (UTF-8, null-terminated)
    "cs_maxTyreStints": 8,                             # Maximum number of tyre stints tracked per car
    "cs_maxNumTyreSets": 20,                           # 13 slick + 7 wet weather
    "cs_maxMarshalsZonePerLap": 21,                    # Maximum marshal zones per lap
    "cs_maxWeatherForecastSamples": 64,                # Maximum weather forecast samples
    "cs_maxSessionsInWeekend": 12,                     # Maximum sessions in a weekend
    "cs_maxNumLapsInHistory": 100,                     # Maximum laps in session history
    "cs_eventStringCodeLen": 4,                        # Event string code length
    "cs_maxNumLapsInLapPositionsHistoryPacket": 50,    # Maximum laps in lap-positions history packet
}

# Different packet types
PACKET_ID = {
    "ePacketIdMotion": 0,                # Contains all motion data for player's car - only sent while player is in control
    "ePacketIdSession": 1,               # Data about the session - track, time left
    "ePacketIdLapData": 2,               # Data about all the lap times of cars in the session
    "ePacketIdEvent": 3,                 # Various notable events that happen during a session
    "ePacketIdParticipants": 4,          # List of participants in the session, mostly relevant for multiplayer
    "ePacketIdCarSetups": 5,             # Packet detailing car setups for cars in the race
    "ePacketIdCarTelemetry": 6,          # Telemetry data for all cars
    "ePacketIdCarStatus": 7,             # Status data for all cars
    "ePacketIdFinalClassification": 8,   # Final classification confirmation at the end of a race
    "ePacketIdLobbyInfo": 9,             # Information about players in a multiplayer lobby
    "ePacketIdCarDamage": 10,            # Damage status for all cars
    "ePacketIdSessionHistory": 11,       # Lap and tyre data for session
    "ePacketIdTyreSets": 12,             # Extended tyre set data
    "ePacketIdMotionEx": 13,             # Extended motion data for player car
    "ePacketIdTimeTrial": 14,            # Time Trial specific data
    "ePacketIdLapPositions": 15,         # Lap positions on each lap so a chart can be constructed
}

# Header - 29 bytes
PACKET_HEADER = {
    "m_packetFormat": "H",               # uint16 - 2025
    "m_gameYear": "B",                   # uint8  - Game year - last two digits e.g. 25
    "m_gameMajorVersion": "B",           # uint8  - Game major version - "X.00"
    "m_gameMinorVersion": "B",           # uint8  - Game minor version - "1.XX"
    "m_packetVersion": "B",              # uint8  - Version of this packet type
    "m_packetId": "B",                   # uint8  - Identifier for the packet type
    "m_sessionUID": "Q",                 # uint64 - Unique identifier for the session
    "m_sessionTime": "f",                # float  - Session timestamp
    "m_frameIdentifier": "I",            # uint32 - Identifier for the frame the data was retrieved on
    "m_overallFrameIdentifier": "I",     # uint32 - Overall identifier for the frame, doesn't go back after flashbacks
    "m_playerCarIndex": "B",             # uint8  - Index of player's car in the array
    "m_secondaryPlayerCarIndex": "B",    # uint8  - Index of secondary player's car (splitscreen) - 255 if no second player
}
