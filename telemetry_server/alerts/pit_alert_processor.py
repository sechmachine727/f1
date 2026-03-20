"""Pit status alert processor — detects pit lane and pit box transitions."""

from telemetry_server.alerts.alert_utils import format_session_time


class PitAlertProcessor:
    """Detects pit lane and pit box transitions and generates alerts for the race engineer.

    Tracks ``pitStatus`` transitions (0=none, 1=pitting, 2=in pit area)
    and fires an alert when the car enters/leaves the pit lane or pit box.
    """

    # Session types that are races (pit stop counts are meaningful).
    _RACE_SESSION_TYPES = {10, 11, 12, 15, 17}

    def __init__(self) -> None:
        """Initialise with no previous pit status."""
        self._prev_pit_status: int = 0
        self._prev_driver_status: int = 0
        self._initialized: bool = False
        self.prev_session_time: float = 0.0

    def reset(self) -> None:
        """Clear state (e.g. on session change)."""
        self._prev_pit_status = 0
        self._prev_driver_status = 0
        self._initialized = False
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
        driver_status = pit_data.get("driverStatus", 0)

        # First tick after init/reset: seed previous values, suppress alerts.
        if not self._initialized:
            self._prev_pit_status = status
            self._prev_driver_status = driver_status
            self._initialized = True
            return []

        ts = format_session_time(session_time)
        alerts: list[dict] = []

        is_race = pit_data.get("sessionType", 0) in self._RACE_SESSION_TYPES
        num_stops = pit_data.get("numPitStops", 0)

        # -- Pit status transitions --------------------------------------------
        prev = self._prev_pit_status
        self._prev_pit_status = status

        if status != prev:
            if status == 1 and prev == 0:
                msg = "Box, box! Car is entering the pit lane"
                if is_race:
                    msg += f" (pit stop #{num_stops + 1})"
                alerts.append({"level": "pit", "message": msg, "time": f"{ts} PIT"})
            elif status == 2 and prev != 2:
                msg = "Car has entered the pit box"
                if is_race:
                    msg += f" (pit stop #{num_stops + 1})"
                alerts.append({"level": "pit", "message": msg, "time": f"{ts} PIT"})
            elif prev == 2 and status != 2:
                pit_stop_ms = pit_data.get("pitStopTimeMs", 0)
                time_str = f" — stationary time {pit_stop_ms / 1000:.1f}s" if pit_stop_ms > 0 else ""
                msg = f"Car has left the pit box{time_str}"
                if is_race:
                    msg += f" (total stops: {num_stops})"
                alerts.append({"level": "pit", "message": msg, "time": f"{ts} PIT"})
            elif status == 0 and prev == 1:
                pit_lane_ms = pit_data.get("pitLaneTimeMs", 0)
                time_str = f" — pit lane time {pit_lane_ms / 1000:.1f}s" if pit_lane_ms > 0 else ""
                msg = f"Car has left the pit lane{time_str}"
                if is_race:
                    msg += f" (total stops: {num_stops})"
                alerts.append({"level": "pit", "message": msg, "time": f"{ts} PIT"})

        # -- Driver status transitions (garage detection) ----------------------
        prev_driver = self._prev_driver_status
        self._prev_driver_status = driver_status

        if driver_status != prev_driver:
            if prev_driver == 0 and driver_status != 0:
                alerts.append({
                    "level": "pit",
                    "message": "Car has left the garage — on track",
                    "time": f"{ts} PIT",
                })
            elif driver_status == 0 and prev_driver != 0:
                alerts.append({
                    "level": "pit",
                    "message": "Car has entered the garage",
                    "time": f"{ts} PIT",
                })

        return alerts
