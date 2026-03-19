"""WebSocket bridge that streams live F1 25 telemetry from UDP to the
race_engineer_hub web app.

Usage:
    python -m telemetry_server

Listens on UDP 20777 for F1 25 telemetry packets (decoded via
``common.f1_decoder.PacketDecoder``) and exposes a WebSocket server on
port 8765.  The web app connects to ws://localhost:8765 and receives
JSON frames with the latest telemetry state.
"""

import argparse
import asyncio
import json
import os
import time
from pathlib import Path

import websockets

from common.f1_capture.binary_writer import BinaryWriter
from common.f1_capture.replay_session import ReplaySession
from common.f1_capture.udp_listener import UdpListener
from common.f1_decoder.packet_decoder import PacketDecoder
from telemetry_server.agents.damage_agent import DamageAgent
from telemetry_server.agents.power_unit_agent import PowerUnitAgent
from telemetry_server.agents.race_engineer_agent import RaceEngineerAgent
from telemetry_server.agents.tyres_agent import TyresAgent
from telemetry_server.telemetry_state_adapter import TelemetryStateAdapter

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
UDP_PORT = 20777
WS_HOST = "0.0.0.0"
WS_PORT = 8765

# ---------------------------------------------------------------------------
# Shared state – written by the UDP/replay reader, read by WebSocket handlers
# ---------------------------------------------------------------------------
state: dict[int, dict] = {}
adapter: TelemetryStateAdapter = TelemetryStateAdapter(state)
session_time: float = 0.0
prev_session_uid: int = 0

# Alert state – computed server-side and sent pre-computed via WebSocket
aero_alert_conditions: dict = {}   # key -> {"level": "warn"|"crit", "value": float}
aero_alerts_log: list = []         # accumulated alerts
prev_aero_session_time: float = 0.0

tyre_alert_conditions: dict = {}
tyre_alerts_log: list = []
prev_tyre_session_time: float = 0.0
prev_tyre_compound: str = ""

pu_alert_conditions: dict = {}
pu_alerts_log: list = []
prev_pu_session_time: float = 0.0
prev_pu_ers_mode: str = ""
prev_pu_fuel_mix: str = ""

# DamageAgent state
damage_agent: DamageAgent | None = None
damage_agent_response: str | None = None
damage_agent_response_time: str | None = None
damage_agent_in_flight: bool = False
damage_agent_pending: list[str] = []
damage_agent_batch_handle: asyncio.TimerHandle | None = None
DAMAGE_AGENT_BATCH_DELAY: float = 1.0  # seconds to wait before flushing

# TyresAgent state
tyres_agent: TyresAgent | None = None
tyres_agent_response: str | None = None
tyres_agent_response_time: str | None = None
tyres_agent_in_flight: bool = False
tyres_agent_pending: list[str] = []
tyres_agent_batch_handle: asyncio.TimerHandle | None = None
TYRES_AGENT_BATCH_DELAY: float = 1.0  # seconds to wait before flushing

# PowerUnitAgent state
pu_agent: PowerUnitAgent | None = None
pu_agent_response: str | None = None
pu_agent_response_time: str | None = None
pu_agent_in_flight: bool = False
pu_agent_pending: list[str] = []
pu_agent_batch_handle: asyncio.TimerHandle | None = None
PU_AGENT_BATCH_DELAY: float = 1.0  # seconds to wait before flushing

# RaceEngineerAgent state
re_agent: RaceEngineerAgent | None = None
re_agent_in_flight: bool = False
re_agent_pending: list[str] = []
re_agent_batch_handle: asyncio.TimerHandle | None = None
re_alerts_log: list = []       # accumulated messages received by the race engineer
re_responses_log: list = []    # accumulated race engineer responses (split per target)
RE_AGENT_BATCH_DELAY: float = 2.0  # longer window to batch multiple engineer reports

connected_clients: set = set()

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
    """Format session time in seconds as MM:SS."""
    total = int(seconds)
    m = total // 60
    s = total % 60
    return f"{m:02d}:{s:02d}"


def _is_copy_ack(text: str) -> bool:
    """Return True if the text is a bare 'Copy' acknowledgment, ignoring markdown bold and trailing punctuation."""
    return text.strip().strip("*").strip().rstrip(".").strip().lower() == "copy"


def _reinit_agents(context: str) -> None:
    """Re-instantiate all agents with the given session context."""
    global damage_agent, tyres_agent, pu_agent, re_agent

    for name, cls, var_name in [
        ("DamageAgent", DamageAgent, "damage_agent"),
        ("TyresAgent", TyresAgent, "tyres_agent"),
        ("PowerUnitAgent", PowerUnitAgent, "pu_agent"),
        ("RaceEngineerAgent", RaceEngineerAgent, "re_agent"),
    ]:
        try:
            globals()[var_name] = cls(session_context=context)
            print(f"{name} re-initialized with session context")
        except Exception as exc:
            print(f"{name} re-init failed: {exc}")


def _process_aero_alerts(aero: dict) -> None:
    """Port of useAeroAlerts.ts — accumulates alerts into aero_alerts_log."""
    global aero_alert_conditions, aero_alerts_log, prev_aero_session_time
    global damage_agent_response, damage_agent_response_time, damage_agent_pending, damage_agent_batch_handle
    global re_agent_pending, re_agent_batch_handle
    global re_alerts_log, re_responses_log

    st = aero.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_aero_session_time:
        aero_alert_conditions = {}
        aero_alerts_log = []
        damage_agent_response = None
        damage_agent_response_time = None
        damage_agent_pending = []
        if damage_agent_batch_handle is not None:
            damage_agent_batch_handle.cancel()
            damage_agent_batch_handle = None
        re_agent_pending = []
        re_alerts_log = []
        re_responses_log = []
        if re_agent_batch_handle is not None:
            re_agent_batch_handle.cancel()
            re_agent_batch_handle = None
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
# Tyre alert constants & processing
# ---------------------------------------------------------------------------
TYRE_DAMAGE_REFIRE_STEP = 25  # re-alert every step when worsening (0-255 scale)

TYRE_WHEEL_LABELS = {"fl": "FL", "fr": "FR", "rl": "RL", "rr": "RR"}
TYRE_WHEELS = ("fl", "fr", "rl", "rr")
TYRE_DAMAGE_METRICS = {"dmg", "blst", "wear"}

TYRE_CLEAR_LABELS = {
    "temp": "temp back to normal",
    "wear": "tyre wear stabilised",
    "dmg": "tyre damage stabilised",
    "blst": "blistering subsided",
}


def _process_tyre_alerts(tyre_snapshot: dict) -> None:
    """Port of useTyreAlerts.ts — accumulates alerts into tyre_alerts_log."""
    global tyre_alert_conditions, tyre_alerts_log, prev_tyre_session_time, prev_tyre_compound
    global tyres_agent_response, tyres_agent_response_time, tyres_agent_pending, tyres_agent_batch_handle

    st = tyre_snapshot.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_tyre_session_time:
        tyre_alert_conditions = {}
        tyre_alerts_log = []
        prev_tyre_compound = ""
        tyres_agent_response = None
        tyres_agent_response_time = None
        tyres_agent_pending = []
        if tyres_agent_batch_handle is not None:
            tyres_agent_batch_handle.cancel()
            tyres_agent_batch_handle = None
    prev_tyre_session_time = st

    new_alerts: list[dict] = []
    current_conditions: dict = {}
    ts = _format_session_time(st)
    tyres = tyre_snapshot.get("tyres", {})

    for wn in TYRE_WHEELS:
        t = tyres.get(wn, {})
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
        prev = tyre_alert_conditions.get(key)
        underscore_idx = key.index("_")
        wn = key[:underscore_idx]
        metric = key[underscore_idx + 1:]
        label = TYRE_WHEEL_LABELS[wn]
        t = tyres.get(wn, {})
        life = round(100 - t.get("wear", 0))
        tag = metric.upper()

        is_new = prev is None
        is_escalation = prev is not None and prev["level"] == "warn" and cur["level"] == "crit"
        is_worsened = (
            metric in TYRE_DAMAGE_METRICS
            and prev is not None
            and prev["level"] == cur["level"]
            and cur["value"] >= prev["value"] + TYRE_DAMAGE_REFIRE_STEP
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
    for key in tyre_alert_conditions:
        if key not in current_conditions:
            underscore_idx = key.index("_")
            wn = key[:underscore_idx]
            metric = key[underscore_idx + 1:]
            label = TYRE_WHEEL_LABELS.get(wn, wn.upper())
            tag = metric.upper()
            clear_msg = TYRE_CLEAR_LABELS.get(metric)
            if clear_msg:
                new_alerts.append({"level": "info", "message": f"{label} {clear_msg}", "time": f"{ts} {tag}"})

    # Compound change
    compound = tyre_snapshot.get("compound", "")
    compound_visual = tyre_snapshot.get("compoundVisual", "")
    compound_key = f"{compound}_{compound_visual}"
    if compound and compound_key != prev_tyre_compound:
        prev_tyre_compound = compound_key
        new_alerts.append({
            "level": "info",
            "message": f"{compound_visual.upper()} ({compound}) fitted",
            "time": f"{ts} TYRE",
        })

    tyre_alert_conditions = current_conditions
    tyre_alerts_log.extend(new_alerts)
    return new_alerts


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
    global pu_agent_response, pu_agent_response_time, pu_agent_pending, pu_agent_batch_handle

    st = pu.get("sessionTime", 0.0)

    # Detect new session (session time resets)
    if st < prev_pu_session_time:
        pu_alert_conditions = {}
        pu_alerts_log = []
        prev_pu_ers_mode = ""
        prev_pu_fuel_mix = ""
        pu_agent_response = None
        pu_agent_response_time = None
        pu_agent_pending = []
        if pu_agent_batch_handle is not None:
            pu_agent_batch_handle.cancel()
            pu_agent_batch_handle = None
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
    return new_alerts


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
    global damage_agent_response, damage_agent_response_time, damage_agent_in_flight, damage_agent_pending
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
        # Skip bare "Copy" acknowledgments
        if response and not _is_copy_ack(response):
            damage_agent_response = response
            damage_agent_response_time = _format_session_time(prev_aero_session_time)
            _queue_race_engineer([f"From Damage Engineer: {response}"])
    except Exception as exc:
        print(f"DamageAgent error: {exc}")
    finally:
        damage_agent_in_flight = False

    # If more alerts arrived while we were busy, flush again
    if damage_agent_pending:
        await _flush_damage_agent()


def _queue_tyres_alerts(alert_texts: list[str]) -> None:
    """Add alert texts to the pending list and (re)start the batch timer."""
    global tyres_agent_batch_handle

    tyres_agent_pending.extend(alert_texts)

    if tyres_agent_batch_handle is not None:
        tyres_agent_batch_handle.cancel()

    loop = asyncio.get_running_loop()
    tyres_agent_batch_handle = loop.call_later(
        TYRES_AGENT_BATCH_DELAY,
        lambda: asyncio.ensure_future(_flush_tyres_agent()),
    )


async def _flush_tyres_agent() -> None:
    """Drain the pending list and send the batch to the TyresAgent."""
    global tyres_agent_response, tyres_agent_response_time, tyres_agent_in_flight, tyres_agent_pending
    global tyres_agent_batch_handle

    tyres_agent_batch_handle = None

    if tyres_agent is None or not tyres_agent_pending:
        return

    if tyres_agent_in_flight:
        return

    batch = tyres_agent_pending
    tyres_agent_pending = []

    tyres_agent_in_flight = True
    try:
        response = await asyncio.to_thread(tyres_agent.process_messages, batch)
        # Skip bare "Copy" acknowledgments
        if response and not _is_copy_ack(response):
            tyres_agent_response = response
            tyres_agent_response_time = _format_session_time(prev_tyre_session_time)
            _queue_race_engineer([f"From Tyres Engineer: {response}"])
    except Exception as exc:
        print(f"TyresAgent error: {exc}")
    finally:
        tyres_agent_in_flight = False

    if tyres_agent_pending:
        await _flush_tyres_agent()


def _queue_pu_alerts(alert_texts: list[str]) -> None:
    """Add alert texts to the pending list and (re)start the batch timer."""
    global pu_agent_batch_handle

    pu_agent_pending.extend(alert_texts)

    if pu_agent_batch_handle is not None:
        pu_agent_batch_handle.cancel()

    loop = asyncio.get_running_loop()
    pu_agent_batch_handle = loop.call_later(
        PU_AGENT_BATCH_DELAY,
        lambda: asyncio.ensure_future(_flush_pu_agent()),
    )


async def _flush_pu_agent() -> None:
    """Drain the pending list and send the batch to the PowerUnitAgent."""
    global pu_agent_response, pu_agent_response_time, pu_agent_in_flight, pu_agent_pending
    global pu_agent_batch_handle

    pu_agent_batch_handle = None

    if pu_agent is None or not pu_agent_pending:
        return

    if pu_agent_in_flight:
        return

    batch = pu_agent_pending
    pu_agent_pending = []

    pu_agent_in_flight = True
    try:
        response = await asyncio.to_thread(pu_agent.process_messages, batch)
        # Skip bare "Copy" acknowledgments
        if response and not _is_copy_ack(response):
            pu_agent_response = response
            pu_agent_response_time = _format_session_time(prev_pu_session_time)
            _queue_race_engineer([f"From Power Unit Engineer: {response}"])
    except Exception as exc:
        print(f"PowerUnitAgent error: {exc}")
    finally:
        pu_agent_in_flight = False

    if pu_agent_pending:
        await _flush_pu_agent()


def _queue_race_engineer(messages: list[str]) -> None:
    """Add engineer reports to the pending list and (re)start the batch timer."""
    global re_agent_batch_handle

    re_agent_pending.extend(messages)

    # Log incoming messages so they appear in the Race Engineer panel
    ts = _format_session_time(session_time)
    for msg in messages:
        re_alerts_log.append({"level": "info", "message": msg, "time": ts})

    if re_agent_batch_handle is not None:
        re_agent_batch_handle.cancel()

    loop = asyncio.get_running_loop()
    re_agent_batch_handle = loop.call_later(
        RE_AGENT_BATCH_DELAY,
        lambda: asyncio.ensure_future(_flush_race_engineer()),
    )


def _route_race_engineer_response(response: str) -> None:
    """Parse the race engineer's response and route follow-up questions to specialists.

    Handles multi-line messages per target: lines without a known prefix are
    appended to the most recently matched target. This way the race engineer
    can send multi-line messages to a single specialist or address multiple
    specialists in one response.
    """
    route_targets = {
        "To Damage Engineer:": (_queue_damage_alerts, aero_alerts_log),
        "To Tyres Engineer:": (_queue_tyres_alerts, tyre_alerts_log),
        "To Power Unit Engineer:": (_queue_pu_alerts, pu_alerts_log),
    }
    ts = _format_session_time(session_time)

    # Accumulate (target_key, lines) blocks so multi-line messages stay together
    current_target: str | None = None
    accumulated_lines: list[str] = []
    blocks: list[tuple[str, str]] = []  # (target_key, full_message)

    def _flush_block() -> None:
        if current_target and accumulated_lines:
            blocks.append((current_target, " ".join(accumulated_lines)))

    for line in response.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        matched = False
        for prefix in route_targets:
            if line.startswith(prefix):
                _flush_block()
                current_target = prefix
                accumulated_lines = []
                remainder = line[len(prefix):].strip()
                if remainder:
                    accumulated_lines.append(remainder)
                matched = True
                break
        if not matched and current_target:
            # Continuation line for the current target
            accumulated_lines.append(line)

    _flush_block()

    # Dispatch each block to the appropriate specialist
    for target_key, msg in blocks:
        queue_fn, alerts_log = route_targets[target_key]
        prefixed = f"From Race Engineer: {msg}"
        queue_fn([prefixed])
        alerts_log.append({"level": "info", "message": prefixed, "time": ts})


def _split_race_engineer_response(response: str) -> list[str]:
    """Split a race engineer response into individual messages per target.

    Each block starts with a known prefix (e.g. "To Fernando:", "To Damage Engineer:").
    Continuation lines (no prefix) are appended to the current block.
    Text without any prefix is kept as a standalone block.
    """
    prefixes = ("To Fernando:", "To Damage Engineer:", "To Tyres Engineer:", "To Power Unit Engineer:")
    blocks: list[str] = []
    current_lines: list[str] = []

    for line in response.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        if any(line.startswith(p) for p in prefixes):
            if current_lines:
                blocks.append(" ".join(current_lines))
            current_lines = [line]
        else:
            current_lines.append(line)

    if current_lines:
        blocks.append(" ".join(current_lines))

    return blocks


async def _flush_race_engineer() -> None:
    """Drain the pending list and send the batch to the RaceEngineerAgent."""
    global re_agent_in_flight, re_agent_pending
    global re_agent_batch_handle

    re_agent_batch_handle = None

    if re_agent is None or not re_agent_pending:
        return

    if re_agent_in_flight:
        return

    batch = re_agent_pending
    re_agent_pending = []

    re_agent_in_flight = True
    response = None
    try:
        response = await asyncio.to_thread(re_agent.process_messages, batch)
        # Don't send bare "Copy" acknowledgments to the frontend
        if response and not _is_copy_ack(response):
            ts = _format_session_time(session_time)
            for msg in _split_race_engineer_response(response):
                re_responses_log.append({"text": msg, "time": ts})
    except Exception as exc:
        print(f"RaceEngineerAgent error: {exc}")
    finally:
        re_agent_in_flight = False

    # Route follow-up questions back to specialist agents
    if response:
        _route_race_engineer_response(response)

    if re_agent_pending:
        await _flush_race_engineer()


# ---------------------------------------------------------------------------
# Build message & dispatch alerts
# ---------------------------------------------------------------------------
def _state_ready() -> bool:
    """Return True once the essential packets have been received at least once.

    Packets 6 (Car Telemetry), 7 (Car Status), and 10 (Car Damage) are
    required for meaningful alert processing.  Without them the adapter
    returns zeros which trigger spurious alerts (e.g. "0 (0) fitted",
    "Fuel critically low — 0.0 laps remaining") that immediately clear
    once real data arrives.
    """
    return 6 in state and 7 in state and 10 in state


def build_message() -> tuple[str, list[dict], list[dict], list[dict]]:
    """Build a JSON message from the latest merged state via the adapter."""
    aero = adapter.get_aero()
    tyres = adapter.get_tyres()
    compound, compound_visual = adapter.get_compound()
    power_unit = adapter.get_power_unit()
    session = adapter.get_session()
    lap = adapter.get_lap()
    track_map = adapter.get_track_map()

    # Skip alert processing until all essential packets have arrived
    if _state_ready():
        # -- Aero alerts --
        aero_snapshot = {**aero, "sessionTime": session_time}
        new_aero_alerts = _process_aero_alerts(aero_snapshot)

        # -- Tyre alerts --
        new_tyre_alerts = _process_tyre_alerts({
            "sessionTime": session_time,
            "tyres": tyres,
            "compound": compound,
            "compoundVisual": compound_visual,
        })

        # -- Power unit alerts --
        new_pu_alerts = _process_pu_alerts({**power_unit, "sessionTime": session_time})
    else:
        new_aero_alerts = []
        new_tyre_alerts = []
        new_pu_alerts = []

    msg = json.dumps({
        "tyres": tyres,
        "compound": compound,
        "compoundVisual": compound_visual,
        "tyresAgeLaps": adapter.get_tyres_age_laps(),
        "currentLap": lap.get("currentLap", 0),
        "speed": aero.get("speed", 0),
        "sessionTime": session_time,
        "powerUnit": power_unit,
        "aero": aero,
        "session": session,
        "trackMap": track_map,
        "tyreAlerts": {
            "alerts": tyre_alerts_log,
            "activeCount": len(tyre_alert_conditions),
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
            "time": damage_agent_response_time,
        },
        "tyresReport": {
            "response": tyres_agent_response,
            "time": tyres_agent_response_time,
        },
        "puReport": {
            "response": pu_agent_response,
            "time": pu_agent_response_time,
        },
        "raceEngineerReport": {
            "responses": re_responses_log,
            "alerts": re_alerts_log,
        },
    })
    return msg, new_aero_alerts, new_tyre_alerts, new_pu_alerts


# ---------------------------------------------------------------------------
# WebSocket server
# ---------------------------------------------------------------------------
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
        msg, _new_aero_alerts, _new_tyre_alerts, _new_pu_alerts = build_message()
        await websocket.send(msg)
        # Listen for incoming messages (driver radio)
        async for raw in websocket:
            try:
                incoming = json.loads(raw)
                driver_msg = incoming.get("driverMessage")
                if driver_msg and isinstance(driver_msg, str):
                    _queue_race_engineer([f"From Fernando: {driver_msg}"])
            except (json.JSONDecodeError, AttributeError):
                pass
    finally:
        connected_clients.discard(websocket)
        print(f"Client disconnected ({len(connected_clients)} total)")


# ---------------------------------------------------------------------------
# Alert dispatch helper
# ---------------------------------------------------------------------------
def _dispatch_alerts(new_aero_alerts: list[dict], new_tyre_alerts: list[dict], new_pu_alerts: list[dict]) -> None:
    """Dispatch new alerts to their respective specialist agents."""
    if new_aero_alerts:
        alert_texts = [
            f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
            for a in new_aero_alerts
        ]
        _queue_damage_alerts(alert_texts)

    if new_tyre_alerts:
        alert_texts = [
            f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
            for a in new_tyre_alerts
        ]
        _queue_tyres_alerts(alert_texts)

    if new_pu_alerts:
        alert_texts = [
            f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
            for a in new_pu_alerts
        ]
        _queue_pu_alerts(alert_texts)


# ---------------------------------------------------------------------------
# UDP live reader
# ---------------------------------------------------------------------------
async def udp_reader(capture: bool):
    """Read F1 25 UDP packets and update shared state, broadcasting on each
    telemetry frame (packet 6)."""
    global session_time, prev_session_uid

    listener = UdpListener(port=UDP_PORT)
    decoder = PacketDecoder()
    writer = None

    if capture:
        Path("data").mkdir(exist_ok=True)
        capture_path = Path("data") / f"f1_25_capture_{int(time.time())}.f1bin"
        writer = BinaryWriter(capture_path)
        writer.open()
        print(f"Binary capture enabled \u2192 {capture_path}")

    await listener.start()
    try:
        while True:
            datagram = await listener.queue.get()
            header = decoder.decode_header(datagram)
            packet_id = header["m_packetId"]
            state[packet_id] = decoder.decode(datagram)

            if writer:
                writer.write(datagram)

            session_time = adapter.session_time

            # Detect new session
            uid = adapter.session_uid
            if packet_id == 1 and uid != prev_session_uid:
                prev_session_uid = uid
                context = adapter.get_session_context()
                print(f"New session detected \u2014 re-initializing agents\n{context}")
                await asyncio.to_thread(_reinit_agents, context)
                # Rotate capture file on session change
                if writer:
                    writer.close()
                    new_path = Path("data") / f"f1_25_{uid}_{int(time.time())}.f1bin"
                    writer = BinaryWriter(new_path)
                    writer.open()
                    print(f"Capture rotated \u2192 {new_path}")

            # Trigger on Car Telemetry (packet 6)
            if packet_id == 6:
                msg, new_aero, new_tyre, new_pu = build_message()
                await broadcast(msg)
                _dispatch_alerts(new_aero, new_tyre, new_pu)
    finally:
        listener.stop()
        if writer:
            writer.close()


# ---------------------------------------------------------------------------
# .f1bin replay
# ---------------------------------------------------------------------------
async def f1bin_replay(filepath: str, speed: float):
    """Replay a captured .f1bin file as if it were live telemetry."""
    global state, adapter, session_time, prev_session_uid

    path = Path(filepath)
    if not path.exists():
        print(f"ERROR: file not found: {filepath}")
        return

    print(f"Replaying {filepath} ({speed}x) ...")

    session = ReplaySession(path, speed=speed)
    # Share the replay's state dict with our adapter
    state = session.state
    adapter = TelemetryStateAdapter(state)

    replay_task = asyncio.create_task(session.run())

    prev_count = 0
    try:
        while not replay_task.done():
            await asyncio.sleep(1 / 60)
            if session.packets_received == prev_count:
                continue
            prev_count = session.packets_received
            session_time = adapter.session_time

            # Detect new session
            uid = adapter.session_uid
            if uid and uid != prev_session_uid:
                prev_session_uid = uid
                context = adapter.get_session_context()
                print(f"New session detected in replay \u2014 re-initializing agents\n{context}")
                await asyncio.to_thread(_reinit_agents, context)

            msg, new_aero, new_tyre, new_pu = build_message()
            await broadcast(msg)
            _dispatch_alerts(new_aero, new_tyre, new_pu)
    finally:
        session.stop()
        await replay_task

    print(f"Replay complete \u2014 {session.packets_received} packets sent.")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------
async def main():
    """Parse CLI arguments and start the WebSocket server with UDP or replay."""
    global damage_agent, tyres_agent, pu_agent, re_agent

    parser = argparse.ArgumentParser(description="F1 25 telemetry WebSocket bridge")
    parser.add_argument(
        "--capture",
        action="store_true",
        help="Save telemetry to .f1bin files in the data/ folder (one file per session)",
    )
    parser.add_argument(
        "--replay",
        metavar="FILE",
        help="Replay a captured .f1bin file instead of listening for live UDP",
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

    os.environ.setdefault("AGENT_MANIFEST_FILE", "registries/manifest.hocon")
    try:
        damage_agent = DamageAgent()
        print("DamageAgent initialized")
    except Exception as exc:
        print(f"DamageAgent unavailable: {exc}")

    try:
        tyres_agent = TyresAgent()
        print("TyresAgent initialized")
    except Exception as exc:
        print(f"TyresAgent unavailable: {exc}")

    try:
        pu_agent = PowerUnitAgent()
        print("PowerUnitAgent initialized")
    except Exception as exc:
        print(f"PowerUnitAgent unavailable: {exc}")

    try:
        re_agent = RaceEngineerAgent()
        print("RaceEngineerAgent initialized")
    except Exception as exc:
        print(f"RaceEngineerAgent unavailable: {exc}")

    print(f"Starting WebSocket server on ws://{WS_HOST}:{WS_PORT}")
    async with websockets.serve(ws_handler, WS_HOST, WS_PORT):
        if args.replay:
            speed_str = args.speed.rstrip("x")
            try:
                speed = float(speed_str)
            except ValueError:
                parser.error(f"invalid --speed value: {args.speed} (expected Nx, e.g. 2x, 10x)")
            await f1bin_replay(args.replay, speed)
        else:
            await udp_reader(args.capture)


if __name__ == "__main__":
    asyncio.run(main())
