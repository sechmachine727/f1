/**
 * Telemetry WebSocket endpoint.
 *
 * Uses the build-time VITE_WS_URL when set (|| so an empty build arg falls
 * back), otherwise derives the host from the page so the dashboard works from
 * localhost, a LAN address, or a tunnel without a rebuild.
 */
export const WS_URL: string =
  import.meta.env.VITE_WS_URL || `ws://${location.hostname}:8765`;
