# Approved UI reference

[Open the interactive reference](approved-workspaces.html). It contains the approved Drive and Pit wall compositions with static illustrative data. Open it directly in a browser; no server or build is required. Toggle the two workspace controls at the top.

- [Drive at 1920x1080](drive-1920x1080.png)
- [Drive at 2560x1440](drive-2560x1440.png)
- [Pit wall at 1920x1080](pit-wall-1920x1080.png)
- [Editable layout map](workspace-layout.drawio)

## What implementation must preserve

Drive prioritizes the current decision, four circular tyre-wear gauges, very large surface/core values, medium energy/speed figures, nearby cars and compact graphs/track context. Temperatures remain aligned in four stable tyre positions. Keep the display dense enough for sub-second glances without card nesting or decorative empty areas. Match the reference hierarchy and relative sizes, not its HTML implementation.

At 1920x1080/2560x1440, 100% text scale, Drive fits one landscape screen without scroll. Use at least 56 px surface/core numeric text, 28 px key secondary figures, 18 px important labels at 1080p; scale up proportionally at 1440p. Use tabular numerals. The current call region has fixed height and a two-line limit; overflow details stay in the engineer panel.

Pit wall uses a comparison table, selected-driver detail, stint strip, pace/gap plots and pit alternatives. Drivers/Teams are two views of the same available evidence. Team aggregations state contributing cars and missing values. Historical versions preserve composition and show the selected season/session plus Recorded or Replay continuously.

## Semantics and states

Blue: cold or recovering. Green: within an applicable reference range/on plan. Orange: caution or rapid draw. Red: critical or projected shortfall. Neutral: unknown/restricted/stale/unsupported. Color always accompanies words or symbols. A tyre's wear-ring color is independent of surface/core temperature. Energy draw while boosting does not by itself imply a fault. Compound colors retain their game meaning and do not replace semantic status colors.

The reference deliberately demonstrates all four colors. Its temperatures, optimum labels, capacity percentages, race call and corner speed are illustrative, not calibrated measurements or game claims. Production replaces them with contract availability and source labels. Show MJ if capacity is unknown; show corner average as unavailable without validated entry/exit zones. The fallback profile has fewer classified bands than the demonstration.

Implement waiting for telemetry, connected but partial, live, recording stopped, stale, replay paused, loading history, empty history, range error, AI disabled/error, and restricted opponent states. Never populate meaningful zero values while waiting. WCAG 2.2 AA, visible focus, keyboard actions, reduced motion and 200% text zoom are required. At zoom/narrow widths, offer detailed reflow rather than hiding data or shrinking below readability.

Reference capture provenance: preserved from the user-approved in-memory brainstorming preview on 2026-10-18. Screenshots document this mockup only; they do not validate the future application.

The companion uses a bounded preview canvas and explanatory margin. Production Drive must use the supported monitor efficiently and scale its hierarchy at1440p as specified; do not copy the companion page heading/margins into the app. The Pit wall reference can scroll. Both workspace toggle controls were exercised after saving the standalone HTML.
