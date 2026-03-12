"""
Reads F1 25 binary capture files produced by BinaryWriter.

Yields (timestamp_ns, raw_datagram) tuples, which can then be decoded
with PacketDecoder or replayed at original/accelerated speed.

Usage:
    reader = BinaryReader(Path("data/session_001.f1bin"))
    for timestamp_ns, datagram in reader:
        packet = decoder.decode(datagram)
        ...
"""

import struct
from pathlib import Path
from typing import Iterator

# Frame header: uint64 timestamp_ns + uint16 payload_length = 10 bytes
_FRAME_HEADER = struct.Struct("<QH")
_FRAME_HEADER_SIZE = _FRAME_HEADER.size


class BinaryReader:
    """Reads binary capture files frame by frame."""

    def __init__(self, path: Path):
        """Initialize the reader.

        Args:
            path: Path to a .f1bin capture file.
        """
        self._path = path

    @property
    def path(self) -> Path:
        """The capture file path."""
        return self._path

    def __iter__(self) -> Iterator[tuple[int, bytes]]:
        """Iterate over all frames in the capture file.

        Yields:
            (timestamp_ns, datagram) tuples in capture order.
        """
        with open(self._path, "rb") as f:
            while True:
                header_bytes = f.read(_FRAME_HEADER_SIZE)
                if len(header_bytes) < _FRAME_HEADER_SIZE:
                    break  # EOF or truncated trailing frame
                timestamp_ns, payload_length = _FRAME_HEADER.unpack(header_bytes)
                datagram = f.read(payload_length)
                if len(datagram) < payload_length:
                    break  # Truncated file
                yield timestamp_ns, datagram

    def count(self) -> int:
        """Count total frames in the file (scans the full file)."""
        n = 0
        for _ in self:
            n += 1
        return n

    def packet_ids(self) -> dict[int, int]:
        """Count frames per packet ID (scans the full file).

        Returns:
            Dict mapping packet_id -> frame count.
        """
        counts: dict[int, int] = {}
        for _, datagram in self:
            if len(datagram) >= 7:
                # m_packetId is at byte offset 6 in the header (after uint16 + 4*uint8)
                packet_id = datagram[6]
                counts[packet_id] = counts.get(packet_id, 0) + 1
        return counts
