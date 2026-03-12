"""
F1 25 UDP Telemetry — Motion Ex Packet (273 bytes).

Extra player car ONLY data.

Source: F1 25 Telemetry Output Structures (c) 2025 Electronic Arts Inc.
"""

# Note: All wheel arrays have the order: RL, RR, FL, FR

PACKET_MOTION_EX_DATA = {
    "m_suspensionPosition": "4f",            # float[4] - Suspension position (RL, RR, FL, FR)
    "m_suspensionVelocity": "4f",            # float[4] - Suspension velocity (RL, RR, FL, FR)
    "m_suspensionAcceleration": "4f",        # float[4] - Suspension acceleration (RL, RR, FL, FR)
    "m_wheelSpeed": "4f",                    # float[4] - Speed of each wheel
    "m_wheelSlipRatio": "4f",                # float[4] - Slip ratio for each wheel
    "m_wheelSlipAngle": "4f",                # float[4] - Slip angles for each wheel
    "m_wheelLatForce": "4f",                 # float[4] - Lateral forces for each wheel
    "m_wheelLongForce": "4f",                # float[4] - Longitudinal forces for each wheel
    "m_heightOfCOGAboveGround": "f",         # float    - Height of centre of gravity above ground
    "m_localVelocityX": "f",                # float    - Velocity in local space X - metres/s
    "m_localVelocityY": "f",                # float    - Velocity in local space Y
    "m_localVelocityZ": "f",                # float    - Velocity in local space Z
    "m_angularVelocityX": "f",              # float    - Angular velocity x-component - radians/s
    "m_angularVelocityY": "f",              # float    - Angular velocity y-component
    "m_angularVelocityZ": "f",              # float    - Angular velocity z-component
    "m_angularAccelerationX": "f",          # float    - Angular acceleration x-component - radians/s/s
    "m_angularAccelerationY": "f",          # float    - Angular acceleration y-component
    "m_angularAccelerationZ": "f",          # float    - Angular acceleration z-component
    "m_frontWheelsAngle": "f",              # float    - Current front wheels angle in radians
    "m_wheelVertForce": "4f",               # float[4] - Vertical forces for each wheel
    "m_frontAeroHeight": "f",               # float    - Front plank edge height above road surface
    "m_rearAeroHeight": "f",                # float    - Rear plank edge height above road surface
    "m_frontRollAngle": "f",                # float    - Roll angle of the front suspension
    "m_rearRollAngle": "f",                 # float    - Roll angle of the rear suspension
    "m_chassisYaw": "f",                    # float    - Yaw angle of the chassis relative to direction of motion - radians
    "m_chassisPitch": "f",                  # float    - Pitch angle of the chassis relative to direction of motion - radians
    "m_wheelCamber": "4f",                  # float[4] - Camber of each wheel in radians
    "m_wheelCamberGain": "4f",              # float[4] - Camber gain for each wheel in radians (active - dynamic camber)
}

# PacketMotionExData: PACKET_HEADER + PACKET_MOTION_EX_DATA (player car only, not arrayed)
