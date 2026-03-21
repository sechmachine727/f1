import { useEffect, useRef, useState } from "react";

export type SectorColor = "purple" | "green" | "yellow" | "white";

export interface LapHistoryEntry {
  lapNum: number;
  lapTimeMs: number;
  s1Ms: number;
  s2Ms: number;
  s3Ms: number;
  valid: boolean;
  s1Color?: SectorColor;
  s2Color?: SectorColor;
  s3Color?: SectorColor;
  deltaMs?: number;
}

export interface BestTimes {
  lapMs: number;
  s1Ms: number;
  s2Ms: number;
  s3Ms: number;
}

export interface TimingData {
  currentLap: number;
  currentSector: number;
  currentLapTimeMs: number;
  currentLapInvalid: boolean;
  sector1Ms: number;
  sector2Ms: number;
  lastLapTimeMs: number;
  personalBest: BestTimes;
  overallBest: BestTimes;
  lapHistory: LapHistoryEntry[];
}

export interface StandingsEntry {
  position: number;
  abbreviation: string;
  teamId: number;
  teamAbbreviation: string;
  gapToLeaderMs: number;
  gapToFrontMs: number;
  currentLap: number;
  isPlayer: boolean;
  driverStatus: number;
  resultStatus: number;
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;
const LAP_FLASH_DURATION_MS = 2000;

export function formatSectorTime(ms: number): string {
  if (ms <= 0) return "\u2014";
  const totalSeconds = ms / 1000;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes > 0) {
    return `${minutes}:${seconds.toFixed(3).padStart(6, "0")}`;
  }
  return seconds.toFixed(3);
}

export function getSectorColor(
  timeMs: number,
  personalBestMs: number,
  overallBestMs: number,
): SectorColor {
  if (timeMs <= 0) return "white";
  if (overallBestMs > 0 && timeMs <= overallBestMs) return "purple";
  if (personalBestMs > 0 && timeMs <= personalBestMs) return "green";
  return "yellow";
}

export function useTimingData(): {
  timing: TimingData | null;
  standings: StandingsEntry[];
  lapCompleted: boolean;
} {
  const [timing, setTiming] = useState<TimingData | null>(null);
  const [standings, setStandings] = useState<StandingsEntry[]>([]);
  const [lapCompleted, setLapCompleted] = useState(false);
  const prevLapNum = useRef<number>(0);
  const prevSessionTime = useRef<number>(0);
  const flashTimer = useRef<ReturnType<typeof setTimeout>>();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();
  const colorCache = useRef<Map<number, { s1: SectorColor; s2: SectorColor; s3: SectorColor; deltaMs: number | undefined }>>(new Map());

  useEffect(() => {
    let unmounted = false;

    function connect() {
      if (unmounted) return;

      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.timing) {
            const t = msg.timing as TimingData;
            const { personalBest: pb, overallBest: ob } = t;

            // Detect session reset and clear color cache
            if (msg.sessionTime !== undefined && msg.sessionTime < prevSessionTime.current) {
              colorCache.current.clear();
            }
            if (msg.sessionTime !== undefined) {
              prevSessionTime.current = msg.sessionTime;
            }

            // Snapshot sector colors for completed laps (only once per lap)
            for (const lap of t.lapHistory) {
              if (!colorCache.current.has(lap.lapNum)) {
                colorCache.current.set(lap.lapNum, {
                  s1: getSectorColor(lap.s1Ms, pb.s1Ms, ob.s1Ms),
                  s2: getSectorColor(lap.s2Ms, pb.s2Ms, ob.s2Ms),
                  s3: getSectorColor(lap.s3Ms, pb.s3Ms, ob.s3Ms),
                  deltaMs: pb.lapMs > 0 && lap.lapTimeMs > 0 ? lap.lapTimeMs - pb.lapMs : undefined,
                });
              }
            }

            // Attach cached colors to history entries
            const coloredHistory = t.lapHistory.map((lap) => {
              const cached = colorCache.current.get(lap.lapNum);
              return cached ? { ...lap, s1Color: cached.s1, s2Color: cached.s2, s3Color: cached.s3, deltaMs: cached.deltaMs } : lap;
            });

            setTiming({ ...t, lapHistory: coloredHistory });

            // Detect lap completion
            const newLap = t.currentLap;
            if (prevLapNum.current > 0 && newLap > prevLapNum.current) {
              setLapCompleted(true);
              clearTimeout(flashTimer.current);
              flashTimer.current = setTimeout(() => {
                if (!unmounted) setLapCompleted(false);
              }, LAP_FLASH_DURATION_MS);
            }
            prevLapNum.current = newLap;
          }
          if (msg.standings) {
            setStandings(msg.standings);
          }
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (!unmounted) {
          reconnectTimer.current = setTimeout(connect, RECONNECT_INTERVAL_MS);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    }

    connect();

    return () => {
      unmounted = true;
      clearTimeout(reconnectTimer.current);
      clearTimeout(flashTimer.current);
      wsRef.current?.close();
    };
  }, []);

  return { timing, standings, lapCompleted };
}
