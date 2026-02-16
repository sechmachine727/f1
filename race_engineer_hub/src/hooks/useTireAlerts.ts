import { useEffect, useRef, useState } from "react";
import type { TireTelemetryData } from "./useTireTelemetry";
import type { Alert } from "@/components/AlertBox";

const WHEEL_LABELS: Record<string, string> = {
  fl: "FL", fr: "FR", rl: "RL", rr: "RR",
};

const WHEELS = ["fl", "fr", "rl", "rr"] as const;

function formatSessionTime(seconds: number): string {
  const totalSec = Math.floor(seconds);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
}

type Level = "warn" | "crit";

interface Condition {
  level: Level;
  tag: string;
  warnMsg: string;
  critMsg: string;
  clearMsg: string;
}

const CLEAR_LABELS: Record<string, string> = {
  temp: "temp back to normal",
  wear: "tyre wear stabilised",
  dmg: "tyre damage stabilised",
  blst: "blistering subsided",
};

/**
 * Accumulates tire alerts over time. Uses one key per wheel+metric
 * (e.g. "fl_temp") so "back to normal" only fires when the condition
 * fully clears, not on de-escalation from crit to warn.
 */
export function useTireAlerts(data: TireTelemetryData | null): { alerts: Alert[]; activeCount: number } {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [activeCount, setActiveCount] = useState(0);
  // Maps "fl_temp" → current level
  const activeConditions = useRef<Map<string, Level>>(new Map());
  const prevCompound = useRef<string>("");
  const prevSessionTime = useRef<number>(0);

  useEffect(() => {
    if (!data) return;

    // Detect new session (session time resets)
    if (data.sessionTime < prevSessionTime.current) {
      setAlerts([]);
      setActiveCount(0);
      activeConditions.current = new Map();
      prevCompound.current = "";
    }
    prevSessionTime.current = data.sessionTime;

    const newAlerts: Alert[] = [];
    const currentConditions = new Map<string, Level>();
    const ts = formatSessionTime(data.sessionTime);

    for (const wn of WHEELS) {
      const t = data.tires[wn];
      const label = WHEEL_LABELS[wn];
      const life = Math.round(100 - t.wear);

      // Surface temperature
      if (t.surfaceTemp > 108) {
        currentConditions.set(`${wn}_temp`, "crit");
      } else if (t.surfaceTemp > 103) {
        currentConditions.set(`${wn}_temp`, "warn");
      }

      // Wear
      if (life <= 10) {
        currentConditions.set(`${wn}_wear`, "crit");
      } else if (life <= 25) {
        currentConditions.set(`${wn}_wear`, "warn");
      }

      // Damage
      if (t.damage > 150) {
        currentConditions.set(`${wn}_dmg`, "crit");
      } else if (t.damage > 50) {
        currentConditions.set(`${wn}_dmg`, "warn");
      }

      // Blisters
      if (t.blisters > 150) {
        currentConditions.set(`${wn}_blst`, "crit");
      } else if (t.blisters > 50) {
        currentConditions.set(`${wn}_blst`, "warn");
      }
    }

    // Fire alerts for new or escalated conditions
    for (const [key, level] of currentConditions) {
      const prev = activeConditions.current.get(key);
      const underscoreIdx = key.indexOf("_");
      const wn = key.substring(0, underscoreIdx) as typeof WHEELS[number];
      const metric = key.substring(underscoreIdx + 1);
      const label = WHEEL_LABELS[wn];
      const t = data.tires[wn];
      const life = Math.round(100 - t.wear);
      const tag = metric.toUpperCase();

      // New condition or escalation from warn to crit
      if (!prev || (prev === "warn" && level === "crit")) {
        if (metric === "temp") {
          newAlerts.push({
            level: level === "crit" ? "critical" : "warning",
            message: level === "crit"
              ? `${label} surface temp ${t.surfaceTemp}°C — overheating`
              : `${label} surface temp ${t.surfaceTemp}°C — approaching limit`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "wear") {
          newAlerts.push({
            level: level === "crit" ? "critical" : "warning",
            message: level === "crit"
              ? `${label} tyre life critically low at ${life}%`
              : `${label} tyre life low at ${life}%`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "dmg") {
          newAlerts.push({
            level: level === "crit" ? "critical" : "warning",
            message: level === "crit"
              ? `${label} tyre damage critical (${t.damage}/255)`
              : `${label} tyre damage detected (${t.damage}/255)`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "blst") {
          newAlerts.push({
            level: level === "crit" ? "critical" : "warning",
            message: level === "crit"
              ? `${label} severe blistering (${t.blisters}/255)`
              : `${label} blistering detected (${t.blisters}/255)`,
            time: `${ts} ${tag}`,
          });
        }
      }
      // De-escalation (crit → warn): no alert
    }

    // Detect fully cleared conditions
    for (const [key] of activeConditions.current) {
      if (!currentConditions.has(key)) {
        const underscoreIdx = key.indexOf("_");
        const wn = key.substring(0, underscoreIdx);
        const metric = key.substring(underscoreIdx + 1);
        const label = WHEEL_LABELS[wn] ?? wn.toUpperCase();
        const tag = metric.toUpperCase();
        const clearMsg = CLEAR_LABELS[metric];
        if (clearMsg) {
          newAlerts.push({ level: "info", message: `${label} ${clearMsg}`, time: `${ts} ${tag}` });
        }
      }
    }

    // Compound change
    const compoundKey = `${data.compound}_${data.compoundVisual}`;
    if (data.compound && compoundKey !== prevCompound.current) {
      prevCompound.current = compoundKey;
      newAlerts.push({
        level: "info",
        message: `${data.compoundVisual.toUpperCase()} (${data.compound}) fitted`,
        time: `${ts} TYRE`,
      });
    }

    activeConditions.current = currentConditions;
    setActiveCount(currentConditions.size);

    if (newAlerts.length > 0) {
      setAlerts((prev) => [...prev, ...newAlerts]);
    }
  }, [data]);

  return { alerts, activeCount };
}
