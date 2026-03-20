import { useEffect, useRef, useState } from "react";
import { getCircuitInfo } from "@/data/circuitData";
import { TRACK_OUTLINES } from "@/data/trackOutlines";
import type { CircuitInfo, MarshalZone, SectorBoundaries, PlayerDrs } from "@/types/circuitInfo";
import { buildStaticTrack, staticPointAtNorm, trackHeadingAtNorm } from "@/utils/trackGeometry";
import type { StaticTrackData } from "@/utils/trackGeometry";

export interface CarPosition {
  x: number;
  z: number;
  /** Heading angle in radians (direction of travel along the track). */
  heading: number;
  position: number;
  lapDistance: number;
  active: boolean;
  /** 3-letter driver abbreviation (e.g. "VER", "HAM"). Empty if unavailable. */
  abbreviation: string;
  /** Team abbreviation (e.g. "RBR", "MER"). Empty if unavailable. */
  teamAbbreviation: string;
}

export interface TrackMapState {
  trackOutline: Array<{ x: number; z: number }>;
  cars: CarPosition[];
  playerIndex: number;
  outlineComplete: boolean;
  trackName: string;
  trackLength: number;
  staticTrack: StaticTrackData | null;
  circuitInfo: CircuitInfo | null;
  marshalZones: MarshalZone[];
  sectorBoundaries: SectorBoundaries | null;
  playerDrs: PlayerDrs | null;
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

// ---------------------------------------------------------------------------
// Static outline helpers
// ---------------------------------------------------------------------------

/** Build static track from game-world coordinates (fallback). */
function loadStaticTrackFromGame(trackName: string): StaticTrackData | null {
  const coords = TRACK_OUTLINES[trackName];
  if (!coords) return null;
  const points = coords.map(([x, z]) => ({ x, z: -z }));
  return buildStaticTrack(points);
}

/** Build static track from API circuit info outline. */
function loadStaticTrackFromAPI(circuitInfo: CircuitInfo): StaticTrackData {
  // API uses x/y; we map y → z (negated to match SVG convention)
  const points = circuitInfo.outline.map(([x, y]) => ({ x, z: -y }));
  return buildStaticTrack(points);
}

/**
 * Map car positions onto the static track outline using normalised lap distance.
 * Falls back to raw world coordinates when static data is unavailable.
 */
function mapCarsToOutline(
  cars: CarPosition[],
  staticTrack: StaticTrackData | null,
  trackLength: number,
): CarPosition[] {
  if (!staticTrack || trackLength <= 0) return cars;

  return cars.map((car) => {
    if (!car.active) return car;
    const norm = car.lapDistance / trackLength;
    const pt = staticPointAtNorm(staticTrack, norm);
    const heading = trackHeadingAtNorm(staticTrack, norm);
    return { ...car, x: pt.x, z: pt.z, heading };
  });
}

export function useTrackMap(): TrackMapState | null {
  const [state, setState] = useState<TrackMapState | null>(null);

  // Dynamic outline built from the player's live positions (fallback when no static data)
  const dynamicOutlineRef = useRef<Array<{ x: number; z: number }>>([]);
  const dynamicCompleteRef = useRef(false);
  const maxLapDistRef = useRef(0);

  const staticTrackRef = useRef<StaticTrackData | null>(null);
  const circuitInfoRef = useRef<CircuitInfo | null>(null);

  const prevSessionTimeRef = useRef(0);
  const prevTrackIdRef = useRef<number>(-1);

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
          const trackId: number = msg.session?.trackId ?? -1;
          const trackName: string = msg.session?.trackName ?? "";
          const trackLength: number = msg.session?.trackLength ?? 0;

          // Session reset — clear dynamic outline
          if (sessionTime < prevSessionTimeRef.current) {
            dynamicOutlineRef.current = [];
            dynamicCompleteRef.current = false;
            maxLapDistRef.current = 0;
          }
          prevSessionTimeRef.current = sessionTime;

          // Track change — reload static data, reset dynamic outline
          if (trackId !== prevTrackIdRef.current) {
            prevTrackIdRef.current = trackId;
            dynamicOutlineRef.current = [];
            dynamicCompleteRef.current = false;
            maxLapDistRef.current = 0;

            // Look up static circuit info locally
            const circuitInfo = getCircuitInfo(trackId);
            circuitInfoRef.current = circuitInfo;

            // Prefer API outline, fall back to game-world outline
            if (circuitInfo?.outline?.length) {
              staticTrackRef.current = loadStaticTrackFromAPI(circuitInfo);
            } else {
              staticTrackRef.current = trackName ? loadStaticTrackFromGame(trackName) : null;
            }
          }

          const playerIndex: number = trackMap?.playerIndex ?? 0;
          const rawCars: CarPosition[] = trackMap?.cars ?? [];
          const player = rawCars[playerIndex];

          // Accumulate dynamic outline from player position (fallback for unknown tracks)
          if (player && !dynamicCompleteRef.current && (player.x !== 0 || player.z !== 0)) {
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

          const staticTrack = staticTrackRef.current;

          // Choose outline: prefer static, fall back to dynamic
          let outline: Array<{ x: number; z: number }>;
          let outlineComplete: boolean;

          if (staticTrack) {
            outline = staticTrack.points;
            outlineComplete = true;
          } else if (dynamicCompleteRef.current || dynamicOutlineRef.current.length >= 20) {
            outline = dynamicOutlineRef.current;
            outlineComplete = dynamicCompleteRef.current;
          } else {
            outline = dynamicOutlineRef.current;
            outlineComplete = false;
          }

          // Map car positions onto the outline via normalised lap distance
          const cars = mapCarsToOutline(rawCars, staticTrack, trackLength);

          setState({
            trackOutline: outline,
            cars,
            playerIndex,
            outlineComplete,
            trackName,
            trackLength,
            staticTrack,
            circuitInfo: circuitInfoRef.current,
            marshalZones: msg.marshalZones ?? [],
            sectorBoundaries: msg.sectorBoundaries ?? null,
            playerDrs: msg.playerDrs ?? null,
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
