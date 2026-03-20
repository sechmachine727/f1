"""Loads and serves pre-built circuit info from circuits.json."""

import json
from pathlib import Path


class CircuitInfoProvider:
    """Load static circuit data from the pre-built JSON file and serve by trackId."""

    def __init__(self) -> None:
        """Load circuits.json into memory."""
        json_path = Path(__file__).parent / "circuits.json"
        if json_path.exists():
            with open(json_path, encoding="utf-8") as f:
                self._data: dict[str, dict] = json.load(f)
        else:
            self._data = {}

    def get(self, track_id: int) -> dict | None:
        """Return circuit info dict for a given F1 25 trackId, or None if unavailable."""
        return self._data.get(str(track_id))
