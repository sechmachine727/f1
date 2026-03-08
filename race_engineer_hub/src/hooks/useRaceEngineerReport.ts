import { useEffect, useRef, useState } from "react";

export interface RaceEngineerReportEntry {
  text: string;
  time: string;
}

export interface RaceEngineerReport {
  responses: RaceEngineerReportEntry[];
}

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

export function useRaceEngineerReport(): RaceEngineerReport {
  const [report, setReport] = useState<RaceEngineerReport>({ responses: [] });
  const lastResponse = useRef<string | null>(null);
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
            if (msg.raceEngineerReport.response === null) {
              lastResponse.current = null;
              setReport({ responses: [] });
            } else if (msg.raceEngineerReport.response !== lastResponse.current) {
              lastResponse.current = msg.raceEngineerReport.response;
              setReport((prev) => ({
                responses: [...prev.responses, { text: msg.raceEngineerReport.response, time: msg.raceEngineerReport.time ?? "" }],
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

  return report;
}
