"""Aero and damage alert processor — wing damage, brake temps, DRS faults."""

from telemetry_server.alerts.alert_constants import BRAKE_TEMPS
from telemetry_server.alerts.alert_constants import DAMAGE_PARTS
from telemetry_server.alerts.alert_constants import DAMAGE_REFIRE_STEP
from telemetry_server.alerts.alert_utils import format_session_time


class AeroAlertProcessor:
    """Evaluates aero telemetry snapshots and accumulates alerts.

    Tracks wing damage, brake temperatures, and DRS faults. Fires alerts
    when conditions appear, escalate, worsen beyond a refire threshold,
    or fully clear.
    """

    def __init__(self) -> None:
        """Initialise with empty state."""
        self.conditions: dict = {}
        self.alerts_log: list[dict] = []
        self.prev_session_time: float = 0.0

    def reset(self) -> None:
        """Clear all accumulated state (e.g. on session change)."""
        self.conditions.clear()
        self.alerts_log.clear()
        self.prev_session_time = 0.0

    def process(self, snapshot: dict) -> list[dict]:
        """Process an aero snapshot and return any new alerts."""
        st = snapshot.get("sessionTime", 0.0)

        # Detect new session (session time resets)
        if st < self.prev_session_time:
            self.reset()
        self.prev_session_time = st

        new_alerts: list[dict] = []
        current_conditions: dict = {}
        ts = format_session_time(st)

        # Damage conditions
        for part in DAMAGE_PARTS:
            val = snapshot.get(part["key"], 0)
            if val > 50:
                current_conditions[part["key"]] = {"level": "crit", "value": val}
            elif val > 20:
                current_conditions[part["key"]] = {"level": "warn", "value": val}

        # Brake temperatures
        for brk in BRAKE_TEMPS:
            val = snapshot.get(brk["key"], 0)
            if val > 1000:
                current_conditions[brk["key"]] = {"level": "crit", "value": val}
            elif val > 800:
                current_conditions[brk["key"]] = {"level": "warn", "value": val}

        # DRS fault
        if snapshot.get("drsFault"):
            current_conditions["drsFault"] = {"level": "crit", "value": 1}

        # Fire alerts for new, escalated, or worsened conditions
        for key, cur in current_conditions.items():
            prev = self.conditions.get(key)
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
        for key in self.conditions:
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
                            new_alerts.append({
                                "level": "info",
                                "message": f"{brk['label']} temp back to normal",
                                "time": f"{ts} {brk['tag']}",
                            })

        self.conditions = current_conditions
        self.alerts_log.extend(new_alerts)
        return new_alerts
