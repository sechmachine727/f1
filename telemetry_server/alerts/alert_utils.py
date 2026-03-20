"""Shared utility functions for alert processing."""


def format_session_time(seconds: float) -> str:
    """Format session time in seconds as MM:SS."""
    total = int(seconds)
    m = total // 60
    s = total % 60
    return f"{m:02d}:{s:02d}"


def is_copy_ack(text: str) -> bool:
    """Return True if the text is a bare 'Copy' acknowledgment, ignoring markdown bold and trailing punctuation."""
    return text.strip().strip("*").strip().rstrip(".").strip().lower() == "copy"
