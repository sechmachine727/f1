import csv
import socket
import time
from pathlib import Path

from telemetry_server.f1_telemetry_parser import (
    F1TelemetryParser,
    PACKET_ID_CAR_TELEMETRY,
    WHEEL_NAMES,
)

UDP_IP = "0.0.0.0"
UDP_PORT = 20777


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

        parser = F1TelemetryParser()

        last_seen_lap_num = None
        last_seen_compound = None

        while True:
            data, addr = sock.recvfrom(4096)
            packet_id = parser.parse_packet(data)

            if packet_id != PACKET_ID_CAR_TELEMETRY:
                continue

            # Detect tyre compound change
            compound = parser.status_state.get("tyre_compound_actual")
            if compound and last_seen_compound is not None and compound != last_seen_compound:
                print(f"TYRE CHANGE: {last_seen_compound} -> {compound}")
            last_seen_compound = compound

            # Detect lap completion
            current_lap = parser.lap_state.get("current_lap_num")
            if current_lap is not None:
                if last_seen_lap_num is None:
                    last_seen_lap_num = current_lap
                elif current_lap != last_seen_lap_num:
                    wear_str = ", ".join(
                        f"{wn.upper()}={parser.damage_state.get(f'tyre_wear_{wn}', '?')}%"
                        for wn in WHEEL_NAMES
                    )
                    print(
                        f"LAP {last_seen_lap_num} COMPLETED | "
                        f"Compound: {compound or '?'} | "
                        f"Age: {parser.status_state.get('tyres_age_laps', '?')} laps | "
                        f"Wear: {wear_str}"
                    )
                    last_seen_lap_num = current_lap

            # Merge all state and write row
            row = {
                "wall_time": time.time(),
                "session_uid": parser.session_uid,
                "session_time": parser.session_time,
                "frame_id": "",
                "player_car_index": "",
            }
            row.update(parser.lap_state)
            row.update(parser.status_state)
            row.update(parser.damage_state)
            row.update(parser.telemetry_state)

            writer.writerow(row)
            f.flush()


if __name__ == "__main__":
    main()
