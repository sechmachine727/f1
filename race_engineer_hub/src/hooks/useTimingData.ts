import { useEffect, useRef, useState } from "react";

export interface LapHistoryEntry {
  lapNum: number;
  lapTimeMs: number;
  s1Ms: number;
  s2Ms: number;
  s3Ms: number;
  valid: boolean;
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

export type SectorColor = "purple" | "green" | "yellow" | "white";

export function formatSectorTime(ms: number): string {
  if (ms <= 0) return "--.--.---";
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
  const flashTimer = useRef<ReturnType<typeof setTimeout>>();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>();

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
            setTiming(msg.timing);

            // Detect lap completion
            const newLap = msg.timing.currentLap;
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
