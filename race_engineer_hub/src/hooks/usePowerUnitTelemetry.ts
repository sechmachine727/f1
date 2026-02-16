import { useEffect, useRef, useState } from "react";

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

const WS_URL = "ws://localhost:8765";
const RECONNECT_INTERVAL_MS = 2000;

export function usePowerUnitTelemetry(): PowerUnitData | null {
  const [data, setData] = useState<PowerUnitData | null>(null);
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

  return data;
}
