"""Provides track location context (nearest corner, sector, lap %) for alert enrichment.

Loads corner positions from circuits.json and maps them to normalized distances
along the track outline so that a player's lap distance can be resolved to
"approaching Turn 12, S2 (45%), lap 67%".
"""

import json
import math
from pathlib import Path

CIRCUITS_JSON = Path(__file__).resolve().parent.parent / "race_engineer_hub" / "src" / "data" / "circuits.json"


class TrackLocationProvider:
    """Resolves a lap distance to a human-readable location string."""

    def __init__(self) -> None:
        """Load circuits.json and initialise with no active track."""
        self._circuits: dict = {}
        try:
            with open(CIRCUITS_JSON, encoding="utf-8") as f:
                self._circuits = json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            pass

        self._track_id: int = -1
        # Sorted list of (normalized_distance, corner_number) for the active track
        self._corners: list[tuple[float, float]] = []

    def set_track(self, track_id: int) -> None:
        """Precompute corner positions for a new track."""
        if track_id == self._track_id:
            return
        self._track_id = track_id
        self._corners = []

        circuit = self._circuits.get(str(track_id))
        if not circuit:
            return

        outline = circuit.get("outline", [])
        corners = circuit.get("corners", [])
        if not outline or not corners:
            return

        # Build cumulative arc-length distances along the outline
        cum_dist = [0.0]
        for i in range(1, len(outline)):
            dx = outline[i][0] - outline[i - 1][0]
            dy = outline[i][1] - outline[i - 1][1]
            cum_dist.append(cum_dist[-1] + math.sqrt(dx * dx + dy * dy))
        total = cum_dist[-1]
        if total <= 0:
            return

        # For each corner, find the nearest outline point and derive its normalised position
        corner_norms: list[tuple[float, int]] = []
        for c in corners:
            cx, cy = c.get("x", 0), c.get("y", 0)
            best_idx = 0
            best_dist_sq = float("inf")
            for j, pt in enumerate(outline):
                dsq = (pt[0] - cx) ** 2 + (pt[1] - cy) ** 2
                if dsq < best_dist_sq:
                    best_dist_sq = dsq
                    best_idx = j
            norm = cum_dist[best_idx] / total
            corner_norms.append((norm, c.get("number", 0)))

        # Sort by normalised distance around the lap
        corner_norms.sort(key=lambda x: x[0])
        self._corners = corner_norms

    def describe(self, lap_distance: float, track_length: float,
                 sector2_start: float, sector3_start: float) -> str:
        """Return a location string like 'approaching Turn 5, S2 (34%), lap 62%'.

        Args:
            lap_distance: Player's current lap distance in metres.
            track_length: Total track length in metres.
            sector2_start: Sector 2 start distance in metres.
            sector3_start: Sector 3 start distance in metres.

        Returns:
            Human-readable location string, or empty string if unavailable.
        """
        if track_length <= 0:
            return ""

        norm = (lap_distance / track_length) % 1.0
        lap_pct = round(norm * 100)

        # Sector
        if sector2_start > 0 and sector3_start > 0:
            if lap_distance < sector2_start:
                sector_num = 1
                sector_start = 0.0
                sector_end = sector2_start
            elif lap_distance < sector3_start:
                sector_num = 2
                sector_start = sector2_start
                sector_end = sector3_start
            else:
                sector_num = 3
                sector_start = sector3_start
                sector_end = track_length
            sector_len = sector_end - sector_start
            sector_pct = round(((lap_distance - sector_start) / sector_len) * 100) if sector_len > 0 else 0
            sector_str = f"S{sector_num} ({sector_pct}%)"
        else:
            sector_str = ""

        # Corner
        corner_str = self._nearest_corner(norm)

        parts = [p for p in [corner_str, sector_str, f"lap {lap_pct}%"] if p]
        return ", ".join(parts)

    def _nearest_corner(self, norm: float) -> str:
        """Find the nearest corner and describe the relationship."""
        if not self._corners:
            return ""

        # Threshold: within 1.5% of track = "at", within 3% ahead = "approaching"
        at_threshold = 0.015
        approach_threshold = 0.03

        best_corner = 0
        best_delta = float("inf")
        best_raw_delta = 0.0  # positive = corner is ahead, negative = corner is behind

        for cnorm, cnum in self._corners:
            # Signed distance: positive means corner is ahead
            delta = cnorm - norm
            # Wrap around for circular track
            if delta > 0.5:
                delta -= 1.0
            elif delta < -0.5:
                delta += 1.0

            abs_delta = abs(delta)
            if abs_delta < best_delta:
                best_delta = abs_delta
                best_corner = cnum
                best_raw_delta = delta

        if best_corner == 0:
            return ""

        if best_delta <= at_threshold:
            return f"at Turn {best_corner}"
        elif 0 < best_raw_delta <= approach_threshold:
            return f"approaching Turn {best_corner}"
        else:
            return f"near Turn {best_corner}"
