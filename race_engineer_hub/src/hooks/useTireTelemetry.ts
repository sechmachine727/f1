import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/components/AlertBox";

export interface TireData {
  surfaceTemp: number;
  innerTemp: number;
  pressure: number;
  wear: number;
  damage: number;
  blisters: number;
  brakeTemp: number;
}

export interface TireTelemetryData {
  tires: Record<"fl" | "fr" | "rl" | "rr", TireData>;
  compound: string;
  compoundVisual: string;
  tyresAgeLaps: number;
  currentLap: number;
  speed: number;
  sessionTime: number;
}

interface TireAlerts {
  alerts: Alert[];
  activeCount: number;
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

const EMPTY_ALERTS: TireAlerts = { alerts: [], activeCount: 0 };

export function useTireTelemetry(): { data: TireTelemetryData | null; tireAlerts: TireAlerts } {
  const [data, setData] = useState<TireTelemetryData | null>(null);
  const [tireAlerts, setTireAlerts] = useState<TireAlerts>(EMPTY_ALERTS);
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
          if (msg.tireAlerts) {
            setTireAlerts(msg.tireAlerts);
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

  return { data, tireAlerts };
}
