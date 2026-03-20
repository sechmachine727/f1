"""Dispatcher for the RaceEngineerAgent — routes responses back to specialists."""

from __future__ import annotations

import asyncio
from typing import Any

from telemetry_server.alerts.alert_utils import format_session_time
from telemetry_server.alerts.alert_utils import is_copy_ack
from telemetry_server.dispatch.base_agent_dispatcher import BaseAgentDispatcher


class RaceEngineerDispatcher(BaseAgentDispatcher):
    """Batched dispatcher for the RaceEngineerAgent.

    In addition to the standard queue/flush behaviour, this dispatcher
    maintains its own alerts and responses logs and routes follow-up
    questions from the race engineer back to specialist dispatchers.
    """

    BATCH_DELAY: float = 2.0  # longer window to batch multiple engineer reports

    def __init__(self, agent: Any, session_time_fn: Any) -> None:
        """Initialise the race engineer dispatcher.

        Args:
            agent: The RaceEngineerAgent instance.
            session_time_fn: Callable returning current session time.
        """
        super().__init__(agent=agent, batch_delay=self.BATCH_DELAY, name="RaceEngineerAgent",
                         session_time_fn=session_time_fn)
        self.alerts_log: list[dict] = []
        self.responses_log: list[dict] = []
        # Set after construction via set_specialist_dispatchers()
        self._specialist_dispatchers: dict[str, tuple[BaseAgentDispatcher, list[dict]]] = {}

    def set_specialist_dispatchers(
        self,
        damage: BaseAgentDispatcher,
        damage_alerts_log: list[dict],
        tyres: BaseAgentDispatcher,
        tyre_alerts_log: list[dict],
        pu: BaseAgentDispatcher,
        pu_alerts_log: list[dict],
    ) -> None:
        """Wire up specialist dispatchers for routing follow-up responses.

        Args:
            damage: The damage dispatcher.
            damage_alerts_log: Reference to the aero alerts log.
            tyres: The tyres dispatcher.
            tyre_alerts_log: Reference to the tyre alerts log.
            pu: The PU dispatcher.
            pu_alerts_log: Reference to the PU alerts log.
        """
        self._specialist_dispatchers = {
            "To Damage Engineer:": (damage, damage_alerts_log),
            "To Tyres Engineer:": (tyres, tyre_alerts_log),
            "To Power Unit Engineer:": (pu, pu_alerts_log),
        }

    def queue(self, messages: list[str]) -> None:
        """Add messages to the pending list, log them, and (re)start the batch timer."""
        self.pending.extend(messages)

        ts = format_session_time(self._session_time_fn())
        for msg in messages:
            self.alerts_log.append({"level": "info", "message": msg, "time": ts})

        if self.batch_handle is not None:
            self.batch_handle.cancel()

        loop = asyncio.get_running_loop()
        self.batch_handle = loop.call_later(
            self.batch_delay,
            lambda: asyncio.ensure_future(self.flush()),
        )

    async def flush(self) -> None:
        """Drain the pending list and send the batch to the RaceEngineerAgent."""
        self.batch_handle = None

        if self.agent is None or not self.pending:
            return

        if self.in_flight:
            return

        batch = self.pending
        self.pending = []

        self.in_flight = True
        response = None
        try:
            response = await asyncio.to_thread(self.agent.process_messages, batch)
            if response and not is_copy_ack(response):
                ts = format_session_time(self._session_time_fn())
                for msg in self._split_response(response):
                    self.responses_log.append({"text": msg, "time": ts})
        except Exception as exc:
            print(f"RaceEngineerAgent error: {exc}")
        finally:
            self.in_flight = False

        # Route follow-up questions back to specialist agents
        if response:
            self._route_response(response)

        if self.pending:
            await self.flush()

    def _route_response(self, response: str) -> None:
        """Parse the race engineer's response and route follow-up questions to specialists.

        Handles multi-line messages per target: lines without a known prefix are
        appended to the most recently matched target.
        """
        ts = format_session_time(self._session_time_fn())

        current_target: str | None = None
        accumulated_lines: list[str] = []
        blocks: list[tuple[str, str]] = []

        def _flush_block() -> None:
            if current_target and accumulated_lines:
                blocks.append((current_target, " ".join(accumulated_lines)))

        for line in response.strip().splitlines():
            line = line.strip()
            if not line:
                continue
            matched = False
            for prefix in self._specialist_dispatchers:
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
                accumulated_lines.append(line)

        _flush_block()

        for target_key, msg in blocks:
            dispatcher, alerts_log = self._specialist_dispatchers[target_key]
            prefixed = f"From Race Engineer: {msg}"
            dispatcher.queue([prefixed])
            alerts_log.append({"level": "info", "message": prefixed, "time": ts})

    @staticmethod
    def _split_response(response: str) -> list[str]:
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

    def reset(self) -> None:
        """Clear all dispatcher state including logs."""
        super().reset()
        self.alerts_log.clear()
        self.responses_log.clear()
