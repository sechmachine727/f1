"""Dispatcher for the TyresAgent — forwards responses to the race engineer."""

from __future__ import annotations

from typing import TYPE_CHECKING
from typing import Any

from telemetry_server.dispatch.base_agent_dispatcher import BaseAgentDispatcher

if TYPE_CHECKING:
    from telemetry_server.dispatch.race_engineer_dispatcher import RaceEngineerDispatcher


class TyresDispatcher(BaseAgentDispatcher):
    """Batched dispatcher for the TyresAgent.

    On receiving a non-copy-ack response, forwards it to the race
    engineer dispatcher.
    """

    def __init__(self, agent: Any, session_time_fn: Any, race_engineer: RaceEngineerDispatcher) -> None:
        """Initialise the tyres dispatcher.

        Args:
            agent: The TyresAgent instance.
            session_time_fn: Callable returning current session time.
            race_engineer: The race engineer dispatcher to forward responses to.
        """
        super().__init__(agent=agent, batch_delay=1.0, name="TyresAgent", session_time_fn=session_time_fn)
        self._race_engineer = race_engineer

    def _on_response(self, response: str) -> None:
        """Store response and forward to the race engineer."""
        super()._on_response(response)
        self._race_engineer.queue([f"From Tyres Engineer: {response}"])
