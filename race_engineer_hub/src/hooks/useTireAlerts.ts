import { useEffect, useRef, useState } from "react";
import type { TireTelemetryData } from "./useTireTelemetry";
import type { Alert } from "@/components/AlertBox";

const WHEEL_LABELS: Record<string, string> = {
  fl: "FL", fr: "FR", rl: "RL", rr: "RR",
};

const WHEELS = ["fl", "fr", "rl", "rr"] as const;

function timestamp(): string {
  const d = new Date();
  return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}:${d.getSeconds().toString().padStart(2, "0")}`;
}

/** Human-readable labels and category tags for cleared-condition messages. */
const CLEAR_MESSAGES: Record<string, { msg: (label: string) => string; tag: string }> = {
  temp_crit: { msg: (l) => `${l} temp back to normal`, tag: "TEMP" },
  temp_warn: { msg: (l) => `${l} temp back to normal`, tag: "TEMP" },
  wear_crit: { msg: (l) => `${l} tyre wear stabilised`, tag: "WEAR" },
  wear_warn: { msg: (l) => `${l} tyre wear stabilised`, tag: "WEAR" },
  dmg_crit: { msg: (l) => `${l} tyre damage stabilised`, tag: "DMG" },
  dmg_warn: { msg: (l) => `${l} tyre damage stabilised`, tag: "DMG" },
  blst_crit: { msg: (l) => `${l} blistering subsided`, tag: "BLST" },
  blst_warn: { msg: (l) => `${l} blistering subsided`, tag: "BLST" },
};

/**
 * Accumulates tire alerts over time. Each condition fires once when it first
 * triggers, and won't fire again until the condition clears and re-triggers.
 * When a condition clears, an info alert is added.
 */
export function useTireAlerts(data: TireTelemetryData | null): { alerts: Alert[]; activeCount: number } {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [activeCount, setActiveCount] = useState(0);
  const activeConditions = useRef<Set<string>>(new Set());
  const prevCompound = useRef<string>("");

  useEffect(() => {
    if (!data) return;

    const newAlerts: Alert[] = [];
    const currentConditions = new Set<string>();
    const ts = timestamp();

    for (const wn of WHEELS) {
      const t = data.tires[wn];
      const label = WHEEL_LABELS[wn];
      const life = Math.round(100 - t.wear);

      // Surface temperature
      if (t.surfaceTemp > 108) {
        currentConditions.add(`${wn}_temp_crit`);
        currentConditions.add(`${wn}_temp_warn`);
        if (!activeConditions.current.has(`${wn}_temp_crit`)) {
          newAlerts.push({ level: "critical", message: `${label} surface temp ${t.surfaceTemp}°C — overheating`, time: `${ts} TEMP` });
        }
      } else if (t.surfaceTemp > 103) {
        currentConditions.add(`${wn}_temp_warn`);
        if (!activeConditions.current.has(`${wn}_temp_warn`)) {
          newAlerts.push({ level: "warning", message: `${label} surface temp ${t.surfaceTemp}°C — approaching limit`, time: `${ts} TEMP` });
        }
      }

      // Wear
      if (life <= 10) {
        currentConditions.add(`${wn}_wear_crit`);
        currentConditions.add(`${wn}_wear_warn`);
        if (!activeConditions.current.has(`${wn}_wear_crit`)) {
          newAlerts.push({ level: "critical", message: `${label} tyre life critically low at ${life}%`, time: `${ts} WEAR` });
        }
      } else if (life <= 25) {
        currentConditions.add(`${wn}_wear_warn`);
        if (!activeConditions.current.has(`${wn}_wear_warn`)) {
          newAlerts.push({ level: "warning", message: `${label} tyre life low at ${life}%`, time: `${ts} WEAR` });
        }
      }

      // Damage
      if (t.damage > 150) {
        currentConditions.add(`${wn}_dmg_crit`);
        currentConditions.add(`${wn}_dmg_warn`);
        if (!activeConditions.current.has(`${wn}_dmg_crit`)) {
          newAlerts.push({ level: "critical", message: `${label} tyre damage critical (${t.damage}/255)`, time: `${ts} DMG` });
        }
      } else if (t.damage > 50) {
        currentConditions.add(`${wn}_dmg_warn`);
        if (!activeConditions.current.has(`${wn}_dmg_warn`)) {
          newAlerts.push({ level: "warning", message: `${label} tyre damage detected (${t.damage}/255)`, time: `${ts} DMG` });
        }
      }

      // Blisters
      if (t.blisters > 150) {
        currentConditions.add(`${wn}_blst_crit`);
        currentConditions.add(`${wn}_blst_warn`);
        if (!activeConditions.current.has(`${wn}_blst_crit`)) {
          newAlerts.push({ level: "critical", message: `${label} severe blistering (${t.blisters}/255)`, time: `${ts} BLST` });
        }
      } else if (t.blisters > 50) {
        currentConditions.add(`${wn}_blst_warn`);
        if (!activeConditions.current.has(`${wn}_blst_warn`)) {
          newAlerts.push({ level: "warning", message: `${label} blistering detected (${t.blisters}/255)`, time: `${ts} BLST` });
        }
      }
    }

    // Detect cleared conditions
    for (const prevKey of activeConditions.current) {
      if (!currentConditions.has(prevKey)) {
        const underscoreIdx = prevKey.indexOf("_");
        const wn = prevKey.substring(0, underscoreIdx);
        const condType = prevKey.substring(underscoreIdx + 1);
        const label = WHEEL_LABELS[wn] ?? wn.toUpperCase();
        const clear = CLEAR_MESSAGES[condType];
        if (clear) {
          newAlerts.push({ level: "info", message: clear.msg(label), time: `${ts} ${clear.tag}` });
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
