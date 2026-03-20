"""Pit status alert processor — detects pit lane and pit box transitions."""

from telemetry_server.alerts.alert_utils import format_session_time


class PitAlertProcessor:
    """Detects pit lane and pit box transitions and generates alerts for the race engineer.

    Tracks ``pitStatus`` transitions (0=none, 1=pitting, 2=in pit area)
    and fires an alert when the car enters/leaves the pit lane or pit box.
    """

    def __init__(self) -> None:
        """Initialise with no previous pit status."""
        self._prev_pit_status: int = 0
        self.prev_session_time: float = 0.0

    def reset(self) -> None:
        """Clear state (e.g. on session change)."""
        self._prev_pit_status = 0
        self.prev_session_time = 0.0

    def process(self, pit_data: dict, session_time: float) -> list[dict]:
        """Check for pit status transitions and return any new alerts.

        Args:
            pit_data: Dict from ``TelemetryStateAdapter.get_pit_status()``.
            session_time: Current session time in seconds.

        Returns:
            List of alert dicts (may be empty).
        """
        if session_time < self.prev_session_time:
            self.reset()
        self.prev_session_time = session_time

        status = pit_data.get("pitStatus", 0)
        prev = self._prev_pit_status
        self._prev_pit_status = status

        if status == prev:
            return []

        ts = format_session_time(session_time)
        alerts: list[dict] = []
        num_stops = pit_data.get("numPitStops", 0)

        if status == 1 and prev == 0:
            alerts.append({
                "level": "info",
                "message": f"Box, box! Car is entering the pit lane (pit stop #{num_stops + 1})",
                "time": f"{ts} PIT",
            })
        elif status == 2 and prev != 2:
            alerts.append({
                "level": "info",
                "message": f"Car has entered the pit box (pit stop #{num_stops + 1})",
                "time": f"{ts} PIT",
            })
        elif prev == 2 and status != 2:
            pit_stop_ms = pit_data.get("pitStopTimeMs", 0)
            time_str = f" — stationary time {pit_stop_ms / 1000:.1f}s" if pit_stop_ms > 0 else ""
            alerts.append({
                "level": "info",
                "message": f"Car has left the pit box{time_str} (total stops: {num_stops})",
                "time": f"{ts} PIT",
            })
        elif status == 0 and prev == 1:
            pit_lane_ms = pit_data.get("pitLaneTimeMs", 0)
            time_str = f" — pit lane time {pit_lane_ms / 1000:.1f}s" if pit_lane_ms > 0 else ""
            alerts.append({
                "level": "info",
                "message": f"Car has left the pit lane{time_str} (total stops: {num_stops})",
                "time": f"{ts} PIT",
            })

        return alerts
