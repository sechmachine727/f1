"""
Replays a previously captured .f1bin file, feeding packets into the same
state dict that TerminalViewer reads from.

Supports adjustable playback speed (1x = real-time, 10x = 10x faster, etc.).

Usage:
    session = ReplaySession(Path("data/session.f1bin"), speed=5.0)
    await session.run()   # blocks until file ends or stop() is called
"""

import asyncio
from pathlib import Path

from common.f1_capture.binary_reader import BinaryReader
from common.f1_decoder.packet_decoder import PacketDecoder


class ReplaySession:
    """Replays a .f1bin capture file with the same interface as CaptureSession."""

    def __init__(self, path: Path, speed: float = 1.0, max_wait: float = 0.0):
        """Initialize the replay session.

        Args:
            path: Path to the .f1bin capture file.
            speed: Playback speed multiplier (1.0 = real-time, 10.0 = 10x faster).
            max_wait: Maximum seconds to sleep between packets.
                      0 = no waiting (replay as fast as possible).
                      Positive = cap inter-packet delay to this value.
                      Negative = unlimited (respect original timing exactly).
        """
        self._reader = BinaryReader(path)
        self._decoder = PacketDecoder()
        self._speed = speed
        self._max_wait = max_wait
        self._running = False

        # Same public interface as CaptureSession
        self.state: dict[int, dict] = {}
        self.packets_received: int = 0
        self.packets_captured: int = 0
        self.hz: str = f"{speed}x"

        # Replay stats
        self.path = path
        self.speed = speed
        self.progress: float = 0.0  # 0.0 to 1.0

    async def run(self) -> None:
        """Replay the capture file respecting original timing. Blocks until done."""
        self._running = True

        # Pre-scan to get total frame count for progress reporting
        total_frames = self._reader.count()
        self.total_frames = total_frames

        first_ts: int | None = None
        replay_start: float | None = None

        for frame_idx, (timestamp_ns, datagram) in enumerate(self._reader):
            if not self._running:
                break

            # Timing: wait to match the original packet spacing (scaled by speed)
            if first_ts is None:
                first_ts = timestamp_ns
                replay_start = asyncio.get_event_loop().time()
            else:
                # How far into the original recording is this packet?
                original_elapsed_s = (timestamp_ns - first_ts) / 1e9
                # How far into replay time should we be?
                target_replay_s = original_elapsed_s / self._speed
                # How far into replay we actually are
                actual_elapsed_s = asyncio.get_event_loop().time() - replay_start
                wait = target_replay_s - actual_elapsed_s
                if wait > 0 and self._max_wait != 0:
                    if self._max_wait > 0:
                        wait = min(wait, self._max_wait)
                    await asyncio.sleep(wait)

            if not self._running:
                break

            self.packets_received = frame_idx + 1
            self.progress = (frame_idx + 1) / total_frames if total_frames else 0

            # Decode and update state
            if len(datagram) < 29:
                continue
            try:
                header = self._decoder.decode_header(datagram)
                packet_id = header["m_packetId"]
                decoded = self._decoder.decode(datagram)
                self.state[packet_id] = decoded
                self.packets_captured += 1
            except (KeyError, Exception):
                pass

        # Replay finished naturally
        self._running = False

    def stop(self) -> None:
        """Signal the replay to stop."""
        self._running = False
