"""
Entry point for the F1 25 telemetry capture and replay.

Usage:
    # Capture at 20 Hz with live terminal viewer
    python -m common.f1_capture --hz 20 --live

    # Capture at 60 Hz, no viewer
    python -m common.f1_capture --hz 60

    # View-only mode (no .f1bin output)
    python -m common.f1_capture --live --no-capture

    # Replay a captured session at 5x speed with terminal viewer
    python -m common.f1_capture --replay data/session.f1bin --speed 5

    # Custom port and output directory
    python -m common.f1_capture --hz 10 --live --port 20777 --output data/captures
"""

import argparse
import asyncio
import signal

from common.f1_capture.capture_session import CaptureSession


def main():
    """Parse arguments and run the capture or replay session."""
    parser = argparse.ArgumentParser(
        prog="f1_capture",
        description="F1 25 telemetry capture server with optional live terminal viewer.",
    )
    parser.add_argument(
        "--hz", type=int, default=10,
        help="Capture frequency for high-rate packets in Hz (default: 10)",
    )
    parser.add_argument(
        "--port", type=int, default=20777,
        help="UDP port to listen on (default: 20777)",
    )
    parser.add_argument(
        "--output", type=str, default="data",
        help="Output directory for .f1bin capture files (default: data/)",
    )
    parser.add_argument(
        "--live", action="store_true",
        help="Show live terminal viewer",
    )
    parser.add_argument(
        "--no-capture", action="store_true",
        help="Disable .f1bin file capture (view-only mode)",
    )
    parser.add_argument(
        "--replay", type=str, default=None,
        help="Replay a previously captured .f1bin file instead of live capture",
    )
    parser.add_argument(
        "--speed", type=float, default=1.0,
        help="Replay speed multiplier (default: 1.0 = real-time, 10.0 = 10x faster)",
    )
    parser.add_argument(
        "--keep-pauses", action="store_true",
        help="Preserve original game pauses during replay (by default pauses are skipped)",
    )
    args = parser.parse_args()

    from pathlib import Path

    if args.replay:
        _run_replay(args, Path(args.replay))
    else:
        _run_capture(args, Path(args.output))


def _run_capture(args, output_dir):
    """Run live capture mode."""
    session = CaptureSession(
        hz=args.hz,
        port=args.port,
        output_dir=output_dir,
        capture=not args.no_capture,
    )

    async def run():
        """Run capture with optional viewer."""
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, session.stop)

        tasks = [asyncio.create_task(session.run())]

        if args.live:
            from common.f1_viewer.terminal_viewer import TerminalViewer
            viewer = TerminalViewer(session, mode="CAPTURE")
            tasks.append(asyncio.create_task(viewer.run(refresh_hz=10)))

        done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
        for t in pending:
            t.cancel()

    mode = "view-only" if args.no_capture else f"capture @ {args.hz} Hz"
    live = " + live viewer" if args.live else ""
    print(f"[f1_capture] Starting: {mode}{live}")
    print(f"[f1_capture] UDP port: {args.port}, output: {args.output}/")
    print(f"[f1_capture] Press Ctrl+C to stop\n")

    asyncio.run(run())


def _run_replay(args, replay_path):
    """Run replay mode."""
    from pathlib import Path
    from common.f1_capture.replay_session import ReplaySession

    if not replay_path.exists():
        print(f"[f1_capture] Error: file not found: {replay_path}")
        return

    max_gap = None if args.keep_pauses else 0.5
    session = ReplaySession(path=replay_path, speed=args.speed, max_gap=max_gap)

    async def run():
        """Run replay with terminal viewer."""
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, session.stop)

        from common.f1_viewer.terminal_viewer import TerminalViewer
        viewer = TerminalViewer(session, mode="REPLAY")

        tasks = [
            asyncio.create_task(session.run()),
            asyncio.create_task(viewer.run(refresh_hz=10)),
        ]

        done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
        for t in pending:
            t.cancel()

    speed_str = f"{args.speed}x" if args.speed != 1.0 else "real-time"
    print(f"[f1_capture] Replaying: {replay_path}")
    print(f"[f1_capture] Speed: {speed_str}")
    print(f"[f1_capture] Press Ctrl+C to stop\n")

    asyncio.run(run())


if __name__ == "__main__":
    main()
