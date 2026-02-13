import { Circle } from "lucide-react";
import { TelemetryCard, Metric, BarGauge } from "./TelemetryCard";

const tires = [
  { pos: "FL", temp: 102, wear: 96, pressure: 23.5 },
  { pos: "FR", temp: 104, wear: 95, pressure: 23.7 },
  { pos: "RL", temp: 99, wear: 97, pressure: 23.0 },
  { pos: "RR", temp: 105, wear: 94, pressure: 23.3 },
];

export function TireTelemetry() {
  const maxTemp = Math.max(...tires.map(t => t.temp));
  const status = maxTemp > 108 ? "critical" : maxTemp > 103 ? "warning" : "nominal";

  return (
    <TelemetryCard
      title="Tires"
      icon={<Circle className="h-4 w-4 text-primary" />}
      status={status}
    >
      <div className="grid grid-cols-2 gap-2">
        {tires.map((t) => (
          <div key={t.pos} className="bg-secondary/50 rounded p-2 flex flex-col gap-1 border border-border/50">
            <div className="flex items-center justify-between">
              <span className="font-display text-[10px] font-bold tracking-wider text-foreground">{t.pos}</span>
              <span className={`font-display text-sm font-bold ${t.temp > 108 ? "text-accent" : t.temp > 103 ? "text-warning" : "text-primary"}`}>{t.temp}°</span>
            </div>
            <div className="flex items-center justify-between text-[9px] text-muted-foreground">
              <span>{t.pressure.toFixed(1)} PSI</span>
            </div>
            <BarGauge value={t.wear} max={100} label="Life" warn={40} critical={25} />
          </div>
        ))}
      </div>
      <div className="flex items-center justify-between pt-1 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span>NEW SOFT (C5)</span>
        <span>Sets Left: 2</span>
      </div>
    </TelemetryCard>
  );
}
