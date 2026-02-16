import { Circle } from "lucide-react";
import { TelemetryCard, BarGauge } from "./TelemetryCard";
import { useTireTelemetry } from "@/hooks/useTireTelemetry";

const POSITIONS = [
  { key: "fl" as const, label: "FL" },
  { key: "fr" as const, label: "FR" },
  { key: "rl" as const, label: "RL" },
  { key: "rr" as const, label: "RR" },
];

const COMPOUND_COLORS: Record<string, string> = {
  soft: "#FF3333",
  medium: "#FFC906",
  hard: "#FFFFFF",
  inter: "#39B54A",
  wet: "#0072CE",
};

export function TireTelemetry() {
  const data = useTireTelemetry();

  if (!data) {
    return (
      <TelemetryCard
        title="Tires"
        icon={<Circle className="h-4 w-4 text-primary" />}
        status="nominal"
      >
        <div className="flex items-center justify-center py-6 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          Waiting for telemetry…
        </div>
      </TelemetryCard>
    );
  }

  const temps = POSITIONS.map((p) => data.tires[p.key].surfaceTemp);
  const maxTemp = Math.max(...temps);
  const status = maxTemp > 108 ? "critical" : maxTemp > 103 ? "warning" : "nominal";

  const compoundText = data.compound
    ? `${data.compoundVisual.toUpperCase()} (${data.compound})`
    : "—";
  const compoundColor = COMPOUND_COLORS[data.compoundVisual] ?? undefined;

  return (
    <TelemetryCard
      title="Tires"
      icon={<Circle className="h-4 w-4 text-primary" />}
      status={status}
    >
      <div className="grid grid-cols-2 gap-2">
        {POSITIONS.map((p) => {
          const t = data.tires[p.key];
          const life = Math.max(0, Math.round(100 - t.wear));
          return (
            <div key={p.key} className="bg-secondary/50 rounded p-2 flex flex-col gap-1 border border-border/50">
              <div className="flex items-center justify-between">
                <span className="font-display text-[10px] font-bold tracking-wider text-foreground">{p.label}</span>
                <span className={`font-display text-sm font-bold ${t.surfaceTemp > 108 ? "text-accent" : t.surfaceTemp > 103 ? "text-warning" : "text-primary"}`}>{t.surfaceTemp}°</span>
              </div>
              <div className="flex items-center justify-between text-[9px] text-muted-foreground">
                <span>{t.pressure.toFixed(1)} bar</span>
              </div>
              <BarGauge value={life} max={100} label="Life" warn={40} critical={25} invertThresholds />
              <BarGauge value={t.damage} max={255} label="Damage" warn={50} critical={150} />
              <BarGauge value={t.blisters} max={255} label="Blisters" warn={50} critical={150} />
            </div>
          );
        })}
      </div>
      <div className="flex items-center justify-between pt-1 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span style={compoundColor ? { color: compoundColor } : undefined} className="font-bold">{compoundText}</span>
        <span>Age: {data.tyresAgeLaps} laps</span>
      </div>
    </TelemetryCard>
  );
}
