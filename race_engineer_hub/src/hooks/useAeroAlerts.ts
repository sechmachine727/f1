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

const DAMAGE_PARTS: { key: keyof AeroData; label: string; tag: string; clearMsg: string }[] = [
  { key: "frontLeftWingDamage", label: "Front left wing", tag: "FL WING", clearMsg: "front left wing damage stabilised" },
  { key: "frontRightWingDamage", label: "Front right wing", tag: "FR WING", clearMsg: "front right wing damage stabilised" },
  { key: "rearWingDamage", label: "Rear wing", tag: "RR WING", clearMsg: "rear wing damage stabilised" },
  { key: "floorDamage", label: "Floor", tag: "FLOOR", clearMsg: "floor damage stabilised" },
  { key: "diffuserDamage", label: "Diffuser", tag: "DIFF", clearMsg: "diffuser damage stabilised" },
  { key: "sidepodDamage", label: "Sidepod", tag: "SIDEPOD", clearMsg: "sidepod damage stabilised" },
];

/**
 * Accumulates aero alerts over time. Same edge-detection approach as
 * the tire and power-unit hooks.
 */
export function useAeroAlerts(data: AeroData | null): { alerts: Alert[]; activeCount: number } {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [activeCount, setActiveCount] = useState(0);
  const activeConditions = useRef<Map<string, Level>>(new Map());
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
    const currentConditions = new Map<string, Level>();
    const ts = formatSessionTime(data.sessionTime);

    // Damage conditions for each aero part
    for (const part of DAMAGE_PARTS) {
      const val = data[part.key] as number;
      if (val > 50) {
        currentConditions.set(part.key, "crit");
      } else if (val > 20) {
        currentConditions.set(part.key, "warn");
      }
    }

    // DRS fault
    if (data.drsFault) {
      currentConditions.set("drsFault", "crit");
    }

    // Fire alerts for new or escalated conditions
    for (const [key, level] of currentConditions) {
      const prev = activeConditions.current.get(key);

      if (!prev || (prev === "warn" && level === "crit")) {
        const alertLevel = level === "crit" ? "critical" : "warning";

        if (key === "drsFault") {
          newAlerts.push({
            level: "critical",
            message: "DRS system fault detected",
            time: `${ts} DRS`,
          });
        } else {
          const part = DAMAGE_PARTS.find((p) => p.key === key);
          if (part) {
            const val = data[part.key] as number;
            newAlerts.push({
              level: alertLevel,
              message: level === "crit"
                ? `${part.label} damage critical (${val}%)`
                : `${part.label} damage detected (${val}%)`,
              time: `${ts} ${part.tag}`,
            });
          }
        }
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
