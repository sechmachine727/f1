"""
Async UDP listener for F1 25 telemetry packets on port 20777.

Receives raw datagrams and pushes them to an asyncio.Queue for consumption
by CaptureSession or any other consumer.

Usage:
    listener = UdpListener(port=20777)
    await listener.start()
    while True:
        datagram = await listener.queue.get()
        ...
    listener.stop()
"""

import asyncio


class _UdpProtocol(asyncio.DatagramProtocol):
    """Internal protocol that pushes received datagrams to a queue."""

    def __init__(self, queue: asyncio.Queue):
        self._queue = queue

    def datagram_received(self, data: bytes, addr: tuple) -> None:
        """Called by asyncio when a UDP datagram arrives."""
        self._queue.put_nowait(data)

    def error_received(self, exc: Exception) -> None:
        """Called by asyncio on transport error."""
        print(f"[UdpListener] error: {exc}")


class UdpListener:
    """Listens for F1 25 UDP telemetry on the given port."""

    def __init__(self, port: int = 20777, queue_size: int = 4096):
        """Initialize the listener.

        Args:
            port: UDP port to bind to (F1 25 default is 20777).
            queue_size: Max queued datagrams before dropping.
        """
        self._port = port
        self.queue: asyncio.Queue[bytes] = asyncio.Queue(maxsize=queue_size)
        self._transport = None

    async def start(self) -> None:
        """Bind the UDP socket and start receiving."""
        loop = asyncio.get_running_loop()
        self._transport, _ = await loop.create_datagram_endpoint(
            lambda: _UdpProtocol(self.queue),
            local_addr=("0.0.0.0", self._port),
        )
        print(f"[UdpListener] listening on UDP port {self._port}")

    def stop(self) -> None:
        """Close the UDP socket."""
        if self._transport is not None:
            self._transport.close()
            self._transport = None
            print("[UdpListener] stopped")
