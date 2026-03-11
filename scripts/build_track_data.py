"""Download racing line data from the reference repo and build a compact
TypeScript track data file for the Race Engineer Hub frontend.

Usage:
    python scripts/build_track_data.py
"""

import csv
import io
import json
import math
import urllib.request

BASE_URL = (
    "https://raw.githubusercontent.com/Fredrik2002/f1-25-telemetry-application/main/tracks/"
)

# Map from our TRACK_NAMES values to the reference repo filenames.
# The reference files use pos_x → worldX, pos_z → worldZ (same coordinate system).
TRACK_FILE_MAP = {
    "AUSTRALIAN GP": "melbourne_2020_racingline.txt",
    "FRENCH GP": "paul_ricard_2020_racingline.txt",
    "CHINESE GP": "shanghai_2020_racingline.txt",
    "BAHRAIN GP": "Bahrain_racingline.txt",
    "SPANISH GP": "catalunya_2020_racingline.txt",
    "MONACO GP": "monaco_2020_racingline.txt",
    "CANADIAN GP": "montreal_2020_racingline.txt",
    "BRITISH GP": "silverstone_2020_racingline.txt",
    "HUNGARIAN GP": "hungaroring_2020_racingline.txt",
    "BELGIAN GP": "spa_2020_racingline.txt",
    "ITALIAN GP": "monza_2020_racingline.txt",
    "SINGAPORE GP": "singapore_2020_racingline.txt",
    "JAPANESE GP": "suzuka_2020_racingline.txt",
    "ABU DHABI GP": "abu_dhabi_2020_racingline.txt",
    "UNITED STATES GP": "texas_2020_racingline.txt",
    "BRAZILIAN GP": "brazil_2020_racingline.txt",
    "AUSTRIAN GP": "austria_2020_racingline.txt",
    "RUSSIAN GP": "sochi_2020_racingline.txt",
    "MEXICAN GP": "mexico_2020_racingline.txt",
    "AZERBAIJAN GP": "baku_2020_racingline.txt",
    "DUTCH GP": "zandvoort_2020_racingline.txt",
    "EMILIA ROMAGNA GP": "imola_2020_racingline.txt",
    "SAUDI ARABIAN GP": "jeddah_2020_racingline.txt",
    "MIAMI GP": "miami_2020_racingline.txt",
    "LAS VEGAS GP": "Las Vegas_2020_racingline.txt",
    "QATAR GP": "losail_2020_racingline.txt",
    "VIETNAMESE GP": "hanoi_2020_racingline.txt",
    "BAHRAIN SHORT": "sakhir_2020_racingline.txt",
}

TARGET_POINTS = 150  # max points per track after subsampling


def fetch_track(filename: str) -> list[tuple[float, float]]:
    """Download a racing line file and return a list of (x, z) tuples."""
    url = BASE_URL + urllib.request.quote(filename)
    print(f"  Fetching {filename} ...")
    try:
        with urllib.request.urlopen(url, timeout=15) as resp:
            text = resp.read().decode("utf-8")
    except Exception as exc:
        print(f"    FAILED: {exc}")
        return []

    points: list[tuple[float, float]] = []
    reader = csv.reader(io.StringIO(text))
    header = next(reader, None)
    if not header:
        return []

    # Find column indices — headers may be quoted
    cols = [c.strip().strip('"').lower() for c in header]
    # The first line might be a metadata line, not the actual CSV header
    if "pos_x" not in cols:
        # Try reading the next line as header
        header = next(reader, None)
        if not header:
            return []
        cols = [c.strip().strip('"').lower() for c in header]

    try:
        ix = cols.index("pos_x")
        iz = cols.index("pos_z")
    except ValueError:
        print(f"    Could not find pos_x/pos_z columns in {cols}")
        return []

    for row in reader:
        try:
            x = float(row[ix])
            z = float(row[iz])
            points.append((x, z))
        except (ValueError, IndexError):
            continue

    return points


def subsample(points: list[tuple[float, float]], target: int) -> list[tuple[float, float]]:
    """Subsample a polyline to approximately `target` evenly-spaced points."""
    if len(points) <= target:
        return points

    # Compute cumulative arc length
    lengths = [0.0]
    for i in range(1, len(points)):
        dx = points[i][0] - points[i - 1][0]
        dz = points[i][1] - points[i - 1][1]
        lengths.append(lengths[-1] + math.sqrt(dx * dx + dz * dz))

    total = lengths[-1]
    if total == 0:
        return points[:target]

    step = total / target
    result: list[tuple[float, float]] = [points[0]]
    next_dist = step
    j = 1

    for i in range(1, len(points)):
        while next_dist <= lengths[i] and j < target:
            # Interpolate between points[i-1] and points[i]
            t = (next_dist - lengths[i - 1]) / (lengths[i] - lengths[i - 1]) if lengths[i] != lengths[i - 1] else 0
            x = points[i - 1][0] + t * (points[i][0] - points[i - 1][0])
            z = points[i - 1][1] + t * (points[i][1] - points[i - 1][1])
            result.append((round(x, 1), round(z, 1)))
            next_dist += step
            j += 1

    return result


def main():
    """Build the track data and write it to a TypeScript file."""
    tracks: dict[str, list[list[float]]] = {}

    for track_name, filename in sorted(TRACK_FILE_MAP.items()):
        print(f"[{track_name}]")
        points = fetch_track(filename)
        if not points:
            print(f"    Skipped (no data)")
            continue
        sampled = subsample(points, TARGET_POINTS)
        # Store as [[x, z], ...] for compactness
        tracks[track_name] = [[p[0], p[1]] for p in sampled]
        print(f"    {len(points)} pts → {len(sampled)} pts")

    # Write TypeScript file
    out_path = "race_engineer_hub/src/data/trackOutlines.ts"
    print(f"\nWriting {out_path} ({len(tracks)} tracks) ...")

    lines = [
        "/** Static track outlines keyed by track name (from TRACK_NAMES).",
        " *  Each value is an array of [x, z] world-coordinate pairs.",
        " *  Generated by scripts/build_track_data.py — do not edit by hand.",
        " */",
        f"export const TRACK_OUTLINES: Record<string, [number, number][]> = {json.dumps(tracks, separators=(',', ':'))};",
    ]

    import os
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")

    print("Done.")


if __name__ == "__main__":
    main()
