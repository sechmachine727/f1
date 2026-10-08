import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/components/AlertBox";
import { WS_URL } from "@/lib/wsUrl";

export interface AeroData {
  speed: number;
  drs: boolean;
  drsAllowed: boolean;
  drsActivationDistance: number;
  frontWing: number;
  rearWing: number;
  frontRideHeight: number;
  rearRideHeight: number;
  brakeBias: number;
  frontLeftWingDamage: number;
  frontRightWingDamage: number;
  rearWingDamage: number;
  floorDamage: number;
  diffuserDamage: number;
  sidepodDamage: number;
  drsFault: boolean;
  ersFault: boolean;
  brakeTempFL: number;
  brakeTempFR: number;
  brakeTempRL: number;
  brakeTempRR: number;
  sessionTime: number;
}

interface AeroAlerts {
  alerts: Alert[];
  activeCount: number;
}

export interface DamageReportEntry {
  text: string;
  time: string;
}

export interface DamageReport {
  responses: DamageReportEntry[];
}

const RECONNECT_INTERVAL_MS = 2000;

const EMPTY_ALERTS: AeroAlerts = { alerts: [], activeCount: 0 };

export function useAeroTelemetry(): { data: AeroData | null; aeroAlerts: AeroAlerts; damageReport: DamageReport } {
  const [data, setData] = useState<AeroData | null>(null);
  const [aeroAlerts, setAeroAlerts] = useState<AeroAlerts>(EMPTY_ALERTS);
  const [damageReport, setDamageReport] = useState<DamageReport>({ responses: [] });
  const lastDamageResponse = useRef<string | null>(null);
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
          if (msg.aero) {
            setData({ ...msg.aero, sessionTime: msg.sessionTime ?? 0 });
          }
          if (msg.aeroAlerts) {
            setAeroAlerts(msg.aeroAlerts);
          }
          if (msg.damageReport) {
            if (msg.damageReport.response === null) {
              // Session reset — clear accumulated responses
              lastDamageResponse.current = null;
              setDamageReport({ responses: [] });
            } else if (msg.damageReport.response !== lastDamageResponse.current) {
              lastDamageResponse.current = msg.damageReport.response;
              setDamageReport((prev) => ({
                responses: [...prev.responses, { text: msg.damageReport.response, time: msg.damageReport.time ?? "" }],
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

  return { data, aeroAlerts, damageReport };
}
