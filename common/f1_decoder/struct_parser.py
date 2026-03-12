"""
Generic struct parser that converts an f1_structs field dictionary
into a compiled struct.Struct and can unpack raw bytes into a Python dict.

Usage:
    from common.f1_decoder.struct_parser import StructParser
    from common.f1_structs.motion import CAR_MOTION_DATA

    parser = StructParser(CAR_MOTION_DATA)
    car = parser.unpack(raw_bytes, offset=0)
    # car == {"m_worldPositionX": 123.4, "m_worldPositionY": 0.0, ...}
"""

import struct


class StructParser:
    """Compiles an f1_structs field dict into a struct.Struct for fast unpacking."""

    def __init__(self, field_dict: dict[str, str]):
        """Build a compiled parser from a field dictionary.

        Args:
            field_dict: Ordered dict mapping field names to struct format chars.
                        E.g. {"m_speed": "H", "m_tyresPressure": "4f", "m_name": "32s"}
        """
        self._fields = field_dict
        self._field_map: list[tuple[str, int, bool]] = []  # (name, num_values, is_string)

        fmt_parts = []
        for name, fmt in field_dict.items():
            fmt_parts.append(fmt)
            num_values, is_string = self._parse_format(fmt)
            self._field_map.append((name, num_values, is_string))

        self._struct = struct.Struct("<" + "".join(fmt_parts))

    @property
    def size(self) -> int:
        """Size in bytes of one instance of this struct."""
        return self._struct.size

    @property
    def format(self) -> str:
        """The compiled struct format string (with '<' prefix)."""
        return self._struct.format

    def unpack(self, data: bytes, offset: int = 0) -> dict:
        """Unpack bytes at the given offset into a field dict.

        Args:
            data: Raw byte buffer (at least offset + self.size bytes).
            offset: Byte offset to start reading from.

        Returns:
            Dict mapping field names to decoded values.
            Array fields (e.g. "4f") become lists.
            String fields (e.g. "32s") become str (decoded UTF-8, null-trimmed).
        """
        values = self._struct.unpack_from(data, offset)
        result = {}
        idx = 0
        for name, num_values, is_string in self._field_map:
            if num_values == 1:
                val = values[idx]
                if is_string:
                    # Null-terminate and decode bytes to str
                    val = val.split(b"\x00")[0].decode("utf-8", errors="replace")
                result[name] = val
            else:
                result[name] = list(values[idx : idx + num_values])
            idx += num_values
        return result

    def unpack_array(self, data: bytes, offset: int, count: int) -> tuple[list[dict], int]:
        """Unpack a contiguous array of this struct.

        Args:
            data: Raw byte buffer.
            offset: Byte offset to start reading from.
            count: Number of struct instances to unpack.

        Returns:
            Tuple of (list of decoded dicts, new offset after all items).
        """
        items = []
        for _ in range(count):
            items.append(self.unpack(data, offset))
            offset += self._struct.size
        return items, offset

    @staticmethod
    def _parse_format(fmt: str) -> tuple[int, bool]:
        """Determine how many values a format char produces and whether it's a string.

        Args:
            fmt: A struct format fragment, e.g. "B", "4f", "32s", "H".

        Returns:
            (num_values, is_string): For "32s" -> (1, True), "4f" -> (4, False), "B" -> (1, False).
        """
        # String fields: "Ns" produces 1 bytes value
        if fmt.endswith("s"):
            return 1, True

        # Extract leading count digits
        count_str = ""
        for ch in fmt:
            if ch.isdigit():
                count_str += ch
            else:
                break

        return (int(count_str) if count_str else 1, False)
