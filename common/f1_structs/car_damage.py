"""
F1 25 UDP Telemetry — Car Damage Packet (1041 bytes).

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Car damage data for one car
CAR_DAMAGE_DATA = {
    "m_tyresWear": "4f",                     # float[4] - Tyre wear (percentage)
    "m_tyresDamage": "4B",                   # uint8[4] - Tyre damage (percentage)
    "m_brakesDamage": "4B",                  # uint8[4] - Brakes damage (percentage)
    "m_tyreBlisters": "4B",                  # uint8[4] - Tyre blisters value (percentage)
    "m_frontLeftWingDamage": "B",            # uint8    - Front left wing damage (percentage)
    "m_frontRightWingDamage": "B",           # uint8    - Front right wing damage (percentage)
    "m_rearWingDamage": "B",                 # uint8    - Rear wing damage (percentage)
    "m_floorDamage": "B",                    # uint8    - Floor damage (percentage)
    "m_diffuserDamage": "B",                 # uint8    - Diffuser damage (percentage)
    "m_sidepodDamage": "B",                  # uint8    - Sidepod damage (percentage)
    "m_drsFault": "B",                       # uint8    - Indicator for DRS fault, 0 = OK, 1 = fault
    "m_ersFault": "B",                       # uint8    - Indicator for ERS fault, 0 = OK, 1 = fault
    "m_gearBoxDamage": "B",                  # uint8    - Gear box damage (percentage)
    "m_engineDamage": "B",                   # uint8    - Engine damage (percentage)
    "m_engineMGUHWear": "B",                 # uint8    - Engine wear MGU-H (percentage)
    "m_engineESWear": "B",                   # uint8    - Engine wear ES (percentage)
    "m_engineCEWear": "B",                   # uint8    - Engine wear CE (percentage)
    "m_engineICEWear": "B",                  # uint8    - Engine wear ICE (percentage)
    "m_engineMGUKWear": "B",                 # uint8    - Engine wear MGU-K (percentage)
    "m_engineTCWear": "B",                   # uint8    - Engine wear TC (percentage)
    "m_engineBlown": "B",                    # uint8    - Engine blown, 0 = OK, 1 = fault
    "m_engineSeized": "B",                   # uint8    - Engine seized, 0 = OK, 1 = fault
}

# PacketCarDamageData: PACKET_HEADER + CAR_DAMAGE_DATA[22]
