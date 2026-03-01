import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/components/AlertBox";

export interface PowerUnitData {
  rpm: number;
  engineTemp: number;
  gear: number;
  fuelInTank: number;
  fuelRemainingLaps: number;
  fuelMix: string;
  icePowerKW: number;
  mgukPowerKW: number;
  ersStoreEnergy: number;
  batteryPct: number;
  ersDeployMode: string;
  ersDeployedThisLap: number;
  ersHarvestedMGUK: number;
  ersHarvestedMGUH: number;
  engineDamage: number;
  gearboxDamage: number;
  sessionTime: number;
}

interface PuAlerts {
  alerts: Alert[];
  activeCount: number;
}

export interface PuReport {
  responses: string[];
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

const EMPTY_ALERTS: PuAlerts = { alerts: [], activeCount: 0 };

export function usePowerUnitTelemetry(): { data: PowerUnitData | null; puAlerts: PuAlerts; puReport: PuReport } {
  const [data, setData] = useState<PowerUnitData | null>(null);
  const [puAlerts, setPuAlerts] = useState<PuAlerts>(EMPTY_ALERTS);
  const [puReport, setPuReport] = useState<PuReport>({ responses: [] });
  const lastPuResponse = useRef<string | null>(null);
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
          if (msg.powerUnit) {
            setData({ ...msg.powerUnit, sessionTime: msg.sessionTime ?? 0 });
          }
          if (msg.puAlerts) {
            setPuAlerts(msg.puAlerts);
          }
          if (msg.puReport) {
            if (msg.puReport.response === null) {
              lastPuResponse.current = null;
              setPuReport({ responses: [] });
            } else if (msg.puReport.response !== lastPuResponse.current) {
              lastPuResponse.current = msg.puReport.response;
              setPuReport((prev) => ({
                responses: [...prev.responses, msg.puReport.response],
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

  return { data, puAlerts, puReport };
}
