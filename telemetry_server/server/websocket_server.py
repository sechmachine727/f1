"""WebSocket server — client tracking, broadcast, and driver radio handler."""

from __future__ import annotations

import asyncio
import json
from typing import TYPE_CHECKING

import websockets

if TYPE_CHECKING:
    from telemetry_server.dispatch.agent_dispatch_manager import AgentDispatchManager
    from telemetry_server.server.message_builder import MessageBuilder


class WebSocketServer:
    """Manages WebSocket connections, broadcasting, and incoming driver messages.

    Each connected client receives the full telemetry JSON on every frame.
    Driver radio messages are forwarded to the race engineer agent.
    """

    def __init__(
        self,
        host: str,
        port: int,
        message_builder: MessageBuilder,
        dispatch_manager: AgentDispatchManager,
    ) -> None:
        """Initialise the WebSocket server.

        Args:
            host: Bind address (e.g. ``"0.0.0.0"``).
            port: Bind port (e.g. ``8765``).
            message_builder: Used to build the initial state message for new clients.
            dispatch_manager: Used to queue driver radio input to the race engineer.
        """
        self.host = host
        self.port = port
        self._message_builder = message_builder
        self._dispatch = dispatch_manager
        self.connected_clients: set = set()

    async def broadcast(self, message: str) -> None:
        """Send a message to every connected WebSocket client."""
        if not self.connected_clients:
            return
        await asyncio.gather(
            *(client.send(message) for client in self.connected_clients),
            return_exceptions=True,
        )

    async def handler(self, websocket: websockets.WebSocketServerProtocol) -> None:
        """Handle a new WebSocket connection."""
        self.connected_clients.add(websocket)
        print(f"Client connected ({len(self.connected_clients)} total)")
        try:
            # Send current state immediately so the UI isn't blank
            msg, _, _, _ = self._message_builder.build()
            await websocket.send(msg)
            # Listen for incoming messages (driver radio)
            async for raw in websocket:
                try:
                    incoming = json.loads(raw)
                    driver_msg = incoming.get("driverMessage")
                    if driver_msg and isinstance(driver_msg, str):
                        self._dispatch.race_engineer.queue([f"From Fernando: {driver_msg}"])
                except (json.JSONDecodeError, AttributeError):
                    pass
        finally:
            self.connected_clients.discard(websocket)
            print(f"Client disconnected ({len(self.connected_clients)} total)")

    def serve(self):
        """Return a ``websockets.serve`` context manager bound to this server."""
        return websockets.serve(self.handler, self.host, self.port)
