import { useEffect, useRef, useState } from "react";
import { TRACK_OUTLINES } from "@/data/trackOutlines";

export interface CarPosition {
  x: number;
  z: number;
  position: number;
  lapDistance: number;
  active: boolean;
}

export interface TrackMapState {
  /** Track outline — static (from bundled data) or dynamically built. */
  trackOutline: Array<{ x: number; z: number }>;
  cars: CarPosition[];
  playerIndex: number;
  /** True when using static data OR when the dynamic outline has completed a full lap. */
  outlineComplete: boolean;
  trackName: string;
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;
/** Minimum squared distance (metres) between consecutive dynamic outline points. */
const MIN_DIST_SQ = 25; // 5 m

function distSq(a: { x: number; z: number }, b: { x: number; z: number }): number {
  const dx = a.x - b.x;
  const dz = a.z - b.z;
  return dx * dx + dz * dz;
}

/** Look up the static outline for a given track name, converting from [x,z][] to {x,z}[]. */
function getStaticOutline(trackName: string): Array<{ x: number; z: number }> | null {
  const coords = TRACK_OUTLINES[trackName];
  if (!coords) return null;
  return coords.map(([x, z]) => ({ x, z }));
}

export function useTrackMap(): TrackMapState | null {
  const [state, setState] = useState<TrackMapState | null>(null);

  // Dynamic outline refs (used only when no static data is available)
  const dynamicOutlineRef = useRef<Array<{ x: number; z: number }>>([]);
  const dynamicCompleteRef = useRef(false);
  const maxLapDistRef = useRef(0);

  // Cached static outline so we don't re-convert on every message
  const staticOutlineCacheRef = useRef<{ name: string; outline: Array<{ x: number; z: number }> } | null>(null);

  const prevSessionTimeRef = useRef(0);
  const prevTrackNameRef = useRef("");

  useEffect(() => {
    let unmounted = false;
    let ws: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout>;

    function connect() {
      if (unmounted) return;
      ws = new WebSocket(WS_URL);

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          const trackMap = msg.trackMap;
          const sessionTime: number = msg.sessionTime ?? 0;
          const trackName: string = msg.session?.trackName ?? "";

          // Session reset — clear dynamic outline
          if (sessionTime < prevSessionTimeRef.current) {
            dynamicOutlineRef.current = [];
            dynamicCompleteRef.current = false;
            maxLapDistRef.current = 0;
          }
          prevSessionTimeRef.current = sessionTime;

          // Track change — reset dynamic outline and invalidate static cache
          if (trackName !== prevTrackNameRef.current) {
            prevTrackNameRef.current = trackName;
            dynamicOutlineRef.current = [];
            dynamicCompleteRef.current = false;
            maxLapDistRef.current = 0;
            staticOutlineCacheRef.current = null;
          }

          // Resolve the outline: prefer static, fall back to dynamic
          let outline: Array<{ x: number; z: number }>;
          let outlineComplete: boolean;

          if (staticOutlineCacheRef.current?.name === trackName) {
            outline = staticOutlineCacheRef.current.outline;
            outlineComplete = true;
          } else {
            const staticOutline = getStaticOutline(trackName);
            if (staticOutline) {
              staticOutlineCacheRef.current = { name: trackName, outline: staticOutline };
              outline = staticOutline;
              outlineComplete = true;
            } else {
              // Dynamic: accumulate from player positions
              outline = dynamicOutlineRef.current;
              outlineComplete = dynamicCompleteRef.current;
            }
          }

          const playerIndex: number = trackMap?.playerIndex ?? 0;
          const cars: CarPosition[] = trackMap?.cars ?? [];
          const player = cars[playerIndex];

          // Build dynamic outline if no static data
          if (!staticOutlineCacheRef.current && player && !dynamicCompleteRef.current && (player.x !== 0 || player.z !== 0)) {
            const dynOutline = dynamicOutlineRef.current;
            const last = dynOutline.length > 0 ? dynOutline[dynOutline.length - 1] : null;
            if (!last || distSq(last, player) > MIN_DIST_SQ) {
              dynOutline.push({ x: player.x, z: player.z });
            }
            const ld = player.lapDistance ?? 0;
            if (maxLapDistRef.current > 500 && ld < maxLapDistRef.current - 500) {
              dynamicCompleteRef.current = true;
            }
            if (ld > maxLapDistRef.current) {
              maxLapDistRef.current = ld;
            }
          }

          setState({
            trackOutline: outline,
            cars,
            playerIndex,
            outlineComplete,
            trackName,
          });
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (!unmounted) {
          reconnectTimer = setTimeout(connect, RECONNECT_INTERVAL_MS);
        }
      };
      ws.onerror = () => ws?.close();
    }

    connect();

    return () => {
      unmounted = true;
      clearTimeout(reconnectTimer);
      ws?.close();
    };
  }, []);

  return state;
}
