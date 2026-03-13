"""
Live terminal viewer for F1 25 telemetry.

Renders a rich dashboard using the alternate screen buffer for flicker-free
output with ANSI escape codes and Unicode box-drawing characters:
  - Session header with position, lap, sector times, weather forecast
  - TYRES panel with temps, wear, pressures, brakes, tyre/brake damage
  - POWER UNIT panel with speed, RPM, gear, fuel, ERS, steer input
  - AERODYNAMICS panel with wing/floor/diffuser/sidepod damage, faults
  - SETUP & DYNAMICS panel with car setup and MotionEx data
  - ASCII track map with all car positions via lap-distance interpolation
  - EVENTS scrolling log

No external dependencies — uses only ANSI codes and Unicode box-drawing.

Usage:
    viewer = TerminalViewer(session)
    await viewer.run(refresh_hz=10)
"""

import asyncio
import math
import os
import re
import sys
from collections import deque

from common.f1_viewer.track_loader import TrackLoader

# ─── ANSI codes ───
_HOME = "\033[H"            # Cursor home — redraws in-place without clearing
_ALT_ON = "\033[?1049h"    # Enter alternate screen buffer
_ALT_OFF = "\033[?1049l"   # Leave alternate screen buffer
_HIDE_CUR = "\033[?25l"    # Hide cursor
_SHOW_CUR = "\033[?25h"    # Show cursor
_ERASE_DOWN = "\033[J"     # Erase from cursor to end of screen
_BOLD = "\033[1m"
_DIM = "\033[2m"
_RESET = "\033[0m"
_RED = "\033[91m"
_GREEN = "\033[92m"
_YELLOW = "\033[93m"
_BLUE = "\033[94m"
_MAGENTA = "\033[95m"
_CYAN = "\033[96m"
_WHITE = "\033[97m"

_ANSI_RE = re.compile(r"\033\[[0-9;]*m")

# Tyre compound display
_COMPOUND_NAMES = {16: "SOFT", 17: "MEDIUM", 18: "HARD", 7: "INTER", 8: "WET"}
_COMPOUND_COLORS = {16: _RED, 17: _YELLOW, 18: _WHITE, 7: _GREEN, 8: _BLUE}

# Lookup tables
_WEATHER = {0: "Clear", 1: "Light Cloud", 2: "Overcast", 3: "Light Rain", 4: "Heavy Rain", 5: "Storm"}
_WEATHER_ICON = {0: "☀", 1: "⛅", 2: "☁", 3: "🌧", 4: "🌧🌧", 5: "⛈"}
_SESSION_TYPES = {
    0: "Unknown", 1: "FP1", 2: "FP2", 3: "FP3", 4: "Short FP",
    5: "Q1", 6: "Q2", 7: "Q3", 8: "Short Q", 9: "OSQ",
    10: "SS1", 11: "SS2", 12: "SS3", 15: "Race", 18: "Time Trial",
}
_FUEL_MIX = {0: "Lean", 1: "Standard", 2: "Rich", 3: "Max"}
_ERS_MODE = {0: "None", 1: "Medium", 2: "Hotlap", 3: "Overtake"}
_SC_STATUS = {0: "", 1: "Full SC", 2: "VSC", 3: "Formation Lap"}

_EVENT_NAMES = {
    "SSTA": "Session Started", "SEND": "Session Ended", "FTLP": "Fastest Lap",
    "RTMT": "Retirement", "DRSE": "DRS Enabled", "DRSD": "DRS Disabled",
    "TMPT": "Team Mate in Pits", "CHQF": "Chequered Flag", "RCWN": "Race Winner",
    "PENA": "Penalty", "SPTP": "Speed Trap", "STLG": "Start Lights",
    "LGOT": "Lights Out", "DTSV": "Drive Through Served", "SGSV": "Stop-Go Served",
    "FLBK": "Flashback", "BUTN": "Button Press", "RDFL": "Red Flag",
    "OVTK": "Overtake", "SGAY": "Safety Car", "COLL": "Collision",
}

_MAX_EVENTS = 8  # Number of events to show in the scrolling log


class TerminalViewer:
    """Renders live telemetry state to the terminal."""

    def __init__(self, session, mode: str = "CAPTURE"):
        """Initialize the viewer.

        Args:
            session: A running CaptureSession or ReplaySession whose .state dict we read from.
            mode: Display mode label — "CAPTURE" or "REPLAY".
        """
        self._session = session
        self._mode = mode
        self._track_loader = TrackLoader()
        self._static_track: _StaticTrack | None = None
        self._loaded_track_id: int = -999
        self._events: deque[str] = deque(maxlen=_MAX_EVENTS)
        self._last_event_frame: int = -1

    async def run(self, refresh_hz: int = 10) -> None:
        """Render loop.

        Args:
            refresh_hz: Display refresh rate in Hz.
        """
        interval = 1.0 / refresh_hz
        # Enter alternate screen buffer — prevents scroll-back pollution
        sys.stdout.write(_ALT_ON + _HIDE_CUR)
        sys.stdout.flush()
        try:
            while self._session._running:
                self._render()
                await asyncio.sleep(interval)
        finally:
            sys.stdout.write(_SHOW_CUR + _ALT_OFF)
            sys.stdout.flush()

    def _render(self) -> None:
        """Build and print one frame to stdout."""
        state = self._session.state
        if not state:
            return  # Nothing to render yet

        try:
            cols, _ = os.get_terminal_size()
        except OSError:
            cols = 120
        cols = min(cols, 200)  # Sanity cap

        lines: list[str] = []

        sess = state.get(1)
        telem = state.get(6)
        status = state.get(7)
        lap = state.get(2)
        damage = state.get(10)
        motion = state.get(0)
        parts = state.get(4)
        motion_ex = state.get(13)
        setups = state.get(5)
        event = state.get(3)

        player_idx = None
        for src in (sess, telem, lap, status):
            if src and "m_playerCarIndex" in src:
                player_idx = src["m_playerCarIndex"]
                break

        # Collect events
        self._collect_events(event)

        lines += self._render_header(sess, lap, player_idx, cols)

        panel_w = (cols - 3) // 2

        # Only render panels once core telemetry data has arrived
        if telem and player_idx is not None:
            # Row 1: TYRES | POWER UNIT
            car_lines = self._render_car_panel(telem, damage, status, setups, motion_ex, player_idx)
            pu_lines = self._render_pu_panel(telem, status, damage, player_idx)
            lines += _side_by_side(car_lines, pu_lines, panel_w)

            # Row 2: AERODYNAMICS | SETUP & DYNAMICS
            aero_lines = self._render_aero_damage_panel(damage, motion_ex, player_idx)
            setup_lines = self._render_setup_panel(setups, motion_ex, status, player_idx)
            lines += _side_by_side(aero_lines, setup_lines, panel_w)

            # Row 3: TRACK MAP | EVENTS
            track_lines = self._render_track_map(sess, motion, lap, parts, player_idx, panel_w)
            event_lines = self._render_events_panel(panel_w)
            lines += _side_by_side(track_lines, event_lines, panel_w)
        else:
            lines.append(f" {_DIM}Waiting for telemetry...{_RESET}")

        # Footer
        lines.append(f" {_DIM}{'─' * (cols - 2)}{_RESET}")
        n_active = parts.get("m_numActiveCars", 0) if parts else 0
        hz_label = self._session.hz
        total = ""
        if hasattr(self._session, "total_frames"):
            total = f"/{self._session.total_frames:,}"
        progress = ""
        if hasattr(self._session, "progress"):
            pct = self._session.progress * 100
            progress = f"  │  {pct:.0f}%"
        lines.append(
            f" {_DIM}Packets: {self._session.packets_received:,}{total}"
            f"  │  {self._session.packets_captured:,} decoded"
            f"  │  {hz_label}"
            f"  │  {n_active} cars{progress}{_RESET}"
        )

        # Truncate any line that would overflow the terminal width
        safe = []
        for line in lines:
            vis = _visible_len(line)
            if vis >= cols:
                line = _truncate(line, cols - 1)
            safe.append(line)

        sys.stdout.write(_HOME + "\n".join(safe) + _ERASE_DOWN + "\n")
        sys.stdout.flush()

    # ─── EVENT COLLECTION ───

    def _collect_events(self, event) -> None:
        """Accumulate events into the scrolling log."""
        if not event:
            return
        frame = event.get("m_overallFrameIdentifier", 0)
        if frame == self._last_event_frame:
            return
        self._last_event_frame = frame
        code = event.get("m_eventStringCode", "")
        if not code or code in ("BUTN",):
            return
        name = _EVENT_NAMES.get(code, code)
        t = event.get("m_sessionTime", 0)
        mins, secs = divmod(t, 60)
        self._events.append(f"{_DIM}{int(mins):02d}:{secs:05.2f}{_RESET}  {name}")

    # ─── HEADER ───

    def _render_header(self, sess, lap, player_idx, cols) -> list[str]:
        """Render the top header bar with weather forecast and sector times."""
        lines = []
        lines.append(f" {_BOLD}{_CYAN}{'═' * (cols - 2)}{_RESET}")

        if sess:
            sess_type = _SESSION_TYPES.get(sess.get("m_sessionType", 0), "?")
            weather = _WEATHER.get(sess.get("m_weather", 0), "?")
            track_temp = sess.get("m_trackTemperature", 0)
            air_temp = sess.get("m_airTemperature", 0)
            time_left = sess.get("m_sessionTimeLeft", 0)
            track_len = sess.get("m_trackLength", 0)
            sc = _SC_STATUS.get(sess.get("m_safetyCarStatus", 0), "")
            total_laps = sess.get("m_totalLaps", 0)
            mins, secs = divmod(time_left, 60)

            pos_str = lap_str = time_str = last_str = ""
            s1_str = s2_str = s3_str = sector_str = ""
            if lap and player_idx is not None:
                cars = lap.get("m_lapData", [])
                if player_idx < len(cars):
                    p = cars[player_idx]
                    pos_str = f"P{p.get('m_carPosition', 0)}"
                    cur_lap = p.get("m_currentLapNum", 0)
                    lap_str = f"Lap {cur_lap}" + (f"/{total_laps}" if total_laps else "")
                    time_str = _fmt_time(p.get("m_currentLapTimeInMS", 0))
                    last_str = _fmt_time(p.get("m_lastLapTimeInMS", 0))

                    # Sector times
                    cur_sector = p.get("m_sector", 0)
                    s1_ms = p.get("m_sector1TimeMSPart", 0) + p.get("m_sector1TimeMinutesPart", 0) * 60000
                    s2_ms = p.get("m_sector2TimeMSPart", 0) + p.get("m_sector2TimeMinutesPart", 0) * 60000
                    s1_str = _fmt_sector(s1_ms) if s1_ms > 0 else ("--.-" if cur_sector == 0 else _fmt_sector(s1_ms))
                    s2_str = _fmt_sector(s2_ms) if s2_ms > 0 else "--.-"
                    s3_str = "--.-"
                    sector_str = f"S1 {s1_str}  S2 {s2_str}  S3 {s3_str}"

                    # Delta to car in front
                    delta_ms = p.get("m_deltaToCarInFrontMSPart", 0) + p.get("m_deltaToCarInFrontMinutesPart", 0) * 60000
                    if delta_ms > 0:
                        sector_str += f"  │  Δ {delta_ms / 1000:+.3f}s"

            # Line 1: title + position/lap/time
            mode_str = f"{_YELLOW}[REPLAY]{_RESET}" if self._mode == "REPLAY" else f"{_GREEN}LIVE CAPTURE{_RESET}"
            title = f" {_BOLD}{_CYAN}F1 25 TELEMETRY{_RESET} — {mode_str}"
            right = f"{_BOLD}{_WHITE}{pos_str}{_RESET}  {_CYAN}{lap_str}{_RESET}  {_GREEN}{time_str}{_RESET}  Last: {_YELLOW}{last_str}{_RESET}"
            gap = cols - _visible_len(title) - _visible_len(right)
            lines.append(f"{title}{' ' * max(gap, 2)}{right}")
            lines.append(f" {_BOLD}{_CYAN}{'═' * (cols - 2)}{_RESET}")

            # Line 2: session info
            sc_str = f"  │  {_RED}{_BOLD}{sc}{_RESET}" if sc else ""
            lines.append(
                f" {_BOLD}{sess_type}{_RESET}  │  {weather}  │  "
                f"Track: {_color_temp_air(track_temp)}°C{_RESET}  "
                f"Air: {air_temp}°C  │  {track_len}m  │  "
                f"Remaining: {_BOLD}{mins:02d}:{secs:02d}{_RESET}{sc_str}"
            )

            # Line 3: sector times
            if sector_str:
                lines.append(f" {_DIM}Sectors:{_RESET}  {sector_str}")

            # Line 4: weather forecast for the CURRENT session only
            forecasts = sess.get("m_weatherForecastSamples", [])
            n_forecasts = sess.get("m_numWeatherForecastSamples", 0)
            cur_sess_type = sess.get("m_sessionType", 0)
            if forecasts and n_forecasts > 0:
                prefix = f" {_DIM}Forecast:{_RESET}  "
                prefix_vis = 12  # " Forecast:  "
                sep = "  ·  "
                sep_vis = 5
                fc_parts = []
                running_vis = prefix_vis
                for fc in forecasts[:n_forecasts]:
                    if fc.get("m_sessionType", 0) != cur_sess_type:
                        continue
                    offset = fc.get("m_timeOffset", 0)
                    if offset == 0:
                        continue
                    fc_weather = _WEATHER.get(fc.get("m_weather", 0), "?")
                    fc_track = fc.get("m_trackTemperature", 0)
                    fc_rain = fc.get("m_rainPercentage", 0)
                    rain_str = f" {_CYAN}{fc_rain}%{_RESET}" if fc_rain > 0 else ""
                    part = f"+{offset}m {fc_weather} {fc_track}°C{rain_str}"
                    part_vis = _visible_len(part)
                    cost = (sep_vis if fc_parts else 0) + part_vis
                    if running_vis + cost > cols - 6:
                        break
                    fc_parts.append(part)
                    running_vis += cost
                    if len(fc_parts) >= 3:
                        break
                if fc_parts:
                    lines.append(f"{prefix}{sep.join(fc_parts)}")

        else:
            mode_str = f"{_YELLOW}[REPLAY]{_RESET}" if self._mode == "REPLAY" else f"{_GREEN}LIVE CAPTURE{_RESET}"
            lines.append(f" {_BOLD}{_CYAN}F1 25 TELEMETRY{_RESET} — {mode_str}")
            lines.append(f" {_BOLD}{_CYAN}{'═' * (cols - 2)}{_RESET}")
            lines.append(f" {_DIM}Waiting for session data...{_RESET}")

        lines.append("")
        return lines

    # ─── TYRES PANEL ───

    def _render_car_panel(self, telem, damage, status, setups, motion_ex, player_idx) -> list[str]:
        """Render ASCII car with tyre data."""
        lines = []
        c = _CYAN
        W = _BOX_W
        lines.append(f"{_BOLD}{c}┌─ TYRES {'─' * (W - 8)}┐{_RESET}")

        if not all([telem, player_idx is not None]):
            lines.append(_bx(c, f" {_DIM}No telemetry data{_RESET}"))
            lines.append(f"{c}└{'─' * W}┘{_RESET}")
            return lines

        t_cars = telem.get("m_carTelemetryData", [])
        t = t_cars[player_idx] if player_idx < len(t_cars) else {}
        d = {}
        if damage:
            d_cars = damage.get("m_carDamageData", [])
            d = d_cars[player_idx] if player_idx < len(d_cars) else {}
        s = {}
        if status:
            s_cars = status.get("m_carStatusData", [])
            s = s_cars[player_idx] if player_idx < len(s_cars) else {}
        su = {}
        if setups:
            su_cars = setups.get("m_carSetupData", [])
            su = su_cars[player_idx] if player_idx < len(su_cars) else {}

        # Tyre arrays: [RL=0, RR=1, FL=2, FR=3]
        surf = t.get("m_tyresSurfaceTemperature", [0]*4)
        inner = t.get("m_tyresInnerTemperature", [0]*4)
        press = t.get("m_tyresPressure", [0]*4)
        brake = t.get("m_brakesTemperature", [0]*4)
        wear = d.get("m_tyresWear", [0]*4)
        blist = d.get("m_tyreBlisters", [0]*4)

        compound_id = s.get("m_visualTyreCompound", 0)
        compound = _COMPOUND_NAMES.get(compound_id, "?")
        compound_col = _COMPOUND_COLORS.get(compound_id, _WHITE)
        age = s.get("m_tyresAgeLaps", 0)
        bias = su.get("m_brakeBias", s.get("m_frontBrakeBias", 0))

        FL, FR, RL, RR = 2, 3, 0, 1
        TB = f"{_WHITE}████{_RESET}"

        b = lambda content: _bx(c, content)  # noqa: E731
        # Tyre column = 8 chars (matching ──████──), center = 10 chars
        lines.append(b(f"  {compound_col}{_BOLD}{compound}{_RESET} ({age} laps)  Brake Bias: {bias}%"))
        lines.append(b(f"           {_DIM}FL{_RESET}                  {_DIM}FR{_RESET}    "))
        lines.append(b(f"       ┌──{TB}──┬──────────┬──{TB}──┐"))
        lines.append(b(f"  Surf {c}│{_RESET} {_rpad(_ct(surf[FL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_ct(surf[FR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"  Innr {c}│{_RESET} {_rpad(_ct(inner[FL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_ct(inner[FR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"  Wear {c}│{_RESET} {_rpad(_cw(wear[FL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_cw(wear[FR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"  Pres {c}│{_RESET} {press[FL]:>7.1f}{c}│{_RESET}          {c}│{_RESET}{press[FR]:>7.1f} {c}│{_RESET}"))
        lines.append(b(f"  Brak {c}│{_RESET} {_rpad(_cb(brake[FL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_cb(brake[FR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"       {c}│{_RESET}        {c}│{_RESET}          {c}│{_RESET}        {c}│{_RESET}"))
        lines.append(b(f"  Surf {c}│{_RESET} {_rpad(_ct(surf[RL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_ct(surf[RR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"  Wear {c}│{_RESET} {_rpad(_cw(wear[RL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_cw(wear[RR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"  Pres {c}│{_RESET} {press[RL]:>7.1f}{c}│{_RESET}          {c}│{_RESET}{press[RR]:>7.1f} {c}│{_RESET}"))
        lines.append(b(f"  Brak {c}│{_RESET} {_rpad(_cb(brake[RL]), 7)}{c}│{_RESET}          {c}│{_RESET}{_lpad(_cb(brake[RR]), 7)} {c}│{_RESET}"))
        lines.append(b(f"       └──{TB}──┴──────────┴──{TB}──┘"))
        lines.append(b(f"           {_DIM}RL{_RESET}                  {_DIM}RR{_RESET}    "))

        # Tyre damage and brake damage
        tyre_dmg = d.get("m_tyresDamage", [0]*4)
        brake_dmg = d.get("m_brakesDamage", [0]*4)
        if any(v > 0 for v in tyre_dmg):
            lines.append(b(f"  {_DIM}Tyre Dmg{_RESET} FL {_rpad(_cd(tyre_dmg[FL]), 2)}% FR {_rpad(_cd(tyre_dmg[FR]), 2)}% RL {_rpad(_cd(tyre_dmg[RL]), 2)}% RR {_rpad(_cd(tyre_dmg[RR]), 2)}%"))
        if any(v > 0 for v in brake_dmg):
            lines.append(b(f"  {_DIM}Brake Dm{_RESET} FL {_rpad(_cd(brake_dmg[FL]), 2)}% FR {_rpad(_cd(brake_dmg[FR]), 2)}% RL {_rpad(_cd(brake_dmg[RL]), 2)}% RR {_rpad(_cd(brake_dmg[RR]), 2)}%"))
        if any(b_val > 0 for b_val in blist):
            lines.append(b(f"  {_DIM}Blister{_RESET}  FL {_rpad(_cd(blist[FL]), 2)}% FR {_rpad(_cd(blist[FR]), 2)}% RL {_rpad(_cd(blist[RL]), 2)}% RR {_rpad(_cd(blist[RR]), 2)}%"))
        lines.append(f"{c}└{'─' * W}┘{_RESET}")
        return lines

    # ─── AERO & DAMAGE PANEL ───

    def _render_aero_damage_panel(self, damage, motion_ex, player_idx) -> list[str]:
        """Render aerodynamics damage data."""
        lines = []
        a = _BLUE
        W = _BOX_W
        lines.append(f"{_BOLD}{a}┌─ AERODYNAMICS {'─' * (W - 15)}┐{_RESET}")

        if not all([damage, player_idx is not None]):
            lines.append(_bx(a, f" {_DIM}No data{_RESET}"))
            lines.append(f"{a}└{'─' * W}┘{_RESET}")
            return lines

        d_cars = damage.get("m_carDamageData", [])
        d = d_cars[player_idx] if player_idx < len(d_cars) else {}

        fl_wing = d.get("m_frontLeftWingDamage", 0)
        fr_wing = d.get("m_frontRightWingDamage", 0)
        rw = d.get("m_rearWingDamage", 0)
        floor = d.get("m_floorDamage", 0)
        diff = d.get("m_diffuserDamage", 0)
        side = d.get("m_sidepodDamage", 0)
        drs_f = d.get("m_drsFault", 0)
        ers_f = d.get("m_ersFault", 0)
        eng_blown = d.get("m_engineBlown", 0)
        eng_seized = d.get("m_engineSeized", 0)

        f_ride = motion_ex.get("m_frontAeroHeight", 0) if motion_ex else 0
        r_ride = motion_ex.get("m_rearAeroHeight", 0) if motion_ex else 0

        drs_txt = f"{_RED}FAULT{_RESET}" if drs_f else f"{_GREEN}OK{_RESET}"
        ers_txt = f"{_RED}FAULT{_RESET}" if ers_f else f"{_GREEN}OK{_RESET}"

        b = lambda content: _bx(a, content)  # noqa: E731

        lines.append(b(f"  Front Wing   L {_rpad(_cd(fl_wing), 3)}%   R {_rpad(_cd(fr_wing), 3)}%"))
        lines.append(b(f"  Rear Wing    {_rpad(_cd(rw), 3)}%     Floor     {_rpad(_cd(floor), 3)}%"))
        lines.append(b(f"  Diffuser     {_rpad(_cd(diff), 3)}%     Sidepod   {_rpad(_cd(side), 3)}%"))
        lines.append(b(f"  DRS {drs_txt}   ERS {ers_txt}   Ride F{f_ride:.3f} R{r_ride:.3f}"))
        if eng_blown:
            lines.append(b(f"  {_RED}{_BOLD}ENGINE BLOWN{_RESET}"))
        if eng_seized:
            lines.append(b(f"  {_RED}{_BOLD}ENGINE SEIZED{_RESET}"))
        lines.append(f"{a}└{'─' * W}┘{_RESET}")
        return lines

    # ─── SETUP & DYNAMICS PANEL ───

    def _render_setup_panel(self, setups, motion_ex, status, player_idx) -> list[str]:
        """Render car setup and dynamics data."""
        lines = []
        w = _WHITE
        W = _BOX_W
        lines.append(f"{_BOLD}{w}┌─ SETUP & DYNAMICS {'─' * (W - 19)}┐{_RESET}")

        if player_idx is None:
            lines.append(_bx(w, f" {_DIM}No data{_RESET}"))
            lines.append(f"{w}└{'─' * W}┘{_RESET}")
            return lines

        su = {}
        if setups:
            su_cars = setups.get("m_carSetupData", [])
            su = su_cars[player_idx] if player_idx < len(su_cars) else {}
        s = {}
        if status:
            s_cars = status.get("m_carStatusData", [])
            s = s_cars[player_idx] if player_idx < len(s_cars) else {}

        b = lambda content: _bx(w, content)  # noqa: E731
        b_sep = f"{w}│{_RESET}  {'─' * (W - 4)}  {w}│{_RESET}"

        # Setup values
        fw = su.get("m_frontWing", 0)
        rew = su.get("m_rearWing", 0)
        f_camber = su.get("m_frontCamber", 0)
        r_camber = su.get("m_rearCamber", 0)
        f_toe = su.get("m_frontToe", 0)
        r_toe = su.get("m_rearToe", 0)
        f_susp = su.get("m_frontSuspension", 0)
        r_susp = su.get("m_rearSuspension", 0)
        f_arb = su.get("m_frontAntiRollBar", 0)
        r_arb = su.get("m_rearAntiRollBar", 0)
        f_sh = su.get("m_frontSuspensionHeight", 0)
        r_sh = su.get("m_rearSuspensionHeight", 0)
        brake_p = su.get("m_brakePressure", 0)
        eng_brake = su.get("m_engineBraking", 0)
        fuel_load = su.get("m_fuelLoad", 0)

        # Status extras
        tc = s.get("m_tractionControl", 0)
        abs_val = s.get("m_antiLockBrakes", 0)
        fia_flag = s.get("m_vehicleFIAFlags", 0)
        flag_names = {0: "", 1: f"{_GREEN}Green{_RESET}", 2: f"{_BLUE}Blue{_RESET}",
                      3: f"{_YELLOW}Yellow{_RESET}", 4: f"{_RED}Red{_RESET}"}
        flag_str = flag_names.get(fia_flag, "")

        lines.append(b(f"  {_DIM}Wings{_RESET}     Front {fw:>2}    Rear {rew:>2}"))
        lines.append(b(f"  {_DIM}Camber{_RESET}    Front {f_camber:>5.2f}  Rear {r_camber:>5.2f}"))
        lines.append(b(f"  {_DIM}Toe{_RESET}       Front {f_toe:>5.3f}  Rear {r_toe:>5.3f}"))
        lines.append(b(f"  {_DIM}Suspension{_RESET} Front {f_susp:>2}    Rear {r_susp:>2}"))
        lines.append(b(f"  {_DIM}AR Bar{_RESET}    Front {f_arb:>2}    Rear {r_arb:>2}"))
        lines.append(b(f"  {_DIM}Ride Ht{_RESET}   Front {f_sh:>2}    Rear {r_sh:>2}"))
        lines.append(b_sep)
        lines.append(b(f"  Brake Press {brake_p:>3}%  EngBrake {eng_brake:>3}%  Fuel {fuel_load:>.1f}kg"))
        lines.append(b(f"  TC {'ON' if tc else 'OFF':>3}  ABS {'ON' if abs_val else 'OFF':>3}  {flag_str}"))

        # MotionEx dynamics
        if motion_ex:
            b_sep_line = b_sep
            lines.append(b_sep_line)
            g_lat = 0
            g_long = 0
            if motion_ex:
                vel_x = motion_ex.get("m_localVelocityX", 0)
                vel_z = motion_ex.get("m_localVelocityZ", 0)
                speed_ms = math.sqrt(vel_x**2 + vel_z**2)

                susp_pos = motion_ex.get("m_suspensionPosition", [0]*4)
                wheel_speed = motion_ex.get("m_wheelSpeed", [0]*4)
                slip_ratio = motion_ex.get("m_wheelSlipRatio", [0]*4)
                cog = motion_ex.get("m_heightOfCOGAboveGround", 0)
                front_roll = motion_ex.get("m_frontRollAngle", 0)
                rear_roll = motion_ex.get("m_rearRollAngle", 0)

                lines.append(b(f"  {_DIM}Dynamics{_RESET}  CoG {cog:.3f}m  Speed {speed_ms:.1f}m/s"))
                lines.append(b(f"  {_DIM}Roll{_RESET}  F {front_roll:>+7.4f}  R {rear_roll:>+7.4f}"))

        lines.append(f"{w}└{'─' * W}┘{_RESET}")
        return lines

    # ─── POWER UNIT PANEL ───

    def _render_pu_panel(self, telem, status, damage, player_idx) -> list[str]:
        """Render the power unit data panel."""
        lines = []
        m = _MAGENTA
        W = _BOX_W
        lines.append(f"{_BOLD}{m}┌─ POWER UNIT {'─' * (W - 13)}┐{_RESET}")

        if not all([telem, player_idx is not None]):
            lines.append(_bx(m, f" {_DIM}No data{_RESET}"))
            lines.append(f"{m}└{'─' * W}┘{_RESET}")
            return lines

        t_cars = telem.get("m_carTelemetryData", [])
        t = t_cars[player_idx] if player_idx < len(t_cars) else {}
        s = {}
        if status:
            s_cars = status.get("m_carStatusData", [])
            s = s_cars[player_idx] if player_idx < len(s_cars) else {}
        d = {}
        if damage:
            d_cars = damage.get("m_carDamageData", [])
            d = d_cars[player_idx] if player_idx < len(d_cars) else {}

        speed = t.get("m_speed", 0)
        rpm = t.get("m_engineRPM", 0)
        gear = t.get("m_gear", 0)
        drs = t.get("m_drs", 0)
        throttle = t.get("m_throttle", 0)
        brake = t.get("m_brake", 0)
        steer = t.get("m_steer", 0)
        engine_temp = t.get("m_engineTemperature", 0)

        gear_str = f"{_BOLD}{_WHITE}N{_RESET}" if gear == 0 else (f"{_BOLD}{_RED}R{_RESET}" if gear < 0 else f"{_BOLD}{_WHITE}{gear}{_RESET}")
        drs_str = f"{_GREEN}{_BOLD}ON{_RESET}" if drs else f"{_DIM}OFF{_RESET}"

        fuel_kg = s.get("m_fuelInTank", 0)
        fuel_cap = s.get("m_fuelCapacity", 0)
        fuel_laps = s.get("m_fuelRemainingLaps", 0)
        fuel_mix = _FUEL_MIX.get(s.get("m_fuelMix", 1), "?")

        ice_w = s.get("m_enginePowerICE", 0)
        mguk_w = s.get("m_enginePowerMGUK", 0)
        ice_bhp = ice_w / 745.7 if ice_w else 0
        mguk_bhp = mguk_w / 745.7 if mguk_w else 0

        ers_j = s.get("m_ersStoreEnergy", 0)
        ers_pct = ers_j / 4_000_000 * 100 if ers_j else 0
        ers_mode = _ERS_MODE.get(s.get("m_ersDeployMode", 0), "?")
        ers_deployed = s.get("m_ersDeployedThisLap", 0)
        ers_harv_k = s.get("m_ersHarvestedThisLapMGUK", 0)
        ers_harv_h = s.get("m_ersHarvestedThisLapMGUH", 0)

        eng_dmg = d.get("m_engineDamage", 0)
        gb_dmg = d.get("m_gearBoxDamage", 0)
        ice_wear = d.get("m_engineICEWear", 0)
        mguh_wear = d.get("m_engineMGUHWear", 0)
        mguk_wear = d.get("m_engineMGUKWear", 0)
        es_wear = d.get("m_engineESWear", 0)
        ce_wear = d.get("m_engineCEWear", 0)
        tc_wear = d.get("m_engineTCWear", 0)

        fuel_col = _GREEN if fuel_laps >= 0 else _RED
        b = lambda content: _bx(m, content)  # noqa: E731
        b_sep = f"{m}│{_RESET}  {'─' * (W - 4)}  {m}│{_RESET}"

        # Steer bar: -1.0 (left) to +1.0 (right), centered
        steer_w = 14
        steer_mid = steer_w // 2
        steer_pos = int(steer * steer_mid)
        steer_bar_chars = [" "] * steer_w
        steer_bar_chars[steer_mid] = "│"
        if steer_pos != 0:
            start = min(steer_mid, steer_mid + steer_pos)
            end = max(steer_mid, steer_mid + steer_pos)
            for si in range(start, end + 1):
                if 0 <= si < steer_w:
                    steer_bar_chars[si] = "█"
        steer_bar = f"{_YELLOW}{''.join(steer_bar_chars)}{_RESET}"

        lines.append(b(f"  Speed   {_BOLD}{_WHITE}{speed:>5d}{_RESET} km/h  {_bar_h(speed / 350, 14)}"))
        lines.append(b(f"  RPM    {rpm:>6,}       Gear: {gear_str}   DRS: {drs_str}"))
        lines.append(b(f"  Throttle {_bar_color(throttle, _GREEN)}  {throttle*100:>4.0f}%"))
        lines.append(b(f"  Brake    {_bar_color(brake, _RED)}  {brake*100:>4.0f}%"))
        lines.append(b(f"  Steer    [{steer_bar}] {steer:>+5.2f}"))
        lines.append(b(f"  Engine   {_color_temp_eng(engine_temp)}{engine_temp:>4d}°C{_RESET}"))
        lines.append(b_sep)
        lines.append(b(f"  Fuel     {fuel_kg:>5.1f} kg / {fuel_cap:.0f}    Mix: {_BOLD}{fuel_mix}{_RESET}"))
        lines.append(b(f"  Delta    {fuel_col}{fuel_laps:>+5.1f}{_RESET} laps"))
        lines.append(b_sep)
        lines.append(b(f"  ICE      {_BOLD}{ice_bhp:>5.0f}{_RESET} bhp"))
        lines.append(b(f"  MGU-K    {_BOLD}{mguk_bhp:>5.0f}{_RESET} bhp"))
        lines.append(b(f"  ERS      {_bar_color(ers_pct / 100, _CYAN)}  {ers_pct:>4.0f}% ({ers_j/1e6:.2f} MJ)"))
        lines.append(b(f"  Mode     {_BOLD}{ers_mode}{_RESET}"))
        lines.append(b(f"  Deploy   {ers_deployed/1e6:.2f} MJ  Harv: {(ers_harv_k+ers_harv_h)/1e6:.2f} MJ"))
        lines.append(b_sep)
        lines.append(b(f"  {_DIM}Wear{_RESET}  ICE {_cw_pu(ice_wear)}  TC {_cw_pu(tc_wear)}  CE {_cw_pu(ce_wear)}"))
        lines.append(b(f"        MGU-H {_cw_pu(mguh_wear)}  MGU-K {_cw_pu(mguk_wear)}  ES {_cw_pu(es_wear)}"))
        lines.append(b(f"  Gearbox: {_cd(gb_dmg)}%  Engine: {_cd(eng_dmg)}%"))
        lines.append(f"{m}└{'─' * W}┘{_RESET}")
        return lines

    # ─── EVENTS PANEL ───

    def _render_events_panel(self, panel_w) -> list[str]:
        """Render scrolling event log."""
        lines = []
        g = _YELLOW
        W = panel_w - 2
        lines.append(f"{_BOLD}{g}┌─ EVENTS {'─' * max(W - 10, 0)}┐{_RESET}")

        if not self._events:
            for _ in range(_MAX_EVENTS):
                lines.append(f"{g}│{_RESET}{' ' * W}{g}│{_RESET}")
        else:
            for i in range(_MAX_EVENTS):
                idx = len(self._events) - _MAX_EVENTS + i
                if 0 <= idx < len(self._events):
                    ev = self._events[idx]
                    vis = _visible_len(ev)
                    pad = W - vis - 1
                    lines.append(f"{g}│{_RESET} {ev}{' ' * max(pad, 0)}{g}│{_RESET}")
                else:
                    lines.append(f"{g}│{_RESET}{' ' * W}{g}│{_RESET}")

        lines.append(f"{g}└{'─' * W}┘{_RESET}")
        return lines

    # ─── TRACK MAP ───

    def _render_track_map(self, sess, motion, lap, parts, player_idx, panel_w) -> list[str]:
        """Render ASCII track map with car positions interpolated by lap distance."""
        lines = []
        map_w = panel_w - 2  # inside border characters
        map_h = 16

        track_name = ""
        track_id = -1
        track_length = 0
        if sess:
            track_id = sess.get("m_trackId", -1)
            track_length = sess.get("m_trackLength", 0)
            from common.f1_structs.f1_constants import TRACK_NAMES
            track_name = TRACK_NAMES.get(track_id, f"Track {track_id}")

        # Count active cars
        n_cars = 0
        if lap:
            for car_lap in lap.get("m_lapData", []):
                rs = car_lap.get("m_resultStatus", 0)
                ds = car_lap.get("m_driverStatus", 0)
                if rs > 1 or ds >= 1:
                    n_cars += 1

        g = _GREEN
        cars_str = f"  {_BOLD}{g}{n_cars}{_RESET}{g} cars" if n_cars else ""
        header = f" TRACK MAP — {track_name}{cars_str} "
        header_vis = _visible_len(header)
        pad = map_w - header_vis - 1
        lines.append(f"{_BOLD}{g}┌─{header}{_RESET}{g}{'─' * max(pad, 0)}┐{_RESET}")

        # Load static track outline and build cumulative distances
        if track_id != self._loaded_track_id:
            self._loaded_track_id = track_id
            raw_pts = self._track_loader.load(track_id)
            if raw_pts:
                self._static_track = _build_static_track(raw_pts)
            else:
                self._static_track = None

        static = self._static_track

        if not static:
            for _ in range(map_h):
                lines.append(f"{g}│{_RESET}{' ' * map_w}{g}│{_RESET}")
            lines.append(f"{g}└{'─' * map_w}┘{_RESET}")
            return lines

        # Gather car positions via lap-distance interpolation
        cars_lap_data = lap.get("m_lapData", []) if lap else []
        cars_parts_data = parts.get("m_participants", []) if parts else []

        car_points: list[tuple[float, float, str, bool]] = []
        for i, car_lap in enumerate(cars_lap_data):
            rs = car_lap.get("m_resultStatus", 0)
            ds = car_lap.get("m_driverStatus", 0)
            if rs <= 1 and ds == 0:
                continue

            lap_dist = car_lap.get("m_lapDistance", 0)
            if track_length <= 0:
                continue
            norm = lap_dist / track_length
            pt = _point_at_norm(static, norm)

            pos = car_lap.get("m_carPosition", 0)
            is_player = (i == player_idx)
            if is_player:
                label = "YOU"
            elif i < len(cars_parts_data):
                name = cars_parts_data[i].get("m_name", "")
                label = name[:3].upper() if name else f"C{i}"
            else:
                label = f"C{i}"
            prefix = f"{pos} " if pos > 0 else ""
            car_points.append((pt[0], pt[1], f"{prefix}{label}", is_player))

        # Bounding box from track outline
        xs = [p[0] for p in static["points"]]
        zs = [p[1] for p in static["points"]]
        min_x, max_x = min(xs), max(xs)
        min_z, max_z = min(zs), max(zs)
        range_x = max_x - min_x or 1
        range_z = max_z - min_z or 1

        # Scale with character aspect ratio (~2.2:1)
        aspect = 2.2
        scale_x = (map_w - 8) / range_x / aspect
        scale_z = (map_h - 1) / range_z
        scale = min(scale_x, scale_z)

        def to_grid(x: float, z: float) -> tuple[int, int]:
            """Convert to grid coordinates."""
            gc = int((x - min_x) * scale * aspect) + 3
            gr = int((z - min_z) * scale)
            return min(max(gc, 0), map_w - 1), min(max(gr, 0), map_h - 1)

        # Initialize grid with spaces
        grid: list[list[str]] = [[" "] * map_w for _ in range(map_h)]

        # Draw track outline
        for px, pz in static["points"]:
            gc, gr = to_grid(px, pz)
            if 0 <= gr < map_h and 0 <= gc < map_w and grid[gr][gc] == " ":
                grid[gr][gc] = f"{_DIM}·{_RESET}"

        # Place car labels (non-player first, player on top)
        for wx, wz, label, is_player in sorted(car_points, key=lambda c: c[3]):
            gc, gr = to_grid(wx, wz)
            if 0 <= gr < map_h:
                color = f"{_RED}{_BOLD}" if is_player else _YELLOW
                vis_len = len(label)
                start = max(0, min(gc, map_w - vis_len - 1))
                for ci, ch in enumerate(label):
                    col = start + ci
                    if 0 <= col < map_w:
                        grid[gr][col] = f"{color}{ch}{_RESET}"

        for row in grid:
            lines.append(f"{g}│{_RESET}{''.join(row)}{g}│{_RESET}")
        lines.append(f"{g}└{'─' * map_w}┘{_RESET}")
        return lines


# ─── Static track helpers (mirrors useTrackMap.ts logic) ───

def _build_static_track(raw_points: list[tuple[float, float]]) -> dict:
    """Build cumulative arc-length data from raw (x, z) track coordinates."""
    # Negate Z to match the UI convention (top-down view)
    points = [(x, -z) for x, z in raw_points]
    distances = [0.0]
    total = 0.0
    for i in range(1, len(points)):
        dx = points[i][0] - points[i - 1][0]
        dz = points[i][1] - points[i - 1][1]
        total += math.sqrt(dx * dx + dz * dz)
        distances.append(total)
    return {"points": points, "distances": distances, "totalLength": total}


_StaticTrack = dict  # {"points": list[(x,z)], "distances": list[float], "totalLength": float}


def _point_at_norm(track: dict, norm: float) -> tuple[float, float]:
    """Interpolate a point on the track outline at a normalised position (0-1)."""
    n = ((norm % 1) + 1) % 1
    target = n * track["totalLength"]
    dists = track["distances"]
    pts = track["points"]

    # Binary search for the segment
    lo, hi = 0, len(dists) - 1
    while lo < hi - 1:
        mid = (lo + hi) >> 1
        if dists[mid] <= target:
            lo = mid
        else:
            hi = mid

    seg_len = dists[hi] - dists[lo]
    t = (target - dists[lo]) / seg_len if seg_len > 0 else 0
    x = pts[lo][0] + t * (pts[hi][0] - pts[lo][0])
    z = pts[lo][1] + t * (pts[hi][1] - pts[lo][1])
    return x, z


# ─── Formatting helpers ───

def _fmt_time(ms: int) -> str:
    """Format milliseconds as M:SS.mmm."""
    if ms == 0:
        return "--:--.---"
    minutes = ms // 60000
    seconds = (ms % 60000) / 1000
    return f"{minutes}:{seconds:06.3f}"


def _fmt_sector(ms: int) -> str:
    """Format sector time in milliseconds as SS.mmm."""
    if ms == 0:
        return " --.-"
    seconds = ms / 1000
    return f"{seconds:5.1f}"


def _visible_len(text: str) -> int:
    """Length of text excluding ANSI escape sequences."""
    return len(_ANSI_RE.sub("", text))


def _truncate(text: str, max_vis: int) -> str:
    """Truncate an ANSI string to max_vis visible characters."""
    vis = 0
    i = 0
    while i < len(text) and vis < max_vis:
        # Skip ANSI escape sequences
        if text[i] == "\033" and i + 1 < len(text) and text[i + 1] == "[":
            j = i + 2
            while j < len(text) and text[j] != "m":
                j += 1
            i = j + 1  # skip past the 'm'
            continue
        vis += 1
        i += 1
    return text[:i] + _RESET


def _rpad(text: str, width: int) -> str:
    """Right-pad an ANSI string to a visible width."""
    pad = width - _visible_len(text)
    return text + (" " * pad if pad > 0 else "")


def _lpad(text: str, width: int) -> str:
    """Left-pad an ANSI string to a visible width."""
    pad = width - _visible_len(text)
    return (" " * pad if pad > 0 else "") + text


def _pad(text: str, width: int) -> str:
    """Right-pad an ANSI string to a visible width."""
    return _rpad(text, width)


def _side_by_side(left: list[str], right: list[str], panel_w: int) -> list[str]:
    """Place two panels side by side, each padded to panel_w visible width."""
    max_h = max(len(left), len(right))
    left += [""] * (max_h - len(left))
    right += [""] * (max_h - len(right))
    result = []
    for l_line, r_line in zip(left, right):
        result.append(f" {_pad(l_line, panel_w)} {_pad(r_line, panel_w)}")
    return result


_BOX_W = 48  # Inner width of each panel box (between │ and │)


def _bx(color: str, content: str) -> str:
    """Render a box content line: │ content padded to _BOX_W │."""
    vis = _visible_len(content)
    pad = _BOX_W - vis
    return f"{color}│{_RESET}{content}{' ' * max(pad, 0)}{color}│{_RESET}"


def _bar_h(frac: float, width: int = 14) -> str:
    """Render a fraction (0..1) as a horizontal bar."""
    frac = max(0.0, min(1.0, frac))
    filled = int(frac * width)
    return f"{_DIM}[{_RESET}{'█' * filled}{'░' * (width - filled)}{_DIM}]{_RESET}"


def _bar_color(frac: float, color: str, width: int = 14) -> str:
    """Render a colored fraction bar."""
    frac = max(0.0, min(1.0, frac))
    filled = int(frac * width)
    return f"{_DIM}[{_RESET}{color}{'█' * filled}{_RESET}{'░' * (width - filled)}{_DIM}]{_RESET}"


def _ct(temp: int) -> str:
    """Color-code tyre surface temp (returns colored string, not padded)."""
    s = str(temp) + "°"
    if temp > 110:
        return f"{_RED}{s}{_RESET}"
    if temp > 100:
        return f"{_YELLOW}{s}{_RESET}"
    if temp < 70:
        return f"{_BLUE}{s}{_RESET}"
    return f"{_GREEN}{s}{_RESET}"


def _cb(temp: int) -> str:
    """Color-code brake temp."""
    s = str(temp) + "°"
    if temp > 900:
        return f"{_RED}{s}{_RESET}"
    if temp > 700:
        return f"{_YELLOW}{s}{_RESET}"
    return f"{_GREEN}{s}{_RESET}"


def _cw(wear) -> str:
    """Color-code tyre wear percentage."""
    w = wear if isinstance(wear, (int, float)) else 0
    s = f"{w:.1f}"
    if w > 60:
        return f"{_RED}{s}{_RESET}"
    if w > 30:
        return f"{_YELLOW}{s}{_RESET}"
    return f"{_GREEN}{s}{_RESET}"


def _cw_pu(wear: int) -> str:
    """Color-code PU component wear."""
    s = f"{wear}%"
    if wear > 60:
        return f"{_RED}{s}{_RESET}"
    if wear > 30:
        return f"{_YELLOW}{s}{_RESET}"
    return f"{_GREEN}{s}{_RESET}"


def _cd(dmg) -> str:
    """Color-code damage value."""
    d = dmg if isinstance(dmg, (int, float)) else 0
    s = str(d)
    if d > 30:
        return f"{_RED}{s}{_RESET}"
    if d > 0:
        return f"{_YELLOW}{s}{_RESET}"
    return f"{_GREEN}{s}{_RESET}"


def _color_temp_air(temp: int) -> str:
    """Color-code track temperature."""
    if temp > 45:
        return f"{_RED}{temp}"
    if temp > 35:
        return f"{_YELLOW}{temp}"
    return f"{_GREEN}{temp}"


def _color_temp_eng(temp: int) -> str:
    """Color-code engine temperature (returns color prefix, no value)."""
    if temp > 120:
        return _RED
    if temp > 110:
        return _YELLOW
    return _GREEN
