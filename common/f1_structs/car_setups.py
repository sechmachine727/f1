"""
F1 25 UDP Telemetry — Car Setups Packet (1133 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Data about one car setup
CAR_SETUP_DATA = {
    "m_frontWing": "B",                      # uint8  - Front wing aero
    "m_rearWing": "B",                       # uint8  - Rear wing aero
    "m_onThrottle": "B",                     # uint8  - Differential adjustment on throttle (percentage)
    "m_offThrottle": "B",                    # uint8  - Differential adjustment off throttle (percentage)
    "m_frontCamber": "f",                    # float  - Front camber angle (suspension geometry)
    "m_rearCamber": "f",                     # float  - Rear camber angle (suspension geometry)
    "m_frontToe": "f",                       # float  - Front toe angle (suspension geometry)
    "m_rearToe": "f",                        # float  - Rear toe angle (suspension geometry)
    "m_frontSuspension": "B",               # uint8  - Front suspension
    "m_rearSuspension": "B",                # uint8  - Rear suspension
    "m_frontAntiRollBar": "B",              # uint8  - Front anti-roll bar
    "m_rearAntiRollBar": "B",               # uint8  - Rear anti-roll bar
    "m_frontSuspensionHeight": "B",         # uint8  - Front ride height
    "m_rearSuspensionHeight": "B",          # uint8  - Rear ride height
    "m_brakePressure": "B",                 # uint8  - Brake pressure (percentage)
    "m_brakeBias": "B",                     # uint8  - Brake bias (percentage)
    "m_engineBraking": "B",                 # uint8  - Engine braking (percentage)
    "m_rearLeftTyrePressure": "f",          # float  - Rear left tyre pressure (PSI)
    "m_rearRightTyrePressure": "f",         # float  - Rear right tyre pressure (PSI)
    "m_frontLeftTyrePressure": "f",         # float  - Front left tyre pressure (PSI)
    "m_frontRightTyrePressure": "f",        # float  - Front right tyre pressure (PSI)
    "m_ballast": "B",                       # uint8  - Ballast
    "m_fuelLoad": "f",                      # float  - Fuel load
}

# Packet-level trailing field (after CAR_SETUP_DATA[22])
PACKET_CAR_SETUP_TAIL = {
    "m_nextFrontWingValue": "f",            # float  - Value of front wing after next pit stop - player only
}

# PacketCarSetupData: PACKET_HEADER + CAR_SETUP_DATA[22] + PACKET_CAR_SETUP_TAIL
