"""Facade that owns all agent dispatchers and coordinates their lifecycle."""

from __future__ import annotations

from typing import Callable

from telemetry_server.dispatch.damage_dispatcher import DamageDispatcher
from telemetry_server.dispatch.pu_dispatcher import PuDispatcher
from telemetry_server.dispatch.race_engineer_dispatcher import RaceEngineerDispatcher
from telemetry_server.dispatch.tyres_dispatcher import TyresDispatcher


class AgentDispatchManager:
    """Owns all four dispatchers that queue alerts and hold the UI report state.

    No agent is wired in yet, so the dispatchers stay queue-only until a
    harness replaces them.
    """

    def __init__(self, session_time_fn: Callable[[], float], alert_logs: dict[str, list[dict]]) -> None:
        """Initialise the dispatch manager.

        Args:
            session_time_fn: Callable returning the current session time.
            alert_logs: Dict with keys ``"aero"``, ``"tyre"``, ``"pu"`` mapping
                to the alert log lists owned by the respective processors.
        """
        self._session_time_fn = session_time_fn

        # Create race engineer first (no deps on specialists)
        self.race_engineer = RaceEngineerDispatcher(agent=None, session_time_fn=session_time_fn)

        # Create specialist dispatchers, each referencing the race engineer
        self.damage = DamageDispatcher(agent=None, session_time_fn=session_time_fn, race_engineer=self.race_engineer)
        self.tyres = TyresDispatcher(agent=None, session_time_fn=session_time_fn, race_engineer=self.race_engineer)
        self.pu = PuDispatcher(agent=None, session_time_fn=session_time_fn, race_engineer=self.race_engineer)

        # Wire race engineer back to specialists for follow-up routing
        self.race_engineer.set_specialist_dispatchers(
            damage=self.damage, damage_alerts_log=alert_logs["aero"],
            tyres=self.tyres, tyre_alerts_log=alert_logs["tyre"],
            pu=self.pu, pu_alerts_log=alert_logs["pu"],
        )

    def reset_all(self) -> None:
        """Reset all dispatchers (e.g. on session change)."""
        self.damage.reset()
        self.tyres.reset()
        self.pu.reset()
        self.race_engineer.reset()

    def dispatch_alerts(
        self,
        new_aero_alerts: list[dict],
        new_tyre_alerts: list[dict],
        new_pu_alerts: list[dict],
    ) -> None:
        """Format new alerts as text and queue them to the appropriate dispatchers."""
        if new_aero_alerts:
            self.damage.queue([
                f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
                for a in new_aero_alerts
            ])

        if new_tyre_alerts:
            self.tyres.queue([
                f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
                for a in new_tyre_alerts
            ])

        if new_pu_alerts:
            self.pu.queue([
                f"ALERT {a['level'].upper()} {a['message']} {a['time']}"
                for a in new_pu_alerts
            ])

