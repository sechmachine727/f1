import { useEffect, useRef, useState } from "react";
import type { Alert } from "@/components/AlertBox";

export interface RaceEngineerReportEntry {
  text: string;
  time: string;
}

export interface RaceEngineerReport {
  responses: RaceEngineerReportEntry[];
  alerts: Alert[];
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

export function useRaceEngineerReport(): RaceEngineerReport {
  const [report, setReport] = useState<RaceEngineerReport>({ responses: [], alerts: [] });
  const lastResponse = useRef<string | null>(null);
  const lastAlertCount = useRef<number>(0);
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
          if (msg.raceEngineerReport) {
            const re = msg.raceEngineerReport;
            const serverAlerts: Alert[] = re.alerts ?? [];

            if (re.response === null && serverAlerts.length === 0) {
              lastResponse.current = null;
              lastAlertCount.current = 0;
              setReport({ responses: [], alerts: [] });
            } else {
              setReport((prev) => {
                const newResponses = re.response !== null && re.response !== lastResponse.current
                  ? [...prev.responses, { text: re.response, time: re.time ?? "" }]
                  : prev.responses;

                if (re.response !== null) {
                  lastResponse.current = re.response;
                }

                const newAlerts = serverAlerts.length !== lastAlertCount.current
                  ? serverAlerts
                  : prev.alerts;
                lastAlertCount.current = serverAlerts.length;

                return { responses: newResponses, alerts: newAlerts };
              });
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

  return report;
}
