"""A flashback rewinds sessionTime but keeps the session.

The game's flashback moves m_sessionTime backwards while m_sessionUID stays the
same, so a session clock that goes backwards does not identify a new session.
Alert processors keyed on the clock wiped their logs and re-fired every active
condition on each flashback. Session identity must come from m_sessionUID.
"""

import json

from f1_packet_builder import build_packet

from common.f1_decoder.packet_decoder import PacketDecoder
from telemetry_server.alerts.aero_alert_processor import AeroAlertProcessor
from telemetry_server.alerts.pu_alert_processor import PuAlertProcessor
from telemetry_server.alerts.tyre_alert_processor import TyreAlertProcessor
from telemetry_server.telemetry_server import TelemetryServer


def _aero(uid: int, session_time: float) -> dict:
    """An aero snapshot with a hot front-left brake."""
    return {"sessionUid": uid, "sessionTime": session_time, "brakeTempFL": 850}


def _tyres(uid: int, session_time: float) -> dict:
    """A tyre snapshot with a hot front-left surface."""
    return {
        "sessionUid": uid,
        "sessionTime": session_time,
        "tyres": {"fl": {"surfaceTemp": 105}},
        "compound": "soft",
        "compoundVisual": "soft",
    }


def _pu(uid: int, session_time: float) -> dict:
    """A power unit snapshot with a hot engine."""
    return {"sessionUid": uid, "sessionTime": session_time, "engineTemp": 125}


def test_flashback_keeps_aero_alerts():
    """Rewinding the clock inside one session must not clear the alert log."""
    processor = AeroAlertProcessor()
    processor.process(_aero(1, 100.0))
    before = list(processor.alerts_log)

    processor.process(_aero(1, 60.0))

    assert processor.alerts_log == before, "flashback wiped the alert log"


def test_flashback_does_not_refire_aero_alerts():
    """A condition that was already alerted stays quiet across the rewind."""
    processor = AeroAlertProcessor()
    processor.process(_aero(1, 100.0))

    new_alerts = processor.process(_aero(1, 60.0))

    assert new_alerts == []


def test_new_session_clears_aero_alerts():
    """A different session UID is a real session change and clears state."""
    processor = AeroAlertProcessor()
    processor.process(_aero(1, 100.0))

    processor.process(_aero(2, 1.0))

    assert [a["time"] for a in processor.alerts_log] == ["00:01 BRK FL"]


def test_flashback_keeps_tyre_alerts():
    """Tyre alerts survive a rewind."""
    processor = TyreAlertProcessor()
    processor.process(_tyres(1, 100.0))
    before = list(processor.alerts_log)

    assert processor.process(_tyres(1, 60.0)) == []
    assert processor.alerts_log == before


def test_flashback_keeps_pu_alerts():
    """Power unit alerts survive a rewind."""
    processor = PuAlertProcessor()
    processor.process(_pu(1, 100.0))
    before = list(processor.alerts_log)

    assert processor.process(_pu(1, 60.0)) == []
    assert processor.alerts_log == before


def test_payload_exposes_session_uid():
    """The dashboard needs the session UID to tell a rewind from a new session."""
    msg, _, _, _ = TelemetryServer().message_builder.build()

    assert '"sessionUid": 0' in msg


def _feed(server: TelemetryServer, uid: int, session_time: float) -> None:
    """Store one full set of ready packets with a hot front-left brake."""
    decoder = PacketDecoder()
    header = {"m_packetFormat": 2025, "m_sessionUID": uid, "m_sessionTime": session_time,
              "m_playerCarIndex": 0}
    for packet_id in (6, 7, 10):
        values = dict(header, m_packetId=packet_id)
        if packet_id == 6:
            values["m_carTelemetryData[0].m_brakesTemperature"] = [900, 500, 500, 500]
        server.store_packet(packet_id, decoder.decode(build_packet(2025, packet_id, **values)))


def test_message_builder_keeps_alerts_through_flashback():
    """The full payload path keeps the alert log across a rewind."""
    server = TelemetryServer()
    _feed(server, uid=1, session_time=120.0)
    before, new_before, _, _ = server.message_builder.build()
    assert new_before, "the hot brake should raise an alert"

    _feed(server, uid=1, session_time=60.0)
    after, new_after, _, _ = server.message_builder.build()

    assert new_after == []
    assert json.loads(after)["aeroAlerts"] == json.loads(before)["aeroAlerts"]
