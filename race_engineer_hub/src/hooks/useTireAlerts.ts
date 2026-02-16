import { useMemo } from "react";
import type { TireTelemetryData } from "./useTireTelemetry";
import type { Alert } from "@/components/AlertBox";

const WHEEL_LABELS: Record<string, string> = {
  fl: "FL", fr: "FR", rl: "RL", rr: "RR",
};

const WHEELS = ["fl", "fr", "rl", "rr"] as const;

export function useTireAlerts(data: TireTelemetryData | null): Alert[] {
  return useMemo(() => {
    if (!data) return [];
    const alerts: Alert[] = [];

    for (const wn of WHEELS) {
      const t = data.tires[wn];
      const label = WHEEL_LABELS[wn];

      // Surface temperature
      if (t.surfaceTemp > 108) {
        alerts.push({
          level: "critical",
          message: `${label} surface temp ${t.surfaceTemp}°C — overheating`,
          time: "TEMP",
        });
      } else if (t.surfaceTemp > 103) {
        alerts.push({
          level: "warning",
          message: `${label} surface temp ${t.surfaceTemp}°C — approaching limit`,
          time: "TEMP",
        });
      }

      // Wear
      const life = Math.round(100 - t.wear);
      if (life <= 10) {
        alerts.push({
          level: "critical",
          message: `${label} tyre life critically low at ${life}%`,
          time: "WEAR",
        });
      } else if (life <= 25) {
        alerts.push({
          level: "warning",
          message: `${label} tyre life low at ${life}%`,
          time: "WEAR",
        });
      }

      // Damage
      if (t.damage > 150) {
        alerts.push({
          level: "critical",
          message: `${label} tyre damage critical (${t.damage}/255)`,
          time: "DMG",
        });
      } else if (t.damage > 50) {
        alerts.push({
          level: "warning",
          message: `${label} tyre damage detected (${t.damage}/255)`,
          time: "DMG",
        });
      }

      // Blisters
      if (t.blisters > 150) {
        alerts.push({
          level: "critical",
          message: `${label} severe blistering (${t.blisters}/255)`,
          time: "BLST",
        });
      } else if (t.blisters > 50) {
        alerts.push({
          level: "warning",
          message: `${label} blistering detected (${t.blisters}/255)`,
          time: "BLST",
        });
      }
    }

    // Compound info
    if (data.compound) {
      alerts.push({
        level: "info",
        message: `${data.compoundVisual.toUpperCase()} (${data.compound}) — age ${data.tyresAgeLaps} laps`,
        time: "TYRE",
      });
    }

    return alerts;
  }, [data]);
}
