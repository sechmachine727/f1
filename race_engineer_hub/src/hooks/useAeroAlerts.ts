import { useEffect, useRef, useState } from "react";
import type { AeroData } from "./useAeroTelemetry";
import type { Alert } from "@/components/AlertBox";

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

const DAMAGE_REFIRE_STEP = 10; // re-alert every 10% worsening

const DAMAGE_PARTS: { key: keyof AeroData; label: string; tag: string; clearMsg: string }[] = [
  { key: "frontLeftWingDamage", label: "Front left wing", tag: "FL WING", clearMsg: "front left wing damage stabilised" },
  { key: "frontRightWingDamage", label: "Front right wing", tag: "FR WING", clearMsg: "front right wing damage stabilised" },
  { key: "rearWingDamage", label: "Rear wing", tag: "RR WING", clearMsg: "rear wing damage stabilised" },
  { key: "floorDamage", label: "Floor", tag: "FLOOR", clearMsg: "floor damage stabilised" },
  { key: "diffuserDamage", label: "Diffuser", tag: "DIFF", clearMsg: "diffuser damage stabilised" },
  { key: "sidepodDamage", label: "Sidepod", tag: "SIDEPOD", clearMsg: "sidepod damage stabilised" },
];

const BRAKE_TEMPS: { key: keyof AeroData; label: string; tag: string }[] = [
  { key: "brakeTempFL", label: "FL brake", tag: "BRK FL" },
  { key: "brakeTempFR", label: "FR brake", tag: "BRK FR" },
  { key: "brakeTempRL", label: "RL brake", tag: "BRK RL" },
  { key: "brakeTempRR", label: "RR brake", tag: "BRK RR" },
];

/**
 * Accumulates aero alerts over time. Same edge-detection approach as
 * the tire and power-unit hooks.
 */
export function useAeroAlerts(data: AeroData | null): { alerts: Alert[]; activeCount: number } {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [activeCount, setActiveCount] = useState(0);
  const activeConditions = useRef<Map<string, ConditionState>>(new Map());
  const prevSessionTime = useRef<number>(0);
  const prevDrsFault = useRef<boolean>(false);

  useEffect(() => {
    if (!data) return;

    // Detect new session (session time resets)
    if (data.sessionTime < prevSessionTime.current) {
      setAlerts([]);
      setActiveCount(0);
      activeConditions.current = new Map();
      prevDrsFault.current = false;
    }
    prevSessionTime.current = data.sessionTime;

    const newAlerts: Alert[] = [];
    const currentConditions = new Map<string, ConditionState>();
    const ts = formatSessionTime(data.sessionTime);

    // Damage conditions for each aero part
    for (const part of DAMAGE_PARTS) {
      const val = data[part.key] as number;
      if (val > 50) {
        currentConditions.set(part.key, { level: "crit", value: val });
      } else if (val > 20) {
        currentConditions.set(part.key, { level: "warn", value: val });
      }
    }

    // Brake temperatures
    for (const brk of BRAKE_TEMPS) {
      const val = data[brk.key] as number;
      if (val > 1000) {
        currentConditions.set(brk.key, { level: "crit", value: val });
      } else if (val > 800) {
        currentConditions.set(brk.key, { level: "warn", value: val });
      }
    }

    // DRS fault
    if (data.drsFault) {
      currentConditions.set("drsFault", { level: "crit", value: 1 });
    }

    // Fire alerts for new, escalated, or worsened conditions
    for (const [key, cur] of currentConditions) {
      const prev = activeConditions.current.get(key);
      const isNew = !prev;
      const isEscalation = prev && prev.level === "warn" && cur.level === "crit";
      const part = DAMAGE_PARTS.find((p) => p.key === key);
      const isWorsened = part && prev && prev.level === cur.level
        && cur.value >= prev.value + DAMAGE_REFIRE_STEP;

      if (isNew || isEscalation || isWorsened) {
        const alertLevel = cur.level === "crit" ? "critical" : "warning";

        if (key === "drsFault") {
          newAlerts.push({
            level: "critical",
            message: "DRS system fault detected",
            time: `${ts} DRS`,
          });
        } else if (part) {
          newAlerts.push({
            level: alertLevel,
            message: cur.level === "crit"
              ? `${part.label} damage critical (${cur.value}%)`
              : `${part.label} damage detected (${cur.value}%)`,
            time: `${ts} ${part.tag}`,
          });
          // Update stored value so next re-fire uses new baseline
          cur.value = cur.value;
        } else {
          const brk = BRAKE_TEMPS.find((b) => b.key === key);
          if (brk) {
            newAlerts.push({
              level: alertLevel,
              message: cur.level === "crit"
                ? `${brk.label} temp ${cur.value}°C — overheating`
                : `${brk.label} temp ${cur.value}°C — running hot`,
              time: `${ts} ${brk.tag}`,
            });
          }
        }
      } else if (prev) {
        // Keep the previous alerted value as baseline when no alert fires
        cur.value = prev.value;
      }
    }

    // Detect fully cleared conditions
    for (const [key] of activeConditions.current) {
      if (!currentConditions.has(key)) {
        if (key === "drsFault") {
          newAlerts.push({ level: "info", message: "DRS fault cleared", time: `${ts} DRS` });
        } else {
          const part = DAMAGE_PARTS.find((p) => p.key === key);
          if (part) {
            newAlerts.push({ level: "info", message: part.clearMsg, time: `${ts} ${part.tag}` });
          } else {
            const brk = BRAKE_TEMPS.find((b) => b.key === key);
            if (brk) {
              newAlerts.push({ level: "info", message: `${brk.label} temp back to normal`, time: `${ts} ${brk.tag}` });
            }
          }
        }
      }
    }

    activeConditions.current = currentConditions;
    setActiveCount(currentConditions.size);

    if (newAlerts.length > 0) {
      setAlerts((prev) => [...prev, ...newAlerts]);
    }
  }, [data]);

  return { alerts, activeCount };
}
