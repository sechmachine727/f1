"""WebSocket bridge that streams live F1 25 telemetry from UDP to the
race_engineer_hub web app.

Usage:
    python telemetry_server.py

Listens on UDP 20777 for F1 25 telemetry packets and exposes a WebSocket
server on port 8765.  The web app connects to ws://localhost:8765 and
receives JSON frames with the latest telemetry state (tires + power unit).
"""

import argparse
import asyncio
import csv
import json
import os
import socket
import struct
import time
from pathlib import Path

import websockets

from telemetry_server.damage_agent import DamageAgent

# ---------------------------------------------------------------------------
# Re-use constants and struct definitions from tyre_logger.py
# ---------------------------------------------------------------------------
from telemetry_server.tyre_logger import (
    ACTUAL_COMPOUND,
    CAR_DAMAGE_SIZE,
    CAR_DAMAGE_STRUCT,
    CAR_SETUP_SIZE,
    CAR_SETUP_STRUCT,
    CAR_STATUS_SIZE,
    CAR_STATUS_STRUCT,
    CAR_TELEMETRY_SIZE,
    CAR_TELEMETRY_STRUCT,
    HEADER_SIZE,
    HEADER_STRUCT,
    LAPDATA_SIZE,
    LAPDATA_STRUCT,
    MOTION_EX_SIZE,
    MOTION_EX_STRUCT,
    PACKET_ID_CAR_DAMAGE,
    PACKET_ID_CAR_SETUPS,
    PACKET_ID_CAR_STATUS,
    PACKET_ID_CAR_TELEMETRY,
    PACKET_ID_LAP_DATA,
    PACKET_ID_MOTION_EX,
    PACKET_ID_SESSION,
    SESSION_HEADER_SIZE,
    SESSION_HEADER_STRUCT,
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
setup_state: dict = {}
motion_ex_state: dict = {}
session_state: dict = {}
session_time: float = 0.0

# Alert state – computed server-side and sent pre-computed via WebSocket
aero_alert_conditions: dict = {}   # key -> {"level": "warn"|"crit", "value": float}
aero_alerts_log: list = []         # accumulated alerts
prev_aero_session_time: float = 0.0

tire_alert_conditions: dict = {}
tire_alerts_log: list = []
prev_tire_session_time: float = 0.0
prev_tire_compound: str = ""

pu_alert_conditions: dict = {}
pu_alerts_log: list = []
prev_pu_session_time: float = 0.0
prev_pu_ers_mode: str = ""
prev_pu_fuel_mix: str = ""

# DamageAgent state
damage_agent: DamageAgent | None = None
damage_agent_response: str | None = None
damage_agent_in_flight: bool = False
damage_agent_pending: list[str] = []
damage_agent_batch_handle: asyncio.TimerHandle | None = None
DAMAGE_AGENT_BATCH_DELAY: float = 1.0  # seconds to wait before flushing

# ---------------------------------------------------------------------------
# Lookups
# ---------------------------------------------------------------------------
SESSION_TYPE_LABELS = {
    0: "UNKNOWN", 1: "P1", 2: "P2", 3: "P3", 4: "SHORT PRACTICE",
    5: "Q1", 6: "Q2", 7: "Q3", 8: "SHORT QUALIFYING", 9: "OSQ",
    10: "RACE", 11: "RACE 2", 12: "RACE 3", 13: "TIME TRIAL",
    14: "SQ1", 15: "RACE SHORT", 16: "SQ3", 17: "SPRINT",
}

TRACK_NAMES = {
    0: "AUSTRALIAN GP", 1: "FRENCH GP", 2: "CHINESE GP", 3: "BAHRAIN GP",
    4: "SPANISH GP", 5: "MONACO GP", 6: "CANADIAN GP", 7: "BRITISH GP",
    8: "GERMAN GP", 9: "HUNGARIAN GP", 10: "BELGIAN GP", 11: "ITALIAN GP",
    12: "SINGAPORE GP", 13: "JAPANESE GP", 14: "ABU DHABI GP", 15: "UNITED STATES GP",
    16: "BRAZILIAN GP", 17: "AUSTRIAN GP", 18: "RUSSIAN GP", 19: "MEXICAN GP",
    20: "AZERBAIJAN GP", 21: "BAHRAIN SHORT", 22: "BRITISH SHORT",
    23: "US SHORT", 24: "JAPANESE SHORT", 25: "VIETNAMESE GP", 26: "DUTCH GP",
    27: "EMILIA ROMAGNA GP", 28: "PORTUGUESE GP", 29: "SAUDI ARABIAN GP", 30: "MIAMI GP",
    31: "LAS VEGAS GP", 32: "QATAR GP", 33: "QATAR GP",
}

connected_clients: set = set()

# ---------------------------------------------------------------------------
# CSV capture
# ---------------------------------------------------------------------------
CSV_FIELDNAMES = [
    "wall_time", "session_uid", "session_time", "frame_id",
    # Session
    "session_type", "track_name", "session_time_left",
    # Lap
    "current_lap_num", "car_position", "lap_distance_m",
    "last_lap_time_ms", "current_lap_time_ms",
    # Car dynamics
    "speed_kmh", "gear", "drs",
    # Tyres
    "tyre_compound_actual", "tyre_compound_visual", "tyres_age_laps",
]
for _metric in (
    "tyre_wear", "tyre_damage", "tyre_blisters",
    "tyre_surface_temp", "tyre_inner_temp", "tyre_pressure", "brake_temp",
):
    for _wn in WHEEL_NAMES:
        CSV_FIELDNAMES.append(f"{_metric}_{_wn}")
CSV_FIELDNAMES += [
    # Power unit
    "engine_rpm", "engine_temp",
    "fuel_in_tank", "fuel_remaining_laps", "fuel_mix",
    "engine_power_ice_w", "engine_power_mguk_w",
    "ers_store_energy_j", "ers_deploy_mode",
    "ers_deployed_this_lap_j", "ers_harvested_mguk_j", "ers_harvested_mguh_j",
    "engine_damage", "gearbox_damage",
    # Aero
    "front_wing_setup", "rear_wing_setup", "brake_bias",
    "front_ride_height_mm", "rear_ride_height_mm",
    "front_left_wing_damage", "front_right_wing_damage", "rear_wing_damage",
    "floor_damage", "diffuser_damage", "sidepod_damage", "drs_fault",
]


class CsvCapture:
    """Manages one CSV file per session_uid."""

    def __init__(self, data_dir: Path):
        self.data_dir = data_dir
        self.data_dir.mkdir(exist_ok=True)
        self._current_uid: int | None = None
        self._file = None
        self._writer: csv.DictWriter | None = None

    def _open_file(self, session_uid: int):
        """Open a new CSV for the given session."""
        self.close()
        self._current_uid = session_uid

        track_id = session_state.get("track_id", -1)
        session_type = session_state.get("session_type", 0)
        gp = TRACK_NAMES.get(track_id, f"track_{track_id}").replace(" ", "_").lower()
        sess = SESSION_TYPE_LABELS.get(session_type, f"session_{session_type}").replace(" ", "_").lower()
        ts = time.strftime("%Y%m%d-%H%M%S")

        path = self.data_dir / f"f1_25_{gp}_{sess}_{ts}.csv"
        self._file = path.open("w", newline="", encoding="utf-8")
        self._writer = csv.DictWriter(self._file, fieldnames=CSV_FIELDNAMES)
        self._writer.writeheader()
        self._file.flush()
        print(f"CSV capture started: {path}")

    def write_row(self, session_uid: int, session_time: float, frame_id: int):
        if not session_state:
            return  # Wait until session packet provides GP/session info
        if session_uid != self._current_uid:
            self._open_file(session_uid)

        row = {
            "wall_time": time.time(),
            "session_uid": session_uid,
            "session_time": session_time,
            "frame_id": frame_id,
            # Session
            "session_type": SESSION_TYPE_LABELS.get(
                session_state.get("session_type", 0), "unknown"
            ),
            "track_name": TRACK_NAMES.get(session_state.get("track_id", -1), "unknown"),
            "session_time_left": session_state.get("session_time_left", 0),
            # Lap
            "current_lap_num": lap_state.get("current_lap_num", ""),
            "car_position": lap_state.get("car_position", ""),
            "lap_distance_m": lap_state.get("lap_distance_m", ""),
            "last_lap_time_ms": lap_state.get("last_lap_time_ms", ""),
            "current_lap_time_ms": lap_state.get("current_lap_time_ms", ""),
            # Car dynamics
            "speed_kmh": telemetry_state.get("speed_kmh", ""),
            "gear": telemetry_state.get("gear", ""),
            "drs": telemetry_state.get("drs", ""),
            # Tyres
            "tyre_compound_actual": status_state.get("tyre_compound_actual", ""),
            "tyre_compound_visual": status_state.get("tyre_compound_visual", ""),
            "tyres_age_laps": status_state.get("tyres_age_laps", ""),
            # Power unit
            "engine_rpm": telemetry_state.get("engine_rpm", ""),
            "engine_temp": telemetry_state.get("engine_temp", ""),
            "fuel_in_tank": status_state.get("fuel_in_tank", ""),
            "fuel_remaining_laps": status_state.get("fuel_remaining_laps", ""),
            "fuel_mix": status_state.get("fuel_mix", ""),
            "engine_power_ice_w": status_state.get("engine_power_ice", ""),
            "engine_power_mguk_w": status_state.get("engine_power_mguk", ""),
            "ers_store_energy_j": status_state.get("ers_store_energy", ""),
            "ers_deploy_mode": status_state.get("ers_deploy_mode", ""),
            "ers_deployed_this_lap_j": status_state.get("ers_deployed_this_lap", ""),
            "ers_harvested_mguk_j": status_state.get("ers_harvested_mguk", ""),
            "ers_harvested_mguh_j": status_state.get("ers_harvested_mguh", ""),
            "engine_damage": damage_state.get("engine_damage", ""),
            "gearbox_damage": damage_state.get("gearbox_damage", ""),
            # Aero
            "front_wing_setup": setup_state.get("front_wing", ""),
            "rear_wing_setup": setup_state.get("rear_wing", ""),
            "brake_bias": status_state.get("front_brake_bias", ""),
            "front_ride_height_mm": motion_ex_state.get("front_aero_height", ""),
            "rear_ride_height_mm": motion_ex_state.get("rear_aero_height", ""),
            "front_left_wing_damage": damage_state.get("front_left_wing_damage", ""),
            "front_right_wing_damage": damage_state.get("front_right_wing_damage", ""),
            "rear_wing_damage": damage_state.get("rear_wing_damage", ""),
            "floor_damage": damage_state.get("floor_damage", ""),
            "diffuser_damage": damage_state.get("diffuser_damage", ""),
            "sidepod_damage": damage_state.get("sidepod_damage", ""),
            "drs_fault": damage_state.get("drs_fault", ""),
        }
        # Per-wheel tyre metrics
        for wn in WHEEL_NAMES:
            row[f"tyre_wear_{wn}"] = damage_state.get(f"tyre_wear_{wn}", "")
            row[f"tyre_damage_{wn}"] = damage_state.get(f"tyre_damage_{wn}", "")
            row[f"tyre_blisters_{wn}"] = damage_state.get(f"tyre_blisters_{wn}", "")
            row[f"tyre_surface_temp_{wn}"] = telemetry_state.get(f"tyre_surface_temp_{wn}", "")
            row[f"tyre_inner_temp_{wn}"] = telemetry_state.get(f"tyre_inner_temp_{wn}", "")
            row[f"tyre_pressure_{wn}"] = telemetry_state.get(f"tyre_pressure_{wn}", "")
            row[f"brake_temp_{wn}"] = telemetry_state.get(f"brake_temp_{wn}", "")

        self._writer.writerow(row)
        self._file.flush()

    def close(self):
        if self._file:
            self._file.close()
            self._file = None
            self._writer = None
            self._current_uid = None


csv_capture: CsvCapture | None = None


# ---------------------------------------------------------------------------
# Aero alert constants & processing
# ---------------------------------------------------------------------------
DAMAGE_REFIRE_STEP = 10  # re-alert every 10 % worsening

DAMAGE_PARTS = [
    {"key": "frontLeftWingDamage",  "label": "Front left wing",  "tag": "FL WING",  "clearMsg": "front left wing damage stabilised"},
    {"key": "frontRightWingDamage", "label": "Front right wing", "tag": "FR WING",  "clearMsg": "front right wing damage stabilised"},
    {"key": "rearWingDamage",       "label": "Rear wing",        "tag": "RR WING",  "clearMsg": "rear wing damage stabilised"},
    {"key": "floorDamage",          "label": "Floor",            "tag": "FLOOR",    "clearMsg": "floor damage stabilised"},
    {"key": "diffuserDamage",       "label": "Diffuser",         "tag": "DIFF",     "clearMsg": "diffuser damage stabilised"},
    {"key": "sidepodDamage",        "label": "Sidepod",          "tag": "SIDEPOD",  "clearMsg": "sidepod damage stabilised"},
]

BRAKE_TEMPS = [
    {"key": "brakeTempFL", "label": "FL brake", "tag": "BRK FL"},
    {"key": "brakeTempFR", "label": "FR brake", "tag": "BRK FR"},
    {"key": "brakeTempRL", "label": "RL brake", "tag": "BRK RL"},
    {"key": "brakeTempRR", "label": "RR brake", "tag": "BRK RR"},
]


def _format_session_time(seconds: float) -> str:
    total = int(seconds)
    m = total // 60
    s = total % 60
    return f"{m:02d}:{s:02d}"


def _process_aero_alerts(aero: dict) -> None:
    """Port of useAeroAlerts.ts — accumulates alerts into aero_alerts_log."""
    global aero_alert_conditions, aero_alerts_log, prev_aero_session_time, damage_agent_response

    st = aero.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_aero_session_time:
        aero_alert_conditions = {}
        aero_alerts_log = []
        damage_agent_response = None
    prev_aero_session_time = st

    new_alerts: list[dict] = []
    current_conditions: dict = {}
    ts = _format_session_time(st)

    # Damage conditions
    for part in DAMAGE_PARTS:
        val = aero.get(part["key"], 0)
        if val > 50:
            current_conditions[part["key"]] = {"level": "crit", "value": val}
        elif val > 20:
            current_conditions[part["key"]] = {"level": "warn", "value": val}

    # Brake temperatures
    for brk in BRAKE_TEMPS:
        val = aero.get(brk["key"], 0)
        if val > 1000:
            current_conditions[brk["key"]] = {"level": "crit", "value": val}
        elif val > 800:
            current_conditions[brk["key"]] = {"level": "warn", "value": val}

    # DRS fault
    if aero.get("drsFault"):
        current_conditions["drsFault"] = {"level": "crit", "value": 1}

    # Fire alerts for new, escalated, or worsened conditions
    for key, cur in current_conditions.items():
        prev = aero_alert_conditions.get(key)
        is_new = prev is None
        is_escalation = prev is not None and prev["level"] == "warn" and cur["level"] == "crit"

        part = next((p for p in DAMAGE_PARTS if p["key"] == key), None)
        is_worsened = (
            part is not None
            and prev is not None
            and prev["level"] == cur["level"]
            and cur["value"] >= prev["value"] + DAMAGE_REFIRE_STEP
        )

        if is_new or is_escalation or is_worsened:
            alert_level = "critical" if cur["level"] == "crit" else "warning"

            if key == "drsFault":
                new_alerts.append({
                    "level": "critical",
                    "message": "DRS system fault detected",
                    "time": f"{ts} DRS",
                })
            elif part is not None:
                msg = (
                    f"{part['label']} damage critical ({cur['value']}%)"
                    if cur["level"] == "crit"
                    else f"{part['label']} damage detected ({cur['value']}%)"
                )
                new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {part['tag']}"})
            else:
                brk = next((b for b in BRAKE_TEMPS if b["key"] == key), None)
                if brk is not None:
                    msg = (
                        f"{brk['label']} temp {cur['value']}\u00b0C \u2014 overheating"
                        if cur["level"] == "crit"
                        else f"{brk['label']} temp {cur['value']}\u00b0C \u2014 running hot"
                    )
                    new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {brk['tag']}"})
        elif prev is not None:
            # Keep the previous alerted value as baseline when no alert fires
            cur["value"] = prev["value"]

    # Detect fully cleared conditions
    for key in aero_alert_conditions:
        if key not in current_conditions:
            if key == "drsFault":
                new_alerts.append({"level": "info", "message": "DRS fault cleared", "time": f"{ts} DRS"})
            else:
                part = next((p for p in DAMAGE_PARTS if p["key"] == key), None)
                if part is not None:
                    new_alerts.append({"level": "info", "message": part["clearMsg"], "time": f"{ts} {part['tag']}"})
                else:
                    brk = next((b for b in BRAKE_TEMPS if b["key"] == key), None)
                    if brk is not None:
                        new_alerts.append({"level": "info", "message": f"{brk['label']} temp back to normal", "time": f"{ts} {brk['tag']}"})

    aero_alert_conditions = current_conditions
    aero_alerts_log.extend(new_alerts)
    return new_alerts


# ---------------------------------------------------------------------------
# Tire alert constants & processing
# ---------------------------------------------------------------------------
TIRE_DAMAGE_REFIRE_STEP = 25  # re-alert every step when worsening (0-255 scale)

TIRE_WHEEL_LABELS = {"fl": "FL", "fr": "FR", "rl": "RL", "rr": "RR"}
TIRE_WHEELS = ("fl", "fr", "rl", "rr")
TIRE_DAMAGE_METRICS = {"dmg", "blst", "wear"}

TIRE_CLEAR_LABELS = {
    "temp": "temp back to normal",
    "wear": "tyre wear stabilised",
    "dmg": "tyre damage stabilised",
    "blst": "blistering subsided",
}


def _process_tire_alerts(tire_snapshot: dict) -> None:
    """Port of useTireAlerts.ts — accumulates alerts into tire_alerts_log."""
    global tire_alert_conditions, tire_alerts_log, prev_tire_session_time, prev_tire_compound

    st = tire_snapshot.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_tire_session_time:
        tire_alert_conditions = {}
        tire_alerts_log = []
        prev_tire_compound = ""
    prev_tire_session_time = st

    new_alerts: list[dict] = []
    current_conditions: dict = {}
    ts = _format_session_time(st)
    tires = tire_snapshot.get("tires", {})

    for wn in TIRE_WHEELS:
        t = tires.get(wn, {})
        surface_temp = t.get("surfaceTemp", 0)
        wear = t.get("wear", 0)
        damage = t.get("damage", 0)
        blisters = t.get("blisters", 0)
        life = round(100 - wear)

        # Surface temperature
        if surface_temp > 108:
            current_conditions[f"{wn}_temp"] = {"level": "crit", "value": surface_temp}
        elif surface_temp > 103:
            current_conditions[f"{wn}_temp"] = {"level": "warn", "value": surface_temp}

        # Wear (value = wear amount, higher = worse)
        if life <= 10:
            current_conditions[f"{wn}_wear"] = {"level": "crit", "value": wear}
        elif life <= 25:
            current_conditions[f"{wn}_wear"] = {"level": "warn", "value": wear}

        # Damage
        if damage > 150:
            current_conditions[f"{wn}_dmg"] = {"level": "crit", "value": damage}
        elif damage > 50:
            current_conditions[f"{wn}_dmg"] = {"level": "warn", "value": damage}

        # Blisters
        if blisters > 150:
            current_conditions[f"{wn}_blst"] = {"level": "crit", "value": blisters}
        elif blisters > 50:
            current_conditions[f"{wn}_blst"] = {"level": "warn", "value": blisters}

    # Fire alerts for new, escalated, or worsened conditions
    for key, cur in current_conditions.items():
        prev = tire_alert_conditions.get(key)
        underscore_idx = key.index("_")
        wn = key[:underscore_idx]
        metric = key[underscore_idx + 1:]
        label = TIRE_WHEEL_LABELS[wn]
        t = tires.get(wn, {})
        life = round(100 - t.get("wear", 0))
        tag = metric.upper()

        is_new = prev is None
        is_escalation = prev is not None and prev["level"] == "warn" and cur["level"] == "crit"
        is_worsened = (
            metric in TIRE_DAMAGE_METRICS
            and prev is not None
            and prev["level"] == cur["level"]
            and cur["value"] >= prev["value"] + TIRE_DAMAGE_REFIRE_STEP
        )

        if is_new or is_escalation or is_worsened:
            alert_level = "critical" if cur["level"] == "crit" else "warning"

            if metric == "temp":
                msg = (
                    f"{label} surface temp {t.get('surfaceTemp', 0)}\u00b0C \u2014 overheating"
                    if cur["level"] == "crit"
                    else f"{label} surface temp {t.get('surfaceTemp', 0)}\u00b0C \u2014 approaching limit"
                )
                new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {tag}"})
            elif metric == "wear":
                msg = (
                    f"{label} tyre life critically low at {life}%"
                    if cur["level"] == "crit"
                    else f"{label} tyre life low at {life}%"
                )
                new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {tag}"})
            elif metric == "dmg":
                msg = (
                    f"{label} tyre damage critical ({t.get('damage', 0)}/255)"
                    if cur["level"] == "crit"
                    else f"{label} tyre damage detected ({t.get('damage', 0)}/255)"
                )
                new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {tag}"})
            elif metric == "blst":
                msg = (
                    f"{label} severe blistering ({t.get('blisters', 0)}/255)"
                    if cur["level"] == "crit"
                    else f"{label} blistering detected ({t.get('blisters', 0)}/255)"
                )
                new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {tag}"})
        elif prev is not None:
            # Keep the previous alerted value as baseline
            cur["value"] = prev["value"]

    # Detect fully cleared conditions
    for key in tire_alert_conditions:
        if key not in current_conditions:
            underscore_idx = key.index("_")
            wn = key[:underscore_idx]
            metric = key[underscore_idx + 1:]
            label = TIRE_WHEEL_LABELS.get(wn, wn.upper())
            tag = metric.upper()
            clear_msg = TIRE_CLEAR_LABELS.get(metric)
            if clear_msg:
                new_alerts.append({"level": "info", "message": f"{label} {clear_msg}", "time": f"{ts} {tag}"})

    # Compound change
    compound = tire_snapshot.get("compound", "")
    compound_visual = tire_snapshot.get("compoundVisual", "")
    compound_key = f"{compound}_{compound_visual}"
    if compound and compound_key != prev_tire_compound:
        prev_tire_compound = compound_key
        new_alerts.append({
            "level": "info",
            "message": f"{compound_visual.upper()} ({compound}) fitted",
            "time": f"{ts} TYRE",
        })

    tire_alert_conditions = current_conditions
    tire_alerts_log.extend(new_alerts)


# ---------------------------------------------------------------------------
# Power unit alert constants & processing
# ---------------------------------------------------------------------------
PU_DAMAGE_REFIRE_STEP = 10  # re-alert every 10% worsening
PU_DAMAGE_KEYS = {"eng_dmg", "gbx_dmg"}

PU_ALERT_DEFS = {
    "eng_temp": {"tag": "TEMP", "clearMsg": "engine temp back to normal"},
    "eng_dmg":  {"tag": "ICE",  "clearMsg": "engine damage stabilised"},
    "gbx_dmg":  {"tag": "GBX",  "clearMsg": "gearbox damage stabilised"},
    "fuel":     {"tag": "FUEL", "clearMsg": "fuel delta recovered"},
    "battery":  {"tag": "ERS",  "clearMsg": "battery SOC recovered"},
}


def _process_pu_alerts(pu: dict) -> None:
    """Port of usePowerUnitAlerts.ts — accumulates alerts into pu_alerts_log."""
    global pu_alert_conditions, pu_alerts_log, prev_pu_session_time
    global prev_pu_ers_mode, prev_pu_fuel_mix

    st = pu.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_pu_session_time:
        pu_alert_conditions = {}
        pu_alerts_log = []
        prev_pu_ers_mode = ""
        prev_pu_fuel_mix = ""
    prev_pu_session_time = st

    new_alerts: list[dict] = []
    current_conditions: dict = {}
    ts = _format_session_time(st)

    engine_temp = pu.get("engineTemp", 0)
    engine_damage = pu.get("engineDamage", 0)
    gearbox_damage = pu.get("gearboxDamage", 0)
    fuel_remaining_laps = pu.get("fuelRemainingLaps", 0)
    battery_pct = pu.get("batteryPct", 0)

    # Engine temperature
    if engine_temp > 130:
        current_conditions["eng_temp"] = {"level": "crit", "value": engine_temp}
    elif engine_temp > 120:
        current_conditions["eng_temp"] = {"level": "warn", "value": engine_temp}

    # Engine damage (0-100)
    if engine_damage > 20:
        current_conditions["eng_dmg"] = {"level": "crit", "value": engine_damage}
    elif engine_damage > 5:
        current_conditions["eng_dmg"] = {"level": "warn", "value": engine_damage}

    # Gearbox damage (0-100)
    if gearbox_damage > 20:
        current_conditions["gbx_dmg"] = {"level": "crit", "value": gearbox_damage}
    elif gearbox_damage > 5:
        current_conditions["gbx_dmg"] = {"level": "warn", "value": gearbox_damage}

    # Fuel remaining laps
    if fuel_remaining_laps < 1:
        current_conditions["fuel"] = {"level": "crit", "value": fuel_remaining_laps}
    elif fuel_remaining_laps < 3:
        current_conditions["fuel"] = {"level": "warn", "value": fuel_remaining_laps}

    # Battery SOC
    if battery_pct < 15:
        current_conditions["battery"] = {"level": "crit", "value": battery_pct}
    elif battery_pct < 30:
        current_conditions["battery"] = {"level": "warn", "value": battery_pct}

    # Fire alerts for new, escalated, or worsened conditions
    for key, cur in current_conditions.items():
        prev = pu_alert_conditions.get(key)
        defn = PU_ALERT_DEFS.get(key)
        if not defn:
            continue

        is_new = prev is None
        is_escalation = prev is not None and prev["level"] == "warn" and cur["level"] == "crit"
        is_worsened = (
            key in PU_DAMAGE_KEYS
            and prev is not None
            and prev["level"] == cur["level"]
            and cur["value"] >= prev["value"] + PU_DAMAGE_REFIRE_STEP
        )

        if is_new or is_escalation or is_worsened:
            alert_level = "critical" if cur["level"] == "crit" else "warning"

            if key == "eng_temp":
                msg = (
                    f"Engine temp {engine_temp}\u00b0C \u2014 overheating"
                    if cur["level"] == "crit"
                    else f"Engine temp {engine_temp}\u00b0C \u2014 running hot"
                )
            elif key == "eng_dmg":
                msg = (
                    f"Engine damage critical ({engine_damage}%)"
                    if cur["level"] == "crit"
                    else f"Engine damage detected ({engine_damage}%)"
                )
            elif key == "gbx_dmg":
                msg = (
                    f"Gearbox damage critical ({gearbox_damage}%)"
                    if cur["level"] == "crit"
                    else f"Gearbox damage detected ({gearbox_damage}%)"
                )
            elif key == "fuel":
                msg = (
                    f"Fuel critically low \u2014 {fuel_remaining_laps:.1f} laps remaining"
                    if cur["level"] == "crit"
                    else f"Fuel running low \u2014 {fuel_remaining_laps:.1f} laps remaining"
                )
            elif key == "battery":
                msg = (
                    f"Battery SOC critically low at {battery_pct}%"
                    if cur["level"] == "crit"
                    else f"Battery SOC low at {battery_pct}%"
                )
            else:
                continue

            new_alerts.append({"level": alert_level, "message": msg, "time": f"{ts} {defn['tag']}"})
        elif prev is not None:
            # Keep the previous alerted value as baseline
            cur["value"] = prev["value"]

    # Detect fully cleared conditions
    for key in pu_alert_conditions:
        if key not in current_conditions:
            defn = PU_ALERT_DEFS.get(key)
            if defn:
                new_alerts.append({"level": "info", "message": defn["clearMsg"], "time": f"{ts} {defn['tag']}"})

    # ERS deploy mode change
    ers_mode = pu.get("ersDeployMode", "")
    if ers_mode and ers_mode != prev_pu_ers_mode:
        if prev_pu_ers_mode:
            new_alerts.append({
                "level": "info",
                "message": f"ERS mode \u2192 {ers_mode.upper()}",
                "time": f"{ts} ERS",
            })
        prev_pu_ers_mode = ers_mode

    # Fuel mix change
    fuel_mix = pu.get("fuelMix", "")
    if fuel_mix and fuel_mix != prev_pu_fuel_mix:
        if prev_pu_fuel_mix:
            new_alerts.append({
                "level": "info",
                "message": f"Fuel mix \u2192 {fuel_mix.upper()}",
                "time": f"{ts} FUEL",
            })
        prev_pu_fuel_mix = fuel_mix

    pu_alert_conditions = current_conditions
    pu_alerts_log.extend(new_alerts)


def _queue_damage_alerts(alert_texts: list[str]) -> None:
    """Add alert texts to the pending list and (re)start the batch timer.

    Alerts that arrive within DAMAGE_AGENT_BATCH_DELAY seconds of each
    other are grouped into a single DamageAgent call.
    """
    global damage_agent_batch_handle

    damage_agent_pending.extend(alert_texts)

    # Reset the debounce timer so we keep waiting for more alerts
    if damage_agent_batch_handle is not None:
        damage_agent_batch_handle.cancel()

    loop = asyncio.get_running_loop()
    damage_agent_batch_handle = loop.call_later(
        DAMAGE_AGENT_BATCH_DELAY,
        lambda: asyncio.ensure_future(_flush_damage_agent()),
    )


async def _flush_damage_agent() -> None:
    """Drain the pending list and send the batch to the DamageAgent.

    Uses an in-flight guard so only one agent call runs at a time.
    If new alerts accumulate while the agent is busy, they are
    dispatched when the current call completes.
    """
    global damage_agent_response, damage_agent_in_flight, damage_agent_pending
    global damage_agent_batch_handle

    damage_agent_batch_handle = None

    if damage_agent is None or not damage_agent_pending:
        return

    if damage_agent_in_flight:
        # The in-flight call's finally-block will re-flush.
        return

    batch = damage_agent_pending
    damage_agent_pending = []

    damage_agent_in_flight = True
    try:
        response = await asyncio.to_thread(damage_agent.process_messages, batch)
        damage_agent_response = response
    except Exception as exc:
        print(f"DamageAgent error: {exc}")
    finally:
        damage_agent_in_flight = False

    # If more alerts arrived while we were busy, flush again
    if damage_agent_pending:
        await _flush_damage_agent()


def build_message() -> tuple[str, list[dict]]:
    """Build a JSON message from the latest merged state."""

    # -- Aero alerts (must run before we build the aero dict) --
    # We build the aero snapshot first so _process_aero_alerts can read it,
    # then include it in the final payload.
    aero_snapshot = {
        "sessionTime": session_time,
        "frontLeftWingDamage": damage_state.get("front_left_wing_damage", 0),
        "frontRightWingDamage": damage_state.get("front_right_wing_damage", 0),
        "rearWingDamage": damage_state.get("rear_wing_damage", 0),
        "floorDamage": damage_state.get("floor_damage", 0),
        "diffuserDamage": damage_state.get("diffuser_damage", 0),
        "sidepodDamage": damage_state.get("sidepod_damage", 0),
        "drsFault": bool(damage_state.get("drs_fault", 0)),
        "brakeTempFL": telemetry_state.get("brake_temp_fl", 0),
        "brakeTempFR": telemetry_state.get("brake_temp_fr", 0),
        "brakeTempRL": telemetry_state.get("brake_temp_rl", 0),
        "brakeTempRR": telemetry_state.get("brake_temp_rr", 0),
    }
    new_aero_alerts = _process_aero_alerts(aero_snapshot)

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

    # -- Tire alerts --
    _process_tire_alerts({
        "sessionTime": session_time,
        "tires": tires,
        "compound": status_state.get("tyre_compound_actual", ""),
        "compoundVisual": status_state.get("tyre_compound_visual", ""),
    })

    # -- Power Unit --
    ERS_MAX_ENERGY_J = 4_000_000  # 4 MJ per F1 regulations
    ers_store = status_state.get("ers_store_energy", 0)
    battery_pct = round((ers_store / ERS_MAX_ENERGY_J) * 100, 1) if ERS_MAX_ENERGY_J else 0

    FUEL_MIX_LABELS = {0: "LEAN", 1: "STANDARD", 2: "RICH", 3: "MAX"}
    ERS_MODE_LABELS = {0: "NONE", 1: "MEDIUM", 2: "HOTLAP", 3: "OVERTAKE"}

    power_unit = {
        "rpm": telemetry_state.get("engine_rpm", 0),
        "engineTemp": telemetry_state.get("engine_temp", 0),
        "gear": telemetry_state.get("gear", 0),
        "fuelInTank": round(status_state.get("fuel_in_tank", 0), 2),
        "fuelRemainingLaps": round(status_state.get("fuel_remaining_laps", 0), 1),
        "fuelMix": FUEL_MIX_LABELS.get(status_state.get("fuel_mix", 1), "STANDARD"),
        "icePowerKW": round(status_state.get("engine_power_ice", 0), 1),
        "mgukPowerKW": round(status_state.get("engine_power_mguk", 0), 1),
        "ersStoreEnergy": round(ers_store, 0),
        "batteryPct": battery_pct,
        "ersDeployMode": ERS_MODE_LABELS.get(
            status_state.get("ers_deploy_mode", 0), "NONE"
        ),
        "ersDeployedThisLap": round(status_state.get("ers_deployed_this_lap", 0), 0),
        "ersHarvestedMGUK": round(status_state.get("ers_harvested_mguk", 0), 0),
        "ersHarvestedMGUH": round(status_state.get("ers_harvested_mguh", 0), 0),
        "engineDamage": damage_state.get("engine_damage", 0),
        "gearboxDamage": damage_state.get("gearbox_damage", 0),
    }

    # -- Power unit alerts --
    _process_pu_alerts({**power_unit, "sessionTime": session_time})

    # -- Aero --
    aero = {
        "speed": telemetry_state.get("speed_kmh", 0),
        "drs": bool(telemetry_state.get("drs", 0)),
        "drsAllowed": bool(status_state.get("drs_allowed", 0)),
        "drsActivationDistance": status_state.get("drs_activation_distance", 0),
        "frontWing": setup_state.get("front_wing", 0),
        "rearWing": setup_state.get("rear_wing", 0),
        "frontRideHeight": motion_ex_state.get("front_aero_height", 0),
        "rearRideHeight": motion_ex_state.get("rear_aero_height", 0),
        "brakeBias": status_state.get("front_brake_bias", 0),
        "frontLeftWingDamage": damage_state.get("front_left_wing_damage", 0),
        "frontRightWingDamage": damage_state.get("front_right_wing_damage", 0),
        "rearWingDamage": damage_state.get("rear_wing_damage", 0),
        "floorDamage": damage_state.get("floor_damage", 0),
        "diffuserDamage": damage_state.get("diffuser_damage", 0),
        "sidepodDamage": damage_state.get("sidepod_damage", 0),
        "drsFault": bool(damage_state.get("drs_fault", 0)),
        "brakeTempFL": telemetry_state.get("brake_temp_fl", 0),
        "brakeTempFR": telemetry_state.get("brake_temp_fr", 0),
        "brakeTempRL": telemetry_state.get("brake_temp_rl", 0),
        "brakeTempRR": telemetry_state.get("brake_temp_rr", 0),
    }

    # -- Session --
    session_type = session_state.get("session_type", 0)
    track_id = session_state.get("track_id", -1)
    last_lap_ms = lap_state.get("last_lap_time_ms", 0)

    session = {
        "sessionType": SESSION_TYPE_LABELS.get(session_type, f"SESSION {session_type}"),
        "trackName": TRACK_NAMES.get(track_id, f"TRACK {track_id}"),
        "totalLaps": session_state.get("total_laps", 0),
        "sessionTimeLeft": session_state.get("session_time_left", 0),
        "sessionDuration": session_state.get("session_duration", 0),
        "trackTemp": session_state.get("track_temperature", 0),
        "airTemp": session_state.get("air_temperature", 0),
        "weather": session_state.get("weather", 0),
        "carPosition": lap_state.get("car_position", 0),
        "currentLapTimeMs": lap_state.get("current_lap_time_ms", 0),
        "lastLapTimeMs": last_lap_ms,
    }

    msg = json.dumps({
        "tires": tires,
        "compound": status_state.get("tyre_compound_actual", ""),
        "compoundVisual": status_state.get("tyre_compound_visual", ""),
        "tyresAgeLaps": status_state.get("tyres_age_laps", 0),
        "currentLap": lap_state.get("current_lap_num", 0),
        "speed": telemetry_state.get("speed_kmh", 0),
        "sessionTime": session_time,
        "powerUnit": power_unit,
        "aero": aero,
        "session": session,
        "tireAlerts": {
            "alerts": tire_alerts_log,
            "activeCount": len(tire_alert_conditions),
        },
        "puAlerts": {
            "alerts": pu_alerts_log,
            "activeCount": len(pu_alert_conditions),
        },
        "aeroAlerts": {
            "alerts": aero_alerts_log,
            "activeCount": len(aero_alert_conditions),
        },
        "damageReport": {
            "response": damage_agent_response,
        },
    })
    return msg, new_aero_alerts


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
        msg, _new_alerts = build_message()
        await websocket.send(msg)
        # Keep connection alive – we only push, client doesn't send
        async for _ in websocket:
            pass
    finally:
        connected_clients.discard(websocket)
        print(f"Client disconnected ({len(connected_clients)} total)")


async def udp_reader():
    """Read F1 25 UDP packets and update shared state, broadcasting on each
    telemetry frame (packet 6)."""
    global lap_state, status_state, damage_state, telemetry_state, setup_state, motion_ex_state, session_state, session_time

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

        # -- Session (single block, not per-car) --
        if packet_id == PACKET_ID_SESSION:
            if len(data) < HEADER_SIZE + SESSION_HEADER_SIZE:
                continue
            fields = SESSION_HEADER_STRUCT.unpack_from(data, HEADER_SIZE)
            session_state = {
                "session_type": fields[5],
                "track_id": fields[6],
                "total_laps": fields[3],
                "session_time_left": fields[8],
                "session_duration": fields[9],
                "track_temperature": fields[1],
                "air_temperature": fields[2],
                "weather": fields[0],
            }
            continue

        # -- Lap Data --
        if packet_id == PACKET_ID_LAP_DATA:
            base = HEADER_SIZE + player_car_index * LAPDATA_SIZE
            if len(data) < base + LAPDATA_SIZE:
                continue
            fields = LAPDATA_STRUCT.unpack_from(data, base)
            lap_state = {
                "current_lap_num": fields[14],
                "lap_distance_m": round(fields[10], 2),
                "car_position": fields[13],
                "last_lap_time_ms": fields[0],
                "current_lap_time_ms": fields[1],
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
                # Power unit fields from CarStatus
                "fuel_in_tank": fields[5],
                "fuel_remaining_laps": fields[7],
                "fuel_mix": fields[2],
                "engine_power_ice": fields[17],
                "engine_power_mguk": fields[18],
                "ers_store_energy": fields[19],
                "ers_deploy_mode": fields[20],
                "ers_harvested_mguk": fields[21],
                "ers_harvested_mguh": fields[22],
                "ers_deployed_this_lap": fields[23],
                # Aero fields from CarStatus
                "front_brake_bias": fields[3],
                "drs_allowed": fields[11],
                "drs_activation_distance": fields[12],
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
            # Power unit damage fields
            new_damage["engine_damage"] = fields[25]
            new_damage["gearbox_damage"] = fields[24]
            # Aero damage fields
            new_damage["front_left_wing_damage"] = fields[16]
            new_damage["front_right_wing_damage"] = fields[17]
            new_damage["rear_wing_damage"] = fields[18]
            new_damage["floor_damage"] = fields[19]
            new_damage["diffuser_damage"] = fields[20]
            new_damage["sidepod_damage"] = fields[21]
            new_damage["drs_fault"] = fields[22]
            damage_state = new_damage
            continue

        # -- Car Setups --
        if packet_id == PACKET_ID_CAR_SETUPS:
            base = HEADER_SIZE + player_car_index * CAR_SETUP_SIZE
            if len(data) < base + CAR_SETUP_SIZE:
                continue
            fields = CAR_SETUP_STRUCT.unpack_from(data, base)
            setup_state = {
                "front_wing": fields[0],
                "rear_wing": fields[1],
                "front_suspension_height": fields[12],
                "rear_suspension_height": fields[13],
                "brake_bias": fields[15],
            }
            continue

        # -- Motion Ex (player only, no per-car offset) --
        if packet_id == PACKET_ID_MOTION_EX:
            if len(data) < HEADER_SIZE + MOTION_EX_SIZE:
                continue
            fields = MOTION_EX_STRUCT.unpack_from(data, HEADER_SIZE)
            motion_ex_state = {
                "front_aero_height": round(fields[47] * 1000, 1),  # m -> mm
                "rear_aero_height": round(fields[48] * 1000, 1),
                "front_roll_angle": round(fields[49], 4),
                "rear_roll_angle": round(fields[50], 4),
            }
            continue

        # -- Car Telemetry (main trigger) --
        if packet_id == PACKET_ID_CAR_TELEMETRY:
            base = HEADER_SIZE + player_car_index * CAR_TELEMETRY_SIZE
            if len(data) < base + CAR_TELEMETRY_SIZE:
                continue
            fields = CAR_TELEMETRY_STRUCT.unpack_from(data, base)

            telem = {
                "speed_kmh": fields[0],
                "engine_rpm": fields[6],
                "engine_temp": fields[22],
                "gear": fields[5],
                "drs": fields[7],
            }
            for i, wn in enumerate(WHEEL_NAMES):
                telem[f"brake_temp_{wn}"] = fields[10 + i]
                telem[f"tyre_surface_temp_{wn}"] = fields[14 + i]
                telem[f"tyre_inner_temp_{wn}"] = fields[18 + i]
                telem[f"tyre_pressure_{wn}"] = round(fields[23 + i], 2)
            telemetry_state = telem
            session_time = header[7]  # m_sessionTime (float)

            # Broadcast merged state to all WS clients
            msg, new_aero_alerts = build_message()
            await broadcast(msg)

            # Dispatch new aero alerts to DamageAgent
            if new_aero_alerts:
                alert_texts = [
                    f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
                    for a in new_aero_alerts
                ]
                _queue_damage_alerts(alert_texts)

            # Write CSV row if capture is enabled
            if csv_capture is not None:
                session_uid = header[6]   # m_sessionUID (uint64)
                frame_id = header[8]      # m_frameIdentifier
                csv_capture.write_row(session_uid, session_time, frame_id)


# ---------------------------------------------------------------------------
# CSV replay
# ---------------------------------------------------------------------------
REVERSE_SESSION_TYPE = {v: k for k, v in SESSION_TYPE_LABELS.items()}
REVERSE_TRACK = {v: k for k, v in TRACK_NAMES.items()}


def _float(val, default=0.0):
    try:
        return float(val)
    except (ValueError, TypeError):
        return default


def _int(val, default=0):
    try:
        return int(float(val))
    except (ValueError, TypeError):
        return default


def _populate_state_from_row(row: dict):
    """Fill the shared state dicts from a single CSV row."""
    global lap_state, status_state, damage_state, telemetry_state
    global setup_state, motion_ex_state, session_state, session_time

    session_time = _float(row.get("session_time"))

    session_state = {
        "session_type": REVERSE_SESSION_TYPE.get(row.get("session_type", ""), 0),
        "track_id": REVERSE_TRACK.get(row.get("track_name", ""), -1),
        "session_time_left": _int(row.get("session_time_left")),
        "total_laps": 0,
        "session_duration": 0,
        "track_temperature": 0,
        "air_temperature": 0,
        "weather": 0,
    }

    lap_state = {
        "current_lap_num": _int(row.get("current_lap_num")),
        "lap_distance_m": _float(row.get("lap_distance_m")),
        "car_position": _int(row.get("car_position")),
        "last_lap_time_ms": _int(row.get("last_lap_time_ms")),
        "current_lap_time_ms": _int(row.get("current_lap_time_ms")),
    }

    status_state = {
        "tyre_compound_actual": row.get("tyre_compound_actual", ""),
        "tyre_compound_visual": row.get("tyre_compound_visual", ""),
        "tyres_age_laps": _int(row.get("tyres_age_laps")),
        "fuel_in_tank": _float(row.get("fuel_in_tank")),
        "fuel_remaining_laps": _float(row.get("fuel_remaining_laps")),
        "fuel_mix": _int(row.get("fuel_mix")),
        "engine_power_ice": _float(row.get("engine_power_ice_w")),
        "engine_power_mguk": _float(row.get("engine_power_mguk_w")),
        "ers_store_energy": _float(row.get("ers_store_energy_j")),
        "ers_deploy_mode": _int(row.get("ers_deploy_mode")),
        "ers_harvested_mguk": _float(row.get("ers_harvested_mguk_j")),
        "ers_harvested_mguh": _float(row.get("ers_harvested_mguh_j")),
        "ers_deployed_this_lap": _float(row.get("ers_deployed_this_lap_j")),
        "front_brake_bias": _int(row.get("brake_bias")),
        "drs_allowed": 0,
        "drs_activation_distance": 0,
    }

    new_damage = {}
    for wn in WHEEL_NAMES:
        new_damage[f"tyre_wear_{wn}"] = _float(row.get(f"tyre_wear_{wn}"))
        new_damage[f"tyre_damage_{wn}"] = _int(row.get(f"tyre_damage_{wn}"))
        new_damage[f"tyre_blisters_{wn}"] = _int(row.get(f"tyre_blisters_{wn}"))
    new_damage["engine_damage"] = _int(row.get("engine_damage"))
    new_damage["gearbox_damage"] = _int(row.get("gearbox_damage"))
    new_damage["front_left_wing_damage"] = _int(row.get("front_left_wing_damage"))
    new_damage["front_right_wing_damage"] = _int(row.get("front_right_wing_damage"))
    new_damage["rear_wing_damage"] = _int(row.get("rear_wing_damage"))
    new_damage["floor_damage"] = _int(row.get("floor_damage"))
    new_damage["diffuser_damage"] = _int(row.get("diffuser_damage"))
    new_damage["sidepod_damage"] = _int(row.get("sidepod_damage"))
    new_damage["drs_fault"] = _int(row.get("drs_fault"))
    damage_state = new_damage

    telem = {
        "speed_kmh": _int(row.get("speed_kmh")),
        "engine_rpm": _int(row.get("engine_rpm")),
        "engine_temp": _int(row.get("engine_temp")),
        "gear": _int(row.get("gear")),
        "drs": _int(row.get("drs")),
    }
    for wn in WHEEL_NAMES:
        telem[f"brake_temp_{wn}"] = _int(row.get(f"brake_temp_{wn}"))
        telem[f"tyre_surface_temp_{wn}"] = _int(row.get(f"tyre_surface_temp_{wn}"))
        telem[f"tyre_inner_temp_{wn}"] = _int(row.get(f"tyre_inner_temp_{wn}"))
        telem[f"tyre_pressure_{wn}"] = _float(row.get(f"tyre_pressure_{wn}"))
    telemetry_state = telem

    setup_state = {
        "front_wing": _int(row.get("front_wing_setup")),
        "rear_wing": _int(row.get("rear_wing_setup")),
        "brake_bias": _int(row.get("brake_bias")),
    }

    motion_ex_state = {
        "front_aero_height": _float(row.get("front_ride_height_mm")),
        "rear_aero_height": _float(row.get("rear_ride_height_mm")),
    }


async def csv_replay(filepath: str, speed: int = 1):
    """Replay a captured CSV file as if it were live telemetry."""
    path = Path(filepath)
    if not path.exists():
        print(f"ERROR: file not found: {filepath}")
        return

    print(f"Replaying {filepath} ({speed}x) ...")

    with path.open(newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        prev_wall_time = None
        row_count = 0

        for row in reader:
            wall_time = _float(row.get("wall_time"))

            # Pace replay using original timing, scaled by speed
            if prev_wall_time is not None and wall_time > prev_wall_time:
                delta = (wall_time - prev_wall_time) / speed
                # Cap to 1s to skip long pauses (e.g. game paused)
                await asyncio.sleep(min(delta, 1.0))
            prev_wall_time = wall_time

            _populate_state_from_row(row)
            msg, new_aero_alerts = build_message()
            await broadcast(msg)

            # Dispatch new aero alerts to DamageAgent
            if new_aero_alerts:
                alert_texts = [
                    f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
                    for a in new_aero_alerts
                ]
                _queue_damage_alerts(alert_texts)

            row_count += 1

    print(f"Replay complete — {row_count} frames sent.")


async def main():
    global csv_capture, damage_agent

    parser = argparse.ArgumentParser(description="F1 25 telemetry WebSocket bridge")
    parser.add_argument(
        "--capture",
        action="store_true",
        help="Save telemetry to CSV files in the data/ folder (one file per session)",
    )
    parser.add_argument(
        "--replay",
        metavar="FILE",
        help="Replay a captured CSV file instead of listening for live UDP",
    )
    parser.add_argument(
        "--speed",
        metavar="Nx",
        default="1x",
        help="Replay speed multiplier, e.g. 2x, 10x (default: 1x)",
    )
    args = parser.parse_args()

    if args.capture and args.replay:
        parser.error("--capture and --replay cannot be used together")

    if args.speed != "1x" and not args.replay:
        parser.error("--speed can only be used with --replay")

    if args.capture:
        csv_capture = CsvCapture(Path("data"))
        print("CSV capture enabled → data/")

    os.environ.setdefault("AGENT_MANIFEST_FILE", "registries/manifest.hocon")
    try:
        damage_agent = DamageAgent()
        print("DamageAgent initialized")
    except Exception as exc:
        print(f"DamageAgent unavailable: {exc}")

    print(f"Starting WebSocket server on ws://{WS_HOST}:{WS_PORT}")
    try:
        async with websockets.serve(ws_handler, WS_HOST, WS_PORT):
            if args.replay:
                speed_str = args.speed.rstrip("x")
                try:
                    speed = int(speed_str)
                except ValueError:
                    parser.error(f"invalid --speed value: {args.speed} (expected Nx, e.g. 2x, 10x)")
                await csv_replay(args.replay, speed)
            else:
                await udp_reader()
    finally:
        if csv_capture is not None:
            csv_capture.close()


if __name__ == "__main__":
    asyncio.run(main())
