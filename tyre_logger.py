import csv
import socket
import struct
import time
from pathlib import Path

# ---- UDP config ----
UDP_IP = "0.0.0.0"
UDP_PORT = 20777

# ---- Packet constants (F1 25 spec) ----
PACKET_ID_LAP_DATA = 2
PACKET_ID_CAR_TELEMETRY = 6
PACKET_ID_CAR_STATUS = 7
PACKET_ID_CAR_DAMAGE = 10

# PacketHeader (packed, little-endian)
HEADER_STRUCT = struct.Struct("<HBBBBBQfIIBB")
HEADER_SIZE = HEADER_STRUCT.size  # 29 bytes

NUM_CARS = 22

# Wheel order in all arrays: 0=RL, 1=RR, 2=FL, 3=FR
WHEEL_NAMES = ("rl", "rr", "fl", "fr")

# ---- LapData struct (57 bytes) – only used for lap context ----
LAPDATA_STRUCT = struct.Struct(
    "<"
    "I"   # m_lastLapTimeInMS
    "I"   # m_currentLapTimeInMS
    "H"   # m_sector1TimeMSPart
    "B"   # m_sector1TimeMinutesPart
    "H"   # m_sector2TimeMSPart
    "B"   # m_sector2TimeMinutesPart
    "H"   # m_deltaToCarInFrontMSPart
    "B"   # m_deltaToCarInFrontMinutesPart
    "H"   # m_deltaToRaceLeaderMSPart
    "B"   # m_deltaToRaceLeaderMinutesPart
    "f"   # m_lapDistance
    "f"   # m_totalDistance
    "f"   # m_safetyCarDelta
    "B"   # m_carPosition
    "B"   # m_currentLapNum
    "B"   # m_pitStatus
    "B"   # m_numPitStops
    "B"   # m_sector
    "B"   # m_currentLapInvalid
    "B"   # m_penalties
    "B"   # m_totalWarnings
    "B"   # m_cornerCuttingWarnings
    "B"   # m_numUnservedDriveThroughPens
    "B"   # m_numUnservedStopGoPens
    "B"   # m_gridPosition
    "B"   # m_driverStatus
    "B"   # m_resultStatus
    "B"   # m_pitLaneTimerActive
    "H"   # m_pitLaneTimeInLaneInMS
    "H"   # m_pitStopTimerInMS
    "B"   # m_pitStopShouldServePen
    "f"   # m_speedTrapFastestSpeed
    "B"   # m_speedTrapFastestLap
)
LAPDATA_SIZE = LAPDATA_STRUCT.size  # 57

# ---- CarTelemetryData struct (60 bytes) ----
CAR_TELEMETRY_STRUCT = struct.Struct(
    "<"
    "H"      # m_speed
    "f"      # m_throttle
    "f"      # m_steer
    "f"      # m_brake
    "B"      # m_clutch
    "b"      # m_gear
    "H"      # m_engineRPM
    "B"      # m_drs
    "B"      # m_revLightsPercent
    "H"      # m_revLightsBitValue
    "4H"     # m_brakesTemperature[4]
    "4B"     # m_tyresSurfaceTemperature[4]
    "4B"     # m_tyresInnerTemperature[4]
    "H"      # m_engineTemperature
    "4f"     # m_tyresPressure[4]
    "4B"     # m_surfaceType[4]
)
CAR_TELEMETRY_SIZE = CAR_TELEMETRY_STRUCT.size  # 60

# ---- CarStatusData struct (55 bytes) ----
CAR_STATUS_STRUCT = struct.Struct(
    "<"
    "B"      # m_tractionControl
    "B"      # m_antiLockBrakes
    "B"      # m_fuelMix
    "B"      # m_frontBrakeBias
    "B"      # m_pitLimiterStatus
    "f"      # m_fuelInTank
    "f"      # m_fuelCapacity
    "f"      # m_fuelRemainingLaps
    "H"      # m_maxRPM
    "H"      # m_idleRPM
    "B"      # m_maxGears
    "B"      # m_drsAllowed
    "H"      # m_drsActivationDistance
    "B"      # m_actualTyreCompound
    "B"      # m_visualTyreCompound
    "B"      # m_tyresAgeLaps
    "b"      # m_vehicleFiaFlags
    "f"      # m_enginePowerICE
    "f"      # m_enginePowerMGUK
    "f"      # m_ersStoreEnergy
    "B"      # m_ersDeployMode
    "f"      # m_ersHarvestedThisLapMGUK
    "f"      # m_ersHarvestedThisLapMGUH
    "f"      # m_ersDeployedThisLap
    "B"      # m_networkPaused
)
CAR_STATUS_SIZE = CAR_STATUS_STRUCT.size  # 55

# ---- CarDamageData struct (46 bytes) ----
CAR_DAMAGE_STRUCT = struct.Struct(
    "<"
    "4f"     # m_tyresWear[4]
    "4B"     # m_tyresDamage[4]
    "4B"     # m_brakesDamage[4]
    "4B"     # m_tyreBlisters[4]
    "B"      # m_frontLeftWingDamage
    "B"      # m_frontRightWingDamage
    "B"      # m_rearWingDamage
    "B"      # m_floorDamage
    "B"      # m_diffuserDamage
    "B"      # m_sidepodDamage
    "B"      # m_drsFault
    "B"      # m_ersFault
    "B"      # m_gearBoxDamage
    "B"      # m_engineDamage
    "B"      # m_engineMGUHWear
    "B"      # m_engineESWear
    "B"      # m_engineCEWear
    "B"      # m_engineICEWear
    "B"      # m_engineMGUKWear
    "B"      # m_engineTCWear
    "B"      # m_engineBlown
    "B"      # m_engineSeized
)
CAR_DAMAGE_SIZE = CAR_DAMAGE_STRUCT.size  # 46

# ---- Tyre compound lookup ----
ACTUAL_COMPOUND = {
    16: "C5", 17: "C4", 18: "C3", 19: "C2", 20: "C1",
    21: "C0", 22: "C6", 7: "inter", 8: "wet",
    9: "dry_classic", 10: "wet_classic",
    11: "super_soft_f2", 12: "soft_f2", 13: "medium_f2",
    14: "hard_f2", 15: "wet_f2",
}

VISUAL_COMPOUND = {
    16: "soft", 17: "medium", 18: "hard", 7: "inter", 8: "wet",
    19: "super_soft_f2", 20: "soft_f2", 21: "medium_f2", 22: "hard_f2",
    15: "wet_f2",
}


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind((UDP_IP, UDP_PORT))
    print(f"Listening on UDP {UDP_IP}:{UDP_PORT} ...")

    data_dir = Path("data")
    data_dir.mkdir(exist_ok=True)

    fieldnames = [
        "wall_time",
        "session_uid",
        "session_time",
        "frame_id",
        "player_car_index",
        "current_lap_num",
        "lap_distance_m",
        "speed_kmh",
        "throttle",
        "brake",
        "tyre_compound_actual",
        "tyre_compound_visual",
        "tyres_age_laps",
    ]
    for metric in (
        "tyre_wear", "tyre_damage", "tyre_blisters",
        "tyre_surface_temp", "tyre_inner_temp", "tyre_pressure",
        "brake_temp",
    ):
        for wn in WHEEL_NAMES:
            fieldnames.append(f"{metric}_{wn}")

    out_path = data_dir / f"f1_25_tyres_{int(time.time())}.csv"
    with out_path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()

        # Latest state from each packet type (merged on every telemetry frame)
        lap_state = {}
        status_state = {}
        damage_state = {}

        last_seen_lap_num = None
        last_seen_compound = None

        while True:
            data, addr = sock.recvfrom(4096)
            if len(data) < HEADER_SIZE:
                continue

            (
                packet_format, game_year, game_major, game_minor,
                packet_version, packet_id, session_uid, session_time,
                frame_id, overall_frame_id, player_car_index,
                secondary_player_car_index,
            ) = HEADER_STRUCT.unpack_from(data, 0)

            # -- Lap Data (packet 2): extract lap context --
            if packet_id == PACKET_ID_LAP_DATA:
                base = HEADER_SIZE + player_car_index * LAPDATA_SIZE
                if len(data) < base + LAPDATA_SIZE:
                    continue
                fields = LAPDATA_STRUCT.unpack_from(data, base)
                lap_state = {
                    "current_lap_num": fields[14],   # m_currentLapNum
                    "lap_distance_m": round(fields[10], 2),  # m_lapDistance
                }
                continue

            # -- Car Status (packet 7): extract compound and tyre age --
            if packet_id == PACKET_ID_CAR_STATUS:
                base = HEADER_SIZE + player_car_index * CAR_STATUS_SIZE
                if len(data) < base + CAR_STATUS_SIZE:
                    continue
                fields = CAR_STATUS_STRUCT.unpack_from(data, base)
                actual = fields[13]   # m_actualTyreCompound
                visual = fields[14]   # m_visualTyreCompound
                age = fields[15]      # m_tyresAgeLaps
                status_state = {
                    "tyre_compound_actual": ACTUAL_COMPOUND.get(actual, str(actual)),
                    "tyre_compound_visual": VISUAL_COMPOUND.get(visual, str(visual)),
                    "tyres_age_laps": age,
                }
                continue

            # -- Car Damage (packet 10): extract tyre wear/damage/blisters --
            if packet_id == PACKET_ID_CAR_DAMAGE:
                base = HEADER_SIZE + player_car_index * CAR_DAMAGE_SIZE
                if len(data) < base + CAR_DAMAGE_SIZE:
                    continue
                fields = CAR_DAMAGE_STRUCT.unpack_from(data, base)
                # fields[0:4]  = m_tyresWear[4]      (float)
                # fields[4:8]  = m_tyresDamage[4]     (uint8)
                # fields[8:12] = m_brakesDamage[4]    (uint8)
                # fields[12:16]= m_tyreBlisters[4]    (uint8)
                damage_state = {}
                for i, wn in enumerate(WHEEL_NAMES):
                    damage_state[f"tyre_wear_{wn}"] = round(fields[0 + i], 2)
                    damage_state[f"tyre_damage_{wn}"] = fields[4 + i]
                    damage_state[f"tyre_blisters_{wn}"] = fields[12 + i]
                continue

            # -- Car Telemetry (packet 6): main capture trigger --
            if packet_id == PACKET_ID_CAR_TELEMETRY:
                base = HEADER_SIZE + player_car_index * CAR_TELEMETRY_SIZE
                if len(data) < base + CAR_TELEMETRY_SIZE:
                    continue
                fields = CAR_TELEMETRY_STRUCT.unpack_from(data, base)
                # fields layout:
                #  0: m_speed
                #  1: m_throttle
                #  2: m_steer
                #  3: m_brake
                #  4: m_clutch
                #  5: m_gear
                #  6: m_engineRPM
                #  7: m_drs
                #  8: m_revLightsPercent
                #  9: m_revLightsBitValue
                # 10-13: m_brakesTemperature[4]
                # 14-17: m_tyresSurfaceTemperature[4]
                # 18-21: m_tyresInnerTemperature[4]
                # 22: m_engineTemperature
                # 23-26: m_tyresPressure[4]
                # 27-30: m_surfaceType[4]

                telem = {
                    "speed_kmh": fields[0],
                    "throttle": round(fields[1], 3),
                    "brake": round(fields[3], 3),
                }
                for i, wn in enumerate(WHEEL_NAMES):
                    telem[f"brake_temp_{wn}"] = fields[10 + i]
                    telem[f"tyre_surface_temp_{wn}"] = fields[14 + i]
                    telem[f"tyre_inner_temp_{wn}"] = fields[18 + i]
                    telem[f"tyre_pressure_{wn}"] = round(fields[23 + i], 2)

                # Detect tyre compound change
                compound = status_state.get("tyre_compound_actual")
                if compound and last_seen_compound is not None and compound != last_seen_compound:
                    print(f"TYRE CHANGE: {last_seen_compound} -> {compound}")
                last_seen_compound = compound

                # Detect lap completion
                current_lap = lap_state.get("current_lap_num")
                if current_lap is not None:
                    if last_seen_lap_num is None:
                        last_seen_lap_num = current_lap
                    elif current_lap != last_seen_lap_num:
                        wear_str = ", ".join(
                            f"{wn.upper()}={damage_state.get(f'tyre_wear_{wn}', '?')}%"
                            for wn in WHEEL_NAMES
                        )
                        print(
                            f"LAP {last_seen_lap_num} COMPLETED | "
                            f"Compound: {compound or '?'} | "
                            f"Age: {status_state.get('tyres_age_laps', '?')} laps | "
                            f"Wear: {wear_str}"
                        )
                        last_seen_lap_num = current_lap

                # Merge all state and write row
                row = {
                    "wall_time": time.time(),
                    "session_uid": session_uid,
                    "session_time": session_time,
                    "frame_id": frame_id,
                    "player_car_index": player_car_index,
                }
                row.update(lap_state)
                row.update(status_state)
                row.update(damage_state)
                row.update(telem)

                writer.writerow(row)
                f.flush()


if __name__ == "__main__":
    main()
