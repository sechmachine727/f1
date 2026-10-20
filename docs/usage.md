# Usage

The dashboard has six workspaces: Drive, Pit wall, History, Analysis, Setups and Settings. The top bar switches between them. Live telemetry feeds Drive and the live Pit wall; recorded sessions feed History and selected recorded views.

## Live and recorded views

The header states **Live** when the server is receiving packets, or **Recorded** with the session type when a catalogued session is open. A recorded view reads only that session's catalogue, even while another session is live. **Back to live** clears the selection.

The dashboard shows `Waiting for telemetry` until the first packet. That is expected with the game closed or between sessions.

## Drive

Drive is the at-a-glance view for when you are in the car. It shows the current race-engineer call, tyre surface and core temperatures with wear rings, fuel margin and tank bars, ERS state, speed, gear, lap time, gaps ahead and behind, car issues and damage, and the field map.

Graphs in Drive are small trend lines for a glance. Use the Pit wall for analysis.

## Pit wall

The live Pit wall lists the field. Click a row, or focus it and press Enter or Space, to open the selected-car comparison.

- Select up to two cars. A third pick replaces the earlier selection.
- The comparison shows position, gaps, speed, throttle, brake, gear, ERS, fuel, worst tyre wear, damage and per-corner wear and surface temperature.
- Deltas state which car is ahead, faster or using less. If either value is unavailable, the row states that instead of inventing a number.
- Driver names, markers and lines use team colours. Status values keep the blue, green, orange and red meanings: cold, normal, caution and critical.
- The telemetry chart below the table plots throttle, brake, gear and ERS over the current session.

The live comparison takes at most two cars. The table itself still shows every car.

When a recorded session is open, the Pit wall switches to the recorded workspace described next.

## Recorded Pit wall

Opening a session from History fills the recorded Pit wall. It adds a session summary, driver selection for up to four cars, and these views: Overview, Laps, Sectors, Speed, Inputs, Tyres, Fuel / ERS, Stints / runs and Corners.

The **Inputs** view is the within-lap evidence. Choose cars and an optional lap range, then read throttle, brake, gear and ERS over session time. The deployment band names the state:

- 2025: none, medium, hotlap, overtake.
- 2026: none, medium, hotlap, boost, plus a separate overtake strip with unavailable, available and active.

## Chart controls

Every analysis chart starts at 100 percent, which fits the complete selected range. The controls work with the mouse, touch and keyboard:

| Action | Mouse or touch | Keyboard |
|---|---|---|
| Zoom in | Wheel up or **+** | `+` or `=` |
| Zoom out | Wheel down or **−** | `-` |
| Pan | Drag the plot, drag the navigator strip, or use the scrollbar | Left and right arrows |
| Reset to the full range | **Reset** | `0` |
| Follow new live samples | **Follow latest** | `f` |

Panning away from the newest sample stops the live window from moving. **Follow latest** returns to the end and resumes tracking. Time-chart tooltips show the nearest sample and each selected car's values.

## History

History lists recorded sessions by season, with filters for race, qualifying and practice. Each row shows the circuit, date, duration, laps and best lap. Qualifying segments that share a weekend are groups of Q1, Q2 and Q3.

Selecting a session opens the recorded Pit wall. **Cleanup** deletes the session's recording files after confirmation; the catalog and derived analytics remain until the files are gone. Active recordings are refused.

## Analysis

Analysis works on a recorded session. Pick two to four laps, then **Compare selected laps** for sector deltas, theoretical best and condition differences. The time-aligned chart plots each lap from its own start, so laps of different length end where they end.

The same workspace can:

- export the selected session as a CSV ZIP,
- open a replay of the recording,
- ask the configured race engineer about the session. Answers cite the stored evidence.

Analysis does not run without a recorded session.

## Setups

Setups lists saved recommendations and the experiments recorded against them. Each entry shows the proposed parameter changes, handling effect, tradeoff, source count and experiment outcomes. Setup research is recommendation-only; the package never changes the game.

## Settings

The web Settings workspace is a read-only report of the values the running server resolved, including each value's source and whether a change needs a restart. Secrets show as configured or missing, never as text.

To change values, use the environment or `config.json` on Docker and native installs, or the control center's Settings page for the Windows portable build. See [Settings](settings.md).

## Recording states

`recording` means datagrams are being written. `stopped_user` means recording was stopped in the app. `stopped_limit` means the byte cap or free-disk reserve was reached. `stopped_error` means the writer failed and the log carries the reason. Live telemetry keeps flowing in the last two cases; only the recording stops.

