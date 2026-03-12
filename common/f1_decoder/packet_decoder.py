"""
Decodes raw F1 25 UDP datagrams into Python dicts using f1_structs definitions.

Usage:
    from common.f1_decoder.packet_decoder import PacketDecoder

    decoder = PacketDecoder()
    result = decoder.decode(raw_udp_bytes)
    # result["m_packetId"] == 0  (Motion)
    # result["m_carMotionData"][0]["m_worldPositionX"] == 123.4
"""

from common.f1_decoder.packet_layout import PACKET_LAYOUTS
from common.f1_decoder.packet_layout import Segment
from common.f1_decoder.struct_parser import StructParser
from common.f1_structs.header import PACKET_HEADER


class PacketDecoder:
    """Decodes any F1 25 UDP packet into a nested Python dict."""

    def __init__(self):
        """Pre-compile StructParsers for all known field dicts."""
        self._header_parser = StructParser(PACKET_HEADER)
        self._parser_cache: dict[int, StructParser] = {}

    def decode_header(self, data: bytes) -> dict:
        """Decode just the packet header (29 bytes).

        Useful for routing packets by m_packetId without decoding the full body.
        """
        return self._header_parser.unpack(data)

    def decode(self, data: bytes) -> dict:
        """Decode a complete UDP datagram into a Python dict.

        The header fields are merged into the top-level dict.
        Array segments (count > 1) are nested as lists under their segment name.
        Scalar segments with merge=True are flattened into the top-level dict.
        Scalar segments with merge=False are nested under their segment name.

        Args:
            data: Raw UDP datagram bytes.

        Returns:
            Decoded packet as a dict.

        Raises:
            KeyError: If the packet ID is not in PACKET_LAYOUTS.
        """
        # Peek at packet ID from the header
        header = self._header_parser.unpack(data)
        packet_id = header["m_packetId"]

        layout = PACKET_LAYOUTS[packet_id]
        result = {}
        offset = 0

        for segment in layout:
            parser = self._get_parser(segment)
            if segment.count == 1:
                decoded = parser.unpack(data, offset)
                offset += parser.size
                if segment.merge:
                    result.update(decoded)
                else:
                    result[segment.name] = decoded
            else:
                items, offset = parser.unpack_array(data, offset, segment.count)
                result[segment.name] = items

        return result

    def _get_parser(self, segment: Segment) -> StructParser:
        """Get or create a cached StructParser for a segment's field dict."""
        key = id(segment.fields)
        if key not in self._parser_cache:
            self._parser_cache[key] = StructParser(segment.fields)
        return self._parser_cache[key]
