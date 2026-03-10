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
  speed: number;
  sessionTime: number;
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

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

const EMPTY_ALERTS: TyreAlerts = { alerts: [], activeCount: 0 };

export function useTyreTelemetry(): { data: TyreTelemetryData | null; tyreAlerts: TyreAlerts; tyresReport: TyresReport } {
  const [data, setData] = useState<TyreTelemetryData | null>(null);
  const [tyreAlerts, setTyreAlerts] = useState<TyreAlerts>(EMPTY_ALERTS);
  const [tyresReport, setTyresReport] = useState<TyresReport>({ responses: [] });
  const lastTyresResponse = useRef<string | null>(null);
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
          setData(msg);
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

  return { data, tyreAlerts, tyresReport };
}
