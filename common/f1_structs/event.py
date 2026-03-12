"""
F1 25 UDP Telemetry — Event Packet (45 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Valid event string codes
EVENT_CODES = {
    "SSTA": "Session Started",
    "SEND": "Session Ended",
    "FTLP": "Fastest Lap",
    "RTMT": "Retirement",
    "DRSE": "DRS Enabled",
    "DRSD": "DRS Disabled",
    "TMPT": "Team Mate In Pits",
    "CHQF": "Chequered Flag",
    "RCWN": "Race Winner",
    "PENA": "Penalty",
    "SPTP": "Speed Trap",
    "STLG": "Start Lights",
    "LGOT": "Lights Out",
    "DTSV": "Drive Through Served",
    "SGSV": "Stop Go Served",
    "FLBK": "Flashback",
    "BUTN": "Button Status",
    "RDFL": "Red Flag",
    "OVTK": "Overtake",
    "SCAR": "Safety Car",
    "COLL": "Collision",
}

# Event string code field (after header, before event details)
PACKET_EVENT_CODE = {
    "m_eventStringCode": "4s",               # char[4] - Event string code
}

# The event details union — each event type has different fields.
# Make sure only the correct type is interpreted.

EVENT_FASTEST_LAP = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of car achieving fastest lap
    "lapTime": "f",                         # float  - Lap time is in seconds
}

EVENT_RETIREMENT = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of car retiring
    "reason": "B",                          # uint8  - 0 = invalid, 1 = retired, 2 = finished, 3 = terminal damage,
                                            #          4 = inactive, 5 = not enough laps, 6 = black flagged,
                                            #          7 = red flagged, 8 = mechanical failure, 9 = session skipped,
                                            #          10 = session simulated
}

EVENT_DRS_DISABLED = {
    "reason": "B",                          # uint8  - 0 = Wet track, 1 = Safety car deployed, 2 = Red flag, 3 = Min lap not reached
}

EVENT_TEAM_MATE_IN_PITS = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of team mate
}

EVENT_RACE_WINNER = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of the race winner
}

EVENT_PENALTY = {
    "penaltyType": "B",                     # uint8  - Penalty type - see appendix
    "infringementType": "B",                # uint8  - Infringement type - see appendix
    "vehicleIdx": "B",                      # uint8  - Vehicle index of the car the penalty is applied to
    "otherVehicleIdx": "B",                 # uint8  - Vehicle index of the other car involved
    "time": "B",                            # uint8  - Time gained, or time spent doing action in seconds
    "lapNum": "B",                          # uint8  - Lap the penalty occurred on
    "placesGained": "B",                    # uint8  - Number of places gained by this
}

EVENT_SPEED_TRAP = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of the vehicle triggering speed trap
    "speed": "f",                           # float  - Top speed achieved in kilometres per hour
    "isOverallFastestInSession": "B",       # uint8  - Overall fastest speed in session = 1, otherwise 0
    "isDriverFastestInSession": "B",        # uint8  - Fastest speed for driver in session = 1, otherwise 0
    "fastestVehicleIdxInSession": "B",      # uint8  - Vehicle index of the vehicle that is the fastest in this session
    "fastestSpeedInSession": "f",           # float  - Speed of the vehicle that is the fastest in this session
}

EVENT_START_LIGHTS = {
    "numLights": "B",                       # uint8  - Number of lights showing
}

EVENT_DRIVE_THROUGH_PENALTY_SERVED = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of the vehicle serving drive through
}

EVENT_STOP_GO_PENALTY_SERVED = {
    "vehicleIdx": "B",                      # uint8  - Vehicle index of the vehicle serving stop go
    "stopTime": "f",                        # float  - Time spent serving stop go in seconds
}

EVENT_FLASHBACK = {
    "flashbackFrameIdentifier": "I",        # uint32 - Frame identifier flashed back to
    "flashbackSessionTime": "f",            # float  - Session time flashed back to
}

EVENT_BUTTONS = {
    "buttonStatus": "I",                    # uint32 - Bit flags specifying which buttons are being pressed - see appendix
}

EVENT_OVERTAKE = {
    "overtakingVehicleIdx": "B",            # uint8  - Vehicle index of the vehicle overtaking
    "beingOvertakenVehicleIdx": "B",        # uint8  - Vehicle index of the vehicle being overtaken
}

EVENT_SAFETY_CAR = {
    "safetyCarType": "B",                   # uint8  - 0 = No Safety Car, 1 = Full, 2 = Virtual, 3 = Formation Lap
    "eventType": "B",                       # uint8  - 0 = Deployed, 1 = Returning, 2 = Returned, 3 = Resume Race
}

EVENT_COLLISION = {
    "vehicle1Idx": "B",                     # uint8  - Vehicle index of the first vehicle involved in the collision
    "vehicle2Idx": "B",                     # uint8  - Vehicle index of the second vehicle involved in the collision
}

# PacketEventData: PACKET_HEADER + PACKET_EVENT_CODE + EventDataDetails (union, interpret per event type)
