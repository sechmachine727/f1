"""WebSocket bridge that streams live F1 25 tyre telemetry from UDP to the
race_engineer_hub web app.

Usage:
    python tyre_telemetry_server.py

Listens on UDP 20777 for F1 25 telemetry packets and exposes a WebSocket
server on port 8765.  The web app connects to ws://localhost:8765 and
receives JSON frames with the latest tyre state.
"""

import asyncio
import json
import socket
import struct

import websockets

# ---------------------------------------------------------------------------
# Re-use constants and struct definitions from tyre_logger.py
# ---------------------------------------------------------------------------
from tyre_logger import (
    ACTUAL_COMPOUND,
    CAR_DAMAGE_SIZE,
    CAR_DAMAGE_STRUCT,
    CAR_STATUS_SIZE,
    CAR_STATUS_STRUCT,
    CAR_TELEMETRY_SIZE,
    CAR_TELEMETRY_STRUCT,
    HEADER_SIZE,
    HEADER_STRUCT,
    LAPDATA_SIZE,
    LAPDATA_STRUCT,
    PACKET_ID_CAR_DAMAGE,
    PACKET_ID_CAR_STATUS,
    PACKET_ID_CAR_TELEMETRY,
    PACKET_ID_LAP_DATA,
    VISUAL_COMPOUND,
    WHEEL_NAMES,
)

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
UDP_IP = "0.0.0.0"
UDP_PORT = 20777
WS_HOST = "0.0.0.0"
WS_PORT = 8765

# ---------------------------------------------------------------------------
# Shared state – written by the UDP reader, read by WebSocket handlers
# ---------------------------------------------------------------------------
lap_state: dict = {}
status_state: dict = {}
damage_state: dict = {}
telemetry_state: dict = {}

connected_clients: set = set()


def build_message() -> str:
    """Build a JSON message from the latest merged state."""
    tires = {}
    for wn in WHEEL_NAMES:
        tires[wn] = {
            "surfaceTemp": telemetry_state.get(f"tyre_surface_temp_{wn}", 0),
            "innerTemp": telemetry_state.get(f"tyre_inner_temp_{wn}", 0),
            "pressure": telemetry_state.get(f"tyre_pressure_{wn}", 0),
            "wear": damage_state.get(f"tyre_wear_{wn}", 0),
            "damage": damage_state.get(f"tyre_damage_{wn}", 0),
            "blisters": damage_state.get(f"tyre_blisters_{wn}", 0),
            "brakeTemp": telemetry_state.get(f"brake_temp_{wn}", 0),
        }

    return json.dumps({
        "tires": tires,
        "compound": status_state.get("tyre_compound_actual", ""),
        "compoundVisual": status_state.get("tyre_compound_visual", ""),
        "tyresAgeLaps": status_state.get("tyres_age_laps", 0),
        "currentLap": lap_state.get("current_lap_num", 0),
        "speed": telemetry_state.get("speed_kmh", 0),
    })


async def broadcast(message: str):
    """Send a message to every connected WebSocket client."""
    if not connected_clients:
        return
    await asyncio.gather(
        *(client.send(message) for client in connected_clients),
        return_exceptions=True,
    )


async def ws_handler(websocket):
    """Handle a new WebSocket connection."""
    connected_clients.add(websocket)
    print(f"Client connected ({len(connected_clients)} total)")
    try:
        # Send current state immediately so the UI isn't blank
        await websocket.send(build_message())
        # Keep connection alive – we only push, client doesn't send
        async for _ in websocket:
            pass
    finally:
        connected_clients.discard(websocket)
        print(f"Client disconnected ({len(connected_clients)} total)")


async def udp_reader():
    """Read F1 25 UDP packets and update shared state, broadcasting on each
    telemetry frame (packet 6)."""
    global lap_state, status_state, damage_state, telemetry_state

    loop = asyncio.get_running_loop()
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    if hasattr(socket, "SO_REUSEPORT"):
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
    sock.bind((UDP_IP, UDP_PORT))
    sock.setblocking(False)

    print(f"Listening for F1 25 UDP on {UDP_IP}:{UDP_PORT}")

    while True:
        try:
            data = await loop.sock_recv(sock, 4096)
        except BlockingIOError:
            await asyncio.sleep(0.001)
            continue

        if len(data) < HEADER_SIZE:
            continue

        header = HEADER_STRUCT.unpack_from(data, 0)
        packet_id = header[5]
        player_car_index = header[10]

        # -- Lap Data --
        if packet_id == PACKET_ID_LAP_DATA:
            base = HEADER_SIZE + player_car_index * LAPDATA_SIZE
            if len(data) < base + LAPDATA_SIZE:
                continue
            fields = LAPDATA_STRUCT.unpack_from(data, base)
            lap_state = {
                "current_lap_num": fields[14],
                "lap_distance_m": round(fields[10], 2),
            }
            continue

        # -- Car Status --
        if packet_id == PACKET_ID_CAR_STATUS:
            base = HEADER_SIZE + player_car_index * CAR_STATUS_SIZE
            if len(data) < base + CAR_STATUS_SIZE:
                continue
            fields = CAR_STATUS_STRUCT.unpack_from(data, base)
            actual = fields[13]
            visual = fields[14]
            age = fields[15]
            status_state = {
                "tyre_compound_actual": ACTUAL_COMPOUND.get(actual, str(actual)),
                "tyre_compound_visual": VISUAL_COMPOUND.get(visual, str(visual)),
                "tyres_age_laps": age,
            }
            continue

        # -- Car Damage --
        if packet_id == PACKET_ID_CAR_DAMAGE:
            base = HEADER_SIZE + player_car_index * CAR_DAMAGE_SIZE
            if len(data) < base + CAR_DAMAGE_SIZE:
                continue
            fields = CAR_DAMAGE_STRUCT.unpack_from(data, base)
            new_damage = {}
            for i, wn in enumerate(WHEEL_NAMES):
                new_damage[f"tyre_wear_{wn}"] = round(fields[0 + i], 2)
                new_damage[f"tyre_damage_{wn}"] = fields[4 + i]
                new_damage[f"tyre_blisters_{wn}"] = fields[12 + i]
            damage_state = new_damage
            continue

        # -- Car Telemetry (main trigger) --
        if packet_id == PACKET_ID_CAR_TELEMETRY:
            base = HEADER_SIZE + player_car_index * CAR_TELEMETRY_SIZE
            if len(data) < base + CAR_TELEMETRY_SIZE:
                continue
            fields = CAR_TELEMETRY_STRUCT.unpack_from(data, base)

            telem = {"speed_kmh": fields[0]}
            for i, wn in enumerate(WHEEL_NAMES):
                telem[f"brake_temp_{wn}"] = fields[10 + i]
                telem[f"tyre_surface_temp_{wn}"] = fields[14 + i]
                telem[f"tyre_inner_temp_{wn}"] = fields[18 + i]
                telem[f"tyre_pressure_{wn}"] = round(fields[23 + i], 2)
            telemetry_state = telem

            # Broadcast merged state to all WS clients
            await broadcast(build_message())


async def main():
    print(f"Starting WebSocket server on ws://{WS_HOST}:{WS_PORT}")
    async with websockets.serve(ws_handler, WS_HOST, WS_PORT):
        await udp_reader()


if __name__ == "__main__":
    asyncio.run(main())
