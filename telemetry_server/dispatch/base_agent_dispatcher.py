"""Base class for agent dispatchers with debounced batching."""

from __future__ import annotations

import asyncio
from typing import Any

from telemetry_server.alerts.alert_utils import format_session_time
from telemetry_server.alerts.alert_utils import is_copy_ack


class BaseAgentDispatcher:
    """Debounced batch dispatcher for a specialist agent.

    Queued alert texts are accumulated for ``batch_delay`` seconds before
    being flushed to the underlying agent in a background thread.  An
    in-flight guard prevents concurrent agent calls; alerts that arrive
    while the agent is busy are re-flushed when the current call completes.
    """

    def __init__(self, agent: Any, batch_delay: float, name: str, session_time_fn: Any) -> None:
        """Initialise the dispatcher.

        Args:
            agent: The specialist agent instance (must have ``process_messages``).
            batch_delay: Seconds to wait before flushing accumulated alerts.
            name: Human-readable name for logging.
            session_time_fn: Callable returning the current session time in seconds.
        """
        self.agent: Any = agent
        self.batch_delay: float = batch_delay
        self.name: str = name
        self._session_time_fn = session_time_fn

        self.response: str | None = None
        self.response_time: str | None = None
        self.in_flight: bool = False
        self.pending: list[str] = []
        self.batch_handle: asyncio.TimerHandle | None = None

    def queue(self, alert_texts: list[str]) -> None:
        """Add alert texts to the pending list and (re)start the batch timer."""
        self.pending.extend(alert_texts)

        if self.batch_handle is not None:
            self.batch_handle.cancel()

        loop = asyncio.get_running_loop()
        self.batch_handle = loop.call_later(
            self.batch_delay,
            lambda: asyncio.ensure_future(self.flush()),
        )

    async def flush(self) -> None:
        """Drain the pending list and send the batch to the agent."""
        self.batch_handle = None

        if self.agent is None or not self.pending:
            return

        if self.in_flight:
            return

        batch = self.pending
        self.pending = []

        self.in_flight = True
        try:
            response = await asyncio.to_thread(self.agent.process_messages, batch)
            if response and not is_copy_ack(response):
                self._on_response(response)
        except Exception as exc:
            print(f"{self.name} error: {exc}")
        finally:
            self.in_flight = False

        # If more alerts arrived while we were busy, flush again
        if self.pending:
            await self.flush()

    def _on_response(self, response: str) -> None:
        """Handle a non-copy-ack response from the agent.

        Stores the response and timestamp. Subclasses override to add
        forwarding behaviour (e.g. to the race engineer).
        """
        self.response = response
        self.response_time = format_session_time(self._session_time_fn())

    def reset(self) -> None:
        """Clear all dispatcher state (e.g. on session change)."""
        self.response = None
        self.response_time = None
        self.pending = []
        if self.batch_handle is not None:
            self.batch_handle.cancel()
            self.batch_handle = None

    def reinit(self, agent: Any) -> None:
        """Replace the agent instance (e.g. on session change)."""
        self.agent = agent
