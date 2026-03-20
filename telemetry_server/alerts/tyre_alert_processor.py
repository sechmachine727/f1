"""Tyre alert processor — surface temp, wear, damage, blistering, compound changes."""

from telemetry_server.alerts.alert_constants import TYRE_CLEAR_LABELS
from telemetry_server.alerts.alert_constants import TYRE_DAMAGE_METRICS
from telemetry_server.alerts.alert_constants import TYRE_DAMAGE_REFIRE_STEP
from telemetry_server.alerts.alert_constants import TYRE_WHEEL_LABELS
from telemetry_server.alerts.alert_constants import TYRE_WHEELS
from telemetry_server.alerts.alert_utils import format_session_time


class TyreAlertProcessor:
    """Evaluates tyre telemetry snapshots and accumulates alerts.

    Tracks per-wheel surface temperature, wear, damage, and blistering as
    well as compound changes.
    """

    def __init__(self) -> None:
        """Initialise with empty state."""
        self.conditions: dict = {}
        self.alerts_log: list[dict] = []
        self.prev_session_time: float = 0.0
        self.prev_compound: str = ""

    def reset(self) -> None:
        """Clear all accumulated state (e.g. on session change)."""
        self.conditions.clear()
        self.alerts_log.clear()
        self.prev_session_time = 0.0
        self.prev_compound = ""

    def process(self, snapshot: dict) -> list[dict]:
        """Process a tyre snapshot and return any new alerts."""
        st = snapshot.get("sessionTime", 0.0)

        # Detect new session (session time resets)
        if st < self.prev_session_time:
            self.reset()
        self.prev_session_time = st

        new_alerts: list[dict] = []
        current_conditions: dict = {}
        ts = format_session_time(st)
        tyres = snapshot.get("tyres", {})

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
            prev = self.conditions.get(key)
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
        for key in self.conditions:
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
        compound = snapshot.get("compound", "")
        compound_visual = snapshot.get("compoundVisual", "")
        compound_key = f"{compound}_{compound_visual}"
        if compound and compound_key != self.prev_compound:
            self.prev_compound = compound_key
            new_alerts.append({
                "level": "info",
                "message": f"{compound_visual.upper()} ({compound}) fitted",
                "time": f"{ts} TYRE",
            })

        self.conditions = current_conditions
        self.alerts_log.extend(new_alerts)
        return new_alerts
