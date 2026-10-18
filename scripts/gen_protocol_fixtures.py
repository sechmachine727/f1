#!/usr/bin/env python3
"""Generate Go protocol fixtures from the checked-in Python decoder.

Outputs server/internal/protocol/testdata/:
  golden_2026.bin     length-prefixed 2026 packets, one per packet ID
  golden_2026.json    full expected decode of each packet
  golden_2025.json    full expected decode of the real 2025 capture datagrams

Run from the repository root after changing a layout or the decoder.
"""

import json
import struct
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "tests"))

from f1_packet_builder import build_packet  # noqa: E402
from common.f1_decoder.packet_decoder import PacketDecoder  # noqa: E402

OUT = ROOT / "server/internal/protocol/testdata"


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    decoder = PacketDecoder()

    packets = {}
    header = {"m_packetFormat": 2026, "m_sessionUID": 20260201, "m_playerCarIndex": 3}
    common = {
        "m_gameYear": 26, "m_gameMajorVersion": 1, "m_gameMinorVersion": 23,
        "m_packetVersion": 1, "m_frameIdentifier": 4242, "m_overallFrameIdentifier": 9001,
        "m_sessionTime": 123.5,
    }
    specs = {
        0: {"m_carMotionData[23].m_worldPositionX": 123.5, "m_carMotionData[1].m_gForceLateral": 1500},
        1: {"m_weather": 0, "m_trackTemperature": 40, "m_airTemperature": 25, "m_totalLaps": 57,
            "m_trackLength": 5412, "m_sessionType": 10, "m_trackId": 3, "m_formula": 13,
            "m_sessionTimeLeft": 1800, "m_sessionDuration": 3600, "m_networkGame": 1,
            "m_seasonLinkIdentifier": 11, "m_weekendLinkIdentifier": 12, "m_sessionLinkIdentifier": 13,
            "m_gameMode": 7, "m_safetyCarStatus": 0},
        2: {"m_lapData[3].m_carPosition": 2, "m_lapData[3].m_currentLapNum": 7,
            "m_lapData[3].m_currentLapTimeInMS": 91234, "m_lapData[3].m_speedTrapFastestSpeed": 329.1},
        3: {"m_eventStringCode": b"SCAR"},
        4: {"m_numActiveCars": 24, "m_participants[3].m_driverId": 65535,
            "m_participants[3].m_teamId": 4000, "m_participants[3].m_name": b"Verstappen",
            "m_participants[3].m_yourTelemetry": 0},
        5: {"m_carSetupData[3].m_frontWing": 33, "m_carSetupData[3].m_rearWing": 12},
        6: {"m_carTelemetryData[3].m_speed": 312, "m_carTelemetryData[3].m_gear": 8,
            "m_carTelemetryData[3].m_engineRPM": 12000, "m_carTelemetryData[3].m_throttle": 1.0,
            "m_carTelemetryData[3].m_engineTemperature": 110,
            "m_carTelemetryData[3].m_brakesTemperature": [500, 510, 520, 530],
            "m_carTelemetryData[3].m_tyresSurfaceTemperature": [90, 91, 92, 93],
            "m_carTelemetryData[3].m_tyresInnerTemperature": [80, 81, 82, 83],
            "m_carTelemetryData[3].m_tyresPressure": [22.0, 22.5, 23.0, 23.5]},
        7: {"m_carStatusData[3].m_fuelInTank": 42.5, "m_carStatusData[3].m_fuelRemainingLaps": 8.5,
            "m_carStatusData[3].m_ersStoreEnergy": 3000000.0,
            "m_carStatusData[3].m_ersDeployMode": 3, "m_carStatusData[3].m_ersHarvestLimitPerLap": 4000000.0,
            "m_carStatusData[3].m_actualTyreCompound": 18, "m_carStatusData[3].m_tyresAgeLaps": 12},
        8: {"m_numCars": 24, "m_classificationData[3].m_position": 1},
        9: {"m_numPlayers": 1, "m_lobbyPlayers[0].m_teamId": 7},
        10: {"m_carDamageData[3].m_tyresWear": [1.5, 2.5, 3.5, 4.5],
             "m_carDamageData[3].m_tyreBlisters": [0, 1, 2, 3]},
        11: {"m_carIdx": 3, "m_numLaps": 7, "m_lapHistoryData[0].m_lapTimeInMS": 91234},
        12: {"m_carIdx": 3, "m_tyreSetData[0].m_actualTyreCompound": 18},
        13: {"m_gForceLateral": 1.5, "m_enginePowerICE": 700000.0},
        14: {"m_playerSessionBestDataSet.m_sessionTime": 88.125},
        15: {"m_numLaps": 7, "m_lapStart": 1},
        16: {"m_carTelemetry2Data[3].m_activeAeroMode": 1,
             "m_carTelemetry2Data[3].m_overtakeAvailable": 1,
             "m_carTelemetry2Data[3].m_overtakeActive": 0,
             "m_carTelemetry2Data[3].m_2026Regulations": 1},
    }
    manifest = {"version": 1, "packets": {}}
    blob = bytearray()
    for pid in sorted(specs):
        values = dict(header)
        values.update(common)
        values.update({"m_packetId": pid})
        values.update(specs[pid])
        try:
            data = build_packet(2026, pid, **values)
        except KeyError as exc:
            # Fields differ per supplied structure; report the exact missing path.
            raise SystemExit(f"packet {pid}: {exc}") from exc
        if pid == 3:
            # The layout covers the event prefix; the 16-byte union carries
            # SafetyCar{safetyCarType=2 (VSC), eventType=0 (Deployed)}.
            data += bytes([2, 0] + [0] * 10)
        decoded = decoder.decode(data)
        blob += struct.pack("<IH", pid, len(data)) + data
        manifest["packets"][str(pid)] = {"length": len(data), "decoded": decoded}

    (OUT / "golden_2026.bin").write_bytes(blob)
    (OUT / "golden_2026.json").write_text(json.dumps(manifest, indent=1) + "\n")

    legacy = (ROOT / "tests/fixtures/real_2025_packets.f1bin").read_bytes()
    off = 0
    rows = []
    while off + 10 <= len(legacy):
        ts, length = struct.unpack_from("<QH", legacy, off)
        off += 10
        if off + length > len(legacy):
            rows.append({"timestamp": ts, "truncated": True})
            break
        data = legacy[off:off + length]
        off += length
        try:
            decoded = decoder.decode(data)
            rows.append({"timestamp": ts, "length": length, "decoded": decoded})
        except Exception as exc:  # malformed legacy fixtures stay explicit
            rows.append({"timestamp": ts, "length": length, "error": str(exc)})
    (OUT / "golden_2025.json").write_text(json.dumps({"packets": rows}, indent=1) + "\n")
    print(f"wrote {len(manifest['packets'])} 2026 packets and {len(rows)} legacy packets")


if __name__ == "__main__":
    main()
