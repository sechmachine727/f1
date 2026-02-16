import { useEffect, useRef, useState } from "react";
import type { PowerUnitData } from "./usePowerUnitTelemetry";
import type { Alert } from "@/components/AlertBox";

function formatSessionTime(seconds: number): string {
  const totalSec = Math.floor(seconds);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
}

type Level = "warn" | "crit";

const ALERT_DEFS: Record<string, { tag: string; clearMsg: string }> = {
  eng_temp: { tag: "TEMP", clearMsg: "engine temp back to normal" },
  eng_dmg: { tag: "ICE", clearMsg: "engine damage stabilised" },
  gbx_dmg: { tag: "GBX", clearMsg: "gearbox damage stabilised" },
  fuel: { tag: "FUEL", clearMsg: "fuel delta recovered" },
  battery: { tag: "ERS", clearMsg: "battery SOC recovered" },
};

/**
 * Accumulates power-unit alerts over time. Same edge-detection
 * approach as useTireAlerts — one key per metric, "back to normal"
 * only fires when the condition fully clears.
 */
export function usePowerUnitAlerts(data: PowerUnitData | null): { alerts: Alert[]; activeCount: number } {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [activeCount, setActiveCount] = useState(0);
  const activeConditions = useRef<Map<string, Level>>(new Map());
  const prevSessionTime = useRef<number>(0);
  const prevErsMode = useRef<string>("");
  const prevFuelMix = useRef<string>("");

  useEffect(() => {
    if (!data) return;

    // Detect new session (session time resets)
    if (data.sessionTime < prevSessionTime.current) {
      setAlerts([]);
      setActiveCount(0);
      activeConditions.current = new Map();
      prevErsMode.current = "";
      prevFuelMix.current = "";
    }
    prevSessionTime.current = data.sessionTime;

    const newAlerts: Alert[] = [];
    const currentConditions = new Map<string, Level>();
    const ts = formatSessionTime(data.sessionTime);

    // Engine temperature
    if (data.engineTemp > 130) {
      currentConditions.set("eng_temp", "crit");
    } else if (data.engineTemp > 120) {
      currentConditions.set("eng_temp", "warn");
    }

    // Engine damage (0-100)
    if (data.engineDamage > 20) {
      currentConditions.set("eng_dmg", "crit");
    } else if (data.engineDamage > 5) {
      currentConditions.set("eng_dmg", "warn");
    }

    // Gearbox damage (0-100)
    if (data.gearboxDamage > 20) {
      currentConditions.set("gbx_dmg", "crit");
    } else if (data.gearboxDamage > 5) {
      currentConditions.set("gbx_dmg", "warn");
    }

    // Fuel remaining laps
    if (data.fuelRemainingLaps < 1) {
      currentConditions.set("fuel", "crit");
    } else if (data.fuelRemainingLaps < 3) {
      currentConditions.set("fuel", "warn");
    }

    // Battery SOC
    if (data.batteryPct < 15) {
      currentConditions.set("battery", "crit");
    } else if (data.batteryPct < 30) {
      currentConditions.set("battery", "warn");
    }

    // Fire alerts for new or escalated conditions
    for (const [key, level] of currentConditions) {
      const prev = activeConditions.current.get(key);
      const def = ALERT_DEFS[key];
      if (!def) continue;

      if (!prev || (prev === "warn" && level === "crit")) {
        const alertLevel = level === "crit" ? "critical" : "warning";

        if (key === "eng_temp") {
          newAlerts.push({
            level: alertLevel,
            message: level === "crit"
              ? `Engine temp ${data.engineTemp}°C — overheating`
              : `Engine temp ${data.engineTemp}°C — running hot`,
            time: `${ts} ${def.tag}`,
          });
        } else if (key === "eng_dmg") {
          newAlerts.push({
            level: alertLevel,
            message: level === "crit"
              ? `Engine damage critical (${data.engineDamage}%)`
              : `Engine damage detected (${data.engineDamage}%)`,
            time: `${ts} ${def.tag}`,
          });
        } else if (key === "gbx_dmg") {
          newAlerts.push({
            level: alertLevel,
            message: level === "crit"
              ? `Gearbox damage critical (${data.gearboxDamage}%)`
              : `Gearbox damage detected (${data.gearboxDamage}%)`,
            time: `${ts} ${def.tag}`,
          });
        } else if (key === "fuel") {
          newAlerts.push({
            level: alertLevel,
            message: level === "crit"
              ? `Fuel critically low — ${data.fuelRemainingLaps.toFixed(1)} laps remaining`
              : `Fuel running low — ${data.fuelRemainingLaps.toFixed(1)} laps remaining`,
            time: `${ts} ${def.tag}`,
          });
        } else if (key === "battery") {
          newAlerts.push({
            level: alertLevel,
            message: level === "crit"
              ? `Battery SOC critically low at ${data.batteryPct}%`
              : `Battery SOC low at ${data.batteryPct}%`,
            time: `${ts} ${def.tag}`,
          });
        }
      }
    }

    // Detect fully cleared conditions
    for (const [key] of activeConditions.current) {
      if (!currentConditions.has(key)) {
        const def = ALERT_DEFS[key];
        if (def) {
          newAlerts.push({ level: "info", message: def.clearMsg, time: `${ts} ${def.tag}` });
        }
      }
    }

    // ERS deploy mode change
    if (data.ersDeployMode && data.ersDeployMode !== prevErsMode.current) {
      if (prevErsMode.current) {
        newAlerts.push({
          level: "info",
          message: `ERS mode → ${data.ersDeployMode.toUpperCase()}`,
          time: `${ts} ERS`,
        });
      }
      prevErsMode.current = data.ersDeployMode;
    }

    // Fuel mix change
    if (data.fuelMix && data.fuelMix !== prevFuelMix.current) {
      if (prevFuelMix.current) {
        newAlerts.push({
          level: "info",
          message: `Fuel mix → ${data.fuelMix.toUpperCase()}`,
          time: `${ts} FUEL`,
        });
      }
      prevFuelMix.current = data.fuelMix;
    }

    activeConditions.current = currentConditions;
    setActiveCount(currentConditions.size);

    if (newAlerts.length > 0) {
      setAlerts((prev) => [...prev, ...newAlerts]);
    }
  }, [data]);

  return { alerts, activeCount };
}
