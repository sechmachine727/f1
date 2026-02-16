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

interface ConditionState {
  level: Level;
  value: number;
}

// Damage metrics re-alert every step when worsening (0-255 scale)
const DAMAGE_REFIRE_STEP = 25;

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
  // Maps "fl_temp" → current level + last alerted value
  const activeConditions = useRef<Map<string, ConditionState>>(new Map());
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
    const currentConditions = new Map<string, ConditionState>();
    const ts = formatSessionTime(data.sessionTime);

    for (const wn of WHEELS) {
      const t = data.tires[wn];
      const life = Math.round(100 - t.wear);

      // Surface temperature
      if (t.surfaceTemp > 108) {
        currentConditions.set(`${wn}_temp`, { level: "crit", value: t.surfaceTemp });
      } else if (t.surfaceTemp > 103) {
        currentConditions.set(`${wn}_temp`, { level: "warn", value: t.surfaceTemp });
      }

      // Wear (value = damage amount, higher = worse)
      if (life <= 10) {
        currentConditions.set(`${wn}_wear`, { level: "crit", value: t.wear });
      } else if (life <= 25) {
        currentConditions.set(`${wn}_wear`, { level: "warn", value: t.wear });
      }

      // Damage
      if (t.damage > 150) {
        currentConditions.set(`${wn}_dmg`, { level: "crit", value: t.damage });
      } else if (t.damage > 50) {
        currentConditions.set(`${wn}_dmg`, { level: "warn", value: t.damage });
      }

      // Blisters
      if (t.blisters > 150) {
        currentConditions.set(`${wn}_blst`, { level: "crit", value: t.blisters });
      } else if (t.blisters > 50) {
        currentConditions.set(`${wn}_blst`, { level: "warn", value: t.blisters });
      }
    }

    // Fire alerts for new, escalated, or worsened conditions
    const DAMAGE_METRICS = new Set(["dmg", "blst", "wear"]);

    for (const [key, cur] of currentConditions) {
      const prev = activeConditions.current.get(key);
      const underscoreIdx = key.indexOf("_");
      const wn = key.substring(0, underscoreIdx) as typeof WHEELS[number];
      const metric = key.substring(underscoreIdx + 1);
      const label = WHEEL_LABELS[wn];
      const t = data.tires[wn];
      const life = Math.round(100 - t.wear);
      const tag = metric.toUpperCase();

      const isNew = !prev;
      const isEscalation = prev && prev.level === "warn" && cur.level === "crit";
      const isWorsened = DAMAGE_METRICS.has(metric) && prev
        && prev.level === cur.level && cur.value >= prev.value + DAMAGE_REFIRE_STEP;

      if (isNew || isEscalation || isWorsened) {
        const alertLevel = cur.level === "crit" ? "critical" : "warning";

        if (metric === "temp") {
          newAlerts.push({
            level: alertLevel,
            message: cur.level === "crit"
              ? `${label} surface temp ${t.surfaceTemp}°C — overheating`
              : `${label} surface temp ${t.surfaceTemp}°C — approaching limit`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "wear") {
          newAlerts.push({
            level: alertLevel,
            message: cur.level === "crit"
              ? `${label} tyre life critically low at ${life}%`
              : `${label} tyre life low at ${life}%`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "dmg") {
          newAlerts.push({
            level: alertLevel,
            message: cur.level === "crit"
              ? `${label} tyre damage critical (${t.damage}/255)`
              : `${label} tyre damage detected (${t.damage}/255)`,
            time: `${ts} ${tag}`,
          });
        } else if (metric === "blst") {
          newAlerts.push({
            level: alertLevel,
            message: cur.level === "crit"
              ? `${label} severe blistering (${t.blisters}/255)`
              : `${label} blistering detected (${t.blisters}/255)`,
            time: `${ts} ${tag}`,
          });
        }
      } else if (prev) {
        // Keep the previous alerted value as baseline
        cur.value = prev.value;
      }
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
