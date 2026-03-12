"""
Loads and caches track outline coordinates for the terminal viewer.

Reuses load_track() and subsample() from scripts/build_track_data.py.
Adds a track_id → filename mapping for runtime use (m_trackId from session packet).

Usage:
    loader = TrackLoader()
    points = loader.load(track_id=0)  # Melbourne — list of (x, z) tuples
"""

from pathlib import Path

# Reuse the existing coordinate loader
from scripts.build_track_data import COORDS_DIR
from scripts.build_track_data import load_track
from scripts.build_track_data import subsample

# Map from F1 25 track ID (m_trackId in session packet) to coordinate filename.
# These track IDs come from the official F1 25 UDP spec appendix.
TRACK_ID_TO_FILE: dict[int, str] = {
    0: "melbourne_coordinates.txt",       # Melbourne
    2: "shanghai_coordinates.txt",         # Shanghai
    3: "bahrain_coordinates.txt",          # Sakhir (Bahrain)
    4: "catalunya_coordinates.txt",        # Catalunya
    5: "monaco_coordinates.txt",           # Monaco
    6: "montreal_coordinates.txt",         # Montreal
    7: "silverstone_coordinates.txt",      # Silverstone
    9: "hungaroring_coordinates.txt",      # Hungaroring
    10: "spa_coordinates.txt",             # Spa
    11: "monza_coordinates.txt",           # Monza
    12: "singapore_coordinates.txt",       # Singapore
    13: "suzuka_coordinates.txt",          # Suzuka
    14: "abu_dhabi_coordinates.txt",       # Abu Dhabi
    15: "texas_coordinates.txt",           # Texas
    16: "brazil_coordinates.txt",          # Brazil
    17: "austria_coordinates.txt",         # Austria
    19: "mexico_coordinates.txt",          # Mexico
    20: "baku_coordinates.txt",            # Baku (Azerbaijan)
    26: "zandvoort_coordinates.txt",       # Zandvoort
    27: "imola_coordinates.txt",           # Imola
    29: "jeddah_coordinates.txt",          # Jeddah
    30: "miami_coordinates.txt",           # Miami
    31: "las_vegas_coordinates.txt",       # Las Vegas
    32: "losail_coordinates.txt",          # Losail
}

TARGET_POINTS = 200  # Subsample to this many points for ASCII rendering


class TrackLoader:
    """Loads and caches track outline coordinates."""

    def __init__(self):
        """Initialize with empty cache."""
        self._cache: dict[int, list[tuple[float, float]]] = {}

    def load(self, track_id: int) -> list[tuple[float, float]]:
        """Load track outline as a list of (x, z) world-space coordinates.

        Args:
            track_id: The m_trackId from the session packet.

        Returns:
            List of (x, z) tuples, subsampled for terminal rendering.
            Empty list if track not found.
        """
        if track_id in self._cache:
            return self._cache[track_id]

        filename = TRACK_ID_TO_FILE.get(track_id)
        if not filename:
            return []

        points = load_track(filename)
        if points:
            points = subsample(points, TARGET_POINTS)

        self._cache[track_id] = points
        return points
