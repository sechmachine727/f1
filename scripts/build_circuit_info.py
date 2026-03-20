"""Fetch circuit data from the MultiViewer API and write circuits.json.

Usage:
    python scripts/build_circuit_info.py [--year YEAR]

Fetches corner, marshal sector, and track outline data for all 24 F1 25 tracks
and writes common/circuit_info/circuits.json keyed by trackId.  The API provides
x/y track outline arrays and trackPosition coordinates for corners and sectors
in the same coordinate system, so everything is spatially consistent.
"""

import argparse
import json
import sys
import time
import urllib.request
from pathlib import Path

# Add repo root to path so we can import our mapping
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from common.circuit_info.track_id_map import TrackIdMap
from common.f1_structs.f1_constants import TRACK_NAMES

API_BASE = "https://api.multiviewer.app/api/v1/circuits"
OUTPUT_PATH = Path(__file__).resolve().parent.parent / "race_engineer_hub" / "src" / "data" / "circuits.json"


def fetch_circuit(circuit_key: int, year: int) -> dict | None:
    """Fetch circuit data from the MultiViewer API."""
    url = f"{API_BASE}/{circuit_key}/{year}"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "F1-Telemetry-Build/1.0"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except Exception as exc:
        print(f"  FAILED to fetch circuitKey={circuit_key}: {exc}")
        return None


def build_circuit_data(track_id: int, circuit_key: int, name: str, year: int) -> dict | None:
    """Fetch and process circuit data for one track."""
    data = fetch_circuit(circuit_key, year)
    if not data:
        return None

    corners = data.get("corners", [])
    marshal_sectors = data.get("marshalSectors", [])
    rotation = data.get("rotation", 0)
    x_outline = data.get("x", [])
    y_outline = data.get("y", [])

    if not x_outline or not y_outline or len(x_outline) != len(y_outline):
        print(f"  WARNING: no track outline for {name}")
        return None

    # Track outline as [[x, y], ...] pairs
    outline = [[x_outline[i], y_outline[i]] for i in range(len(x_outline))]

    # Corners with track position coordinates
    corner_list = []
    for c in corners:
        pos = c.get("trackPosition", {})
        corner_list.append({
            "number": c.get("number", 0),
            "letter": c.get("letter", ""),
            "angle": c.get("angle", 0),
            "x": pos.get("x", 0),
            "y": pos.get("y", 0),
        })

    # Marshal sectors with track position coordinates
    sector_list = []
    for s in marshal_sectors:
        pos = s.get("trackPosition", {})
        sector_list.append({
            "number": s.get("number", 0),
            "x": pos.get("x", 0),
            "y": pos.get("y", 0),
        })

    return {
        "name": name,
        "rotation": rotation,
        "outline": outline,
        "corners": corner_list,
        "marshalSectors": sector_list,
    }


def main():
    """Fetch all circuits and write circuits.json."""
    parser = argparse.ArgumentParser(description="Build circuit info JSON from MultiViewer API")
    parser.add_argument("--year", type=int, default=2025, help="Season year (default: 2025)")
    args = parser.parse_args()

    circuits: dict[str, dict] = {}

    for track_id, circuit_key in TrackIdMap.TRACK_ID_TO_CIRCUIT_KEY.items():
        name = TRACK_NAMES.get(track_id, f"Track {track_id}")
        print(f"Fetching {name} (trackId={track_id}, circuitKey={circuit_key})...")

        result = build_circuit_data(track_id, circuit_key, name, args.year)
        if result:
            circuits[str(track_id)] = result
            n_outline = len(result["outline"])
            n_corners = len(result["corners"])
            n_sectors = len(result["marshalSectors"])
            print(f"  OK: {n_outline} outline pts, {n_corners} corners, {n_sectors} marshal sectors")
        else:
            print(f"  SKIPPED")

        time.sleep(0.3)  # Be polite to the API

    OUTPUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(OUTPUT_PATH, "w", encoding="utf-8") as f:
        json.dump(circuits, f, indent=2)

    print(f"\nWrote {len(circuits)} circuits to {OUTPUT_PATH}")


if __name__ == "__main__":
    main()
