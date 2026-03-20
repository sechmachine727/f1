"""Mapping from F1 25 game trackId to MultiViewer API circuitKey."""


class TrackIdMap:
    """Provides the verified mapping from F1 25 trackId to MultiViewer circuitKey."""

    TRACK_ID_TO_CIRCUIT_KEY: dict[int, int] = {
        0: 10,    # Melbourne
        2: 49,    # Shanghai
        3: 63,    # Bahrain
        4: 15,    # Catalunya
        5: 22,    # Monaco
        6: 23,    # Montreal
        7: 2,     # Silverstone
        9: 4,     # Hungaroring
        10: 7,    # Spa
        11: 39,   # Monza
        12: 61,   # Singapore
        13: 46,   # Suzuka
        14: 70,   # Abu Dhabi
        15: 9,    # COTA
        16: 14,   # Brazil
        17: 19,   # Austria
        19: 65,   # Mexico
        20: 144,  # Baku
        26: 55,   # Zandvoort
        27: 6,    # Imola
        29: 149,  # Jeddah
        30: 151,  # Miami
        31: 152,  # Las Vegas
        32: 150,  # Losail
    }
