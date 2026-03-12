"""
Orchestrates live telemetry capture: receives UDP packets, decodes them,
maintains latest state for live viewers, and writes to .f1bin at the configured Hz.

High-frequency packets (Motion, LapData, Telemetry, Status, MotionEx) are
gated by the capture Hz setting. All other packets (Session, Events, Participants,
Damage, etc.) are always captured since they carry infrequent state changes.

Usage:
    session = CaptureSession(hz=20, output_dir=Path("data"))
    await session.run()   # blocks until stopped
"""

import asyncio
import time
from pathlib import Path

from common.f1_capture.binary_writer import BinaryWriter
from common.f1_capture.udp_listener import UdpListener
from common.f1_decoder.packet_decoder import PacketDecoder


# Packets sent every frame (up to 60Hz) — gate these by capture Hz
HIGH_FREQUENCY_PACKET_IDS = {0, 2, 6, 7, 13}  # Motion, LapData, Telemetry, Status, MotionEx

# All other packet IDs are always captured (Session, Event, Participants, Setups,
# FinalClassification, LobbyInfo, Damage, SessionHistory, TyreSets, TimeTrial, LapPositions)


class CaptureSession:
    """Manages a live capture session with frequency-gated binary writing."""

    def __init__(
        self,
        hz: int = 10,
        port: int = 20777,
        output_dir: Path = Path("data"),
        capture: bool = True,
    ):
        """Initialize the capture session.

        Args:
            hz: Capture frequency for high-frequency packets (e.g., 10, 20, 60).
            port: UDP port to listen on.
            output_dir: Directory for .f1bin output files.
            capture: Whether to write .f1bin files. False for view-only mode.
        """
        self.hz = hz
        self._capture = capture
        self._output_dir = output_dir
        self._listener = UdpListener(port=port)
        self._decoder = PacketDecoder()
        self._writer: BinaryWriter | None = None
        self._running = False

        # Frequency gate: minimum interval between high-frequency captures (per packet type)
        self._min_interval_ns = int(1e9 / hz) if hz > 0 else 0
        self._last_capture_ns: dict[int, int] = {}

        # Latest decoded state per packet type (always updated, regardless of Hz)
        self.state: dict[int, dict] = {}

        # Session tracking
        self._current_session_uid: int | None = None

        # Stats
        self.packets_received: int = 0
        self.packets_captured: int = 0

    async def run(self) -> None:
        """Start listening and processing packets. Blocks until stop() is called."""
        await self._listener.start()
        self._running = True

        try:
            while self._running:
                try:
                    datagram = await asyncio.wait_for(
                        self._listener.queue.get(), timeout=0.5
                    )
                except asyncio.TimeoutError:
                    continue
                self._process(datagram)
        finally:
            self._listener.stop()
            if self._writer is not None:
                self._writer.close()
                print(f"[CaptureSession] closed capture file: {self._writer.path}")

    def stop(self) -> None:
        """Signal the session to stop."""
        self._running = False

    def _process(self, datagram: bytes) -> None:
        """Process one raw UDP datagram."""
        if len(datagram) < 29:
            return  # Too short to contain a header

        self.packets_received += 1

        # Decode header to get packet_id and session_uid
        header = self._decoder.decode_header(datagram)
        packet_id = header["m_packetId"]
        session_uid = header["m_sessionUID"]

        # Detect session change — open a new capture file
        if session_uid != self._current_session_uid:
            self._on_session_change(session_uid)

        # Always decode and update latest state (for live viewers)
        try:
            decoded = self._decoder.decode(datagram)
            self.state[packet_id] = decoded
        except (KeyError, Exception):
            pass  # Unknown packet type or decode error — skip

        # Decide whether to write this packet to the capture file
        if not self._capture or self._writer is None:
            return

        now_ns = time.time_ns()

        if packet_id in HIGH_FREQUENCY_PACKET_IDS:
            # Gate high-frequency packets by the capture Hz (per packet type)
            last_ns = self._last_capture_ns.get(packet_id, 0)
            elapsed = now_ns - last_ns
            if elapsed < self._min_interval_ns:
                return
            self._last_capture_ns[packet_id] = now_ns

        self._writer.write(datagram, timestamp_ns=now_ns)
        self.packets_captured += 1

    def _on_session_change(self, session_uid: int) -> None:
        """Handle a new session UID — close old file, open new one."""
        if self._writer is not None:
            self._writer.close()
            print(f"[CaptureSession] closed: {self._writer.path}")

        self._current_session_uid = session_uid
        self.state.clear()

        if self._capture:
            ts = time.strftime("%Y%m%d_%H%M%S")
            path = self._output_dir / f"f1_25_{session_uid:016x}_{ts}.f1bin"
            self._writer = BinaryWriter(path)
            self._writer.open()
            print(f"[CaptureSession] capturing to: {path}")
