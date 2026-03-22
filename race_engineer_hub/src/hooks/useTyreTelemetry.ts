import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/components/AlertBox";

export interface TyreData {
  surfaceTemp: number;
  innerTemp: number;
  pressure: number;
  wear: number;
  damage: number;
  blisters: number;
  brakeTemp: number;
}

export interface TyreTelemetryData {
  tyres: Record<"fl" | "fr" | "rl" | "rr", TyreData>;
  compound: string;
  compoundVisual: string;
  tyresAgeLaps: number;
  currentLap: number;
  lapDistance: number;
  speed: number;
  sessionTime: number;
  trackLength: number;
  sectorBoundaries: { sector2Start: number; sector3Start: number };
}

interface TyreAlerts {
  alerts: Alert[];
  activeCount: number;
}

export interface TyresReportEntry {
  text: string;
  time: string;
}

export interface TyresReport {
  responses: TyresReportEntry[];
}

export interface LapWearRecord {
  lap: number;
  fl: number;
  fr: number;
  rl: number;
  rr: number;
  compound: string;
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

const EMPTY_ALERTS: TyreAlerts = { alerts: [], activeCount: 0 };

export function useTyreTelemetry(): {
  data: TyreTelemetryData | null;
  tyreAlerts: TyreAlerts;
  tyresReport: TyresReport;
  wearHistory: LapWearRecord[];
} {
  const [data, setData] = useState<TyreTelemetryData | null>(null);
  const [tyreAlerts, setTyreAlerts] = useState<TyreAlerts>(EMPTY_ALERTS);
  const [tyresReport, setTyresReport] = useState<TyresReport>({ responses: [] });
  const [wearHistory, setWearHistory] = useState<LapWearRecord[]>([]);
  const lastTyresResponse = useRef<string | null>(null);
  const lastRecordedFractionalLap = useRef<number>(0);
  const prevPitStatus = useRef<number>(0);
  const prevSessionTime = useRef<number>(0);
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
          setData({
            ...msg,
            lapDistance: msg.lapDistance ?? 0,
            trackLength: msg.session?.trackLength ?? 0,
            sectorBoundaries: msg.sectorBoundaries ?? { sector2Start: 0, sector3Start: 0 },
          });

          // Session reset detection
          if (msg.sessionTime !== undefined && msg.sessionTime < prevSessionTime.current - 5) {
            setWearHistory([]);
            lastRecordedFractionalLap.current = 0;
          }
          if (msg.sessionTime !== undefined) {
            prevSessionTime.current = msg.sessionTime;
          }

          // Detect pit exit: reset so recording resumes immediately
          const pitStatus = msg.pitStatus?.pitStatus ?? 0;
          if (prevPitStatus.current !== 0 && pitStatus === 0) {
            lastRecordedFractionalLap.current = 0;
          }
          prevPitStatus.current = pitStatus;

          // Record wear continuously using fractional laps (skip when in pit lane)
          const trackLength = msg.session?.trackLength ?? 0;
          if (msg.currentLap >= 1 && msg.tyres && trackLength > 0 && pitStatus === 0) {
            const lapDist = msg.lapDistance ?? 0;
            const fraction = Math.max(0, Math.min(lapDist / trackLength, 1));
            const fractionalLap = msg.currentLap + fraction;

            // Throttle: record every ~5% of a lap, only moving forward
            const step = 0.05;
            if (fractionalLap >= lastRecordedFractionalLap.current + step) {
              lastRecordedFractionalLap.current = fractionalLap;
              setWearHistory((prev) => [
                ...prev,
                {
                  lap: Math.round(fractionalLap * 100) / 100,
                  fl: msg.tyres.fl.wear,
                  fr: msg.tyres.fr.wear,
                  rl: msg.tyres.rl.wear,
                  rr: msg.tyres.rr.wear,
                  compound: msg.compoundVisual ?? "",
                },
              ]);
            }
          }

          if (msg.tyreAlerts) {
            setTyreAlerts(msg.tyreAlerts);
          }
          if (msg.tyresReport) {
            if (msg.tyresReport.response === null) {
              lastTyresResponse.current = null;
              setTyresReport({ responses: [] });
            } else if (msg.tyresReport.response !== lastTyresResponse.current) {
              lastTyresResponse.current = msg.tyresReport.response;
              setTyresReport((prev) => ({
                responses: [...prev.responses, { text: msg.tyresReport.response, time: msg.tyresReport.time ?? "" }],
              }));
            }
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
      wsRef.current?.close();
    };
  }, []);

  return { data, tyreAlerts, tyresReport, wearHistory };
}
