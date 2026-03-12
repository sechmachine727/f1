"""
Writes raw F1 25 UDP datagrams to a binary capture file.

File format (per frame):
    [uint64  timestamp_ns]   — wall-clock time in nanoseconds (time.time_ns)
    [uint16  payload_length] — length of the UDP datagram in bytes
    [bytes   payload]        — the raw UDP datagram, unchanged

This is the fastest, most lossless capture format — no decoding at write time.
Decode later using PacketDecoder + BinaryReader.

Usage:
    writer = BinaryWriter(Path("data/session_001.f1bin"))
    writer.open()
    writer.write(raw_udp_bytes)   # call per datagram
    writer.close()
"""

import struct
import time
from pathlib import Path

# Frame header: uint64 timestamp_ns + uint16 payload_length = 10 bytes
_FRAME_HEADER = struct.Struct("<QH")


class BinaryWriter:
    """Captures raw UDP datagrams to a binary file with timestamps."""

    def __init__(self, path: Path):
        """Initialize the writer.

        Args:
            path: Output file path. Parent directories are created automatically.
        """
        self._path = path
        self._file = None

    @property
    def path(self) -> Path:
        """The output file path."""
        return self._path

    def open(self) -> None:
        """Open the file for writing. Creates parent directories if needed."""
        self._path.parent.mkdir(parents=True, exist_ok=True)
        self._file = open(self._path, "wb")

    def write(self, datagram: bytes, timestamp_ns: int | None = None) -> None:
        """Write one UDP datagram with a timestamp.

        Args:
            datagram: Raw UDP payload bytes.
            timestamp_ns: Wall-clock time in nanoseconds. Defaults to time.time_ns().
        """
        if self._file is None:
            raise RuntimeError("BinaryWriter is not open — call open() first")
        if timestamp_ns is None:
            timestamp_ns = time.time_ns()
        self._file.write(_FRAME_HEADER.pack(timestamp_ns, len(datagram)))
        self._file.write(datagram)

    def flush(self) -> None:
        """Flush buffered data to disk."""
        if self._file is not None:
            self._file.flush()

    def close(self) -> None:
        """Close the file."""
        if self._file is not None:
            self._file.close()
            self._file = None

    def __enter__(self):
        """Context manager support."""
        self.open()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager support."""
        self.close()
