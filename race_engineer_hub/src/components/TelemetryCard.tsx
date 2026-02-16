import { ReactNode } from "react";

interface TelemetryCardProps {
  title: string;
  icon: ReactNode;
  children: ReactNode;
  status?: "nominal" | "warning" | "critical";
}

const statusStyles = {
  nominal: "border-primary/30 glow-primary",
  warning: "border-warning/40 glow-warning",
  critical: "border-accent/40 glow-accent",
};

const statusLabels = {
  nominal: { text: "NOMINAL", class: "text-primary" },
  warning: { text: "WARNING", class: "text-warning" },
  critical: { text: "CRITICAL", class: "text-accent" },
};

export function TelemetryCard({ title, icon, children, status = "nominal" }: TelemetryCardProps) {
  const s = statusLabels[status];
  return (
    <div className={`bg-card rounded-md border ${statusStyles[status]} p-3 flex flex-col gap-2 telemetry-grid`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {icon}
          <h2 className="font-display text-sm font-bold tracking-widest uppercase text-foreground">{title}</h2>
        </div>
        <span className={`font-display text-[10px] font-bold tracking-wider ${s.class} animate-pulse`}>● {s.text}</span>
      </div>
      {children}
    </div>
  );
}

interface MetricProps {
  label: string;
  value: string | number;
  unit?: string;
  warn?: boolean;
  critical?: boolean;
}

export function Metric({ label, value, unit, warn, critical }: MetricProps) {
  const color = critical ? "text-accent" : warn ? "text-warning" : "text-primary";
  return (
    <div className="flex flex-col">
      <span className="text-[9px] uppercase tracking-wider text-muted-foreground">{label}</span>
      <div className="flex items-baseline gap-0.5">
        <span className={`font-display text-base font-bold ${color}`}>{value}</span>
        {unit && <span className="text-[10px] text-muted-foreground">{unit}</span>}
      </div>
    </div>
  );
}

interface BarGaugeProps {
  value: number;
  max: number;
  label: string;
  warn?: number;
  critical?: number;
  /** When true, warn/critical trigger when value drops *below* the threshold. */
  invertThresholds?: boolean;
}

export function BarGauge({ value, max, label, warn, critical, invertThresholds }: BarGaugeProps) {
  const pct = Math.min((value / max) * 100, 100);
  const isWarn = warn !== undefined && (invertThresholds ? value <= warn : value >= warn);
  const isCrit = critical !== undefined && (invertThresholds ? value <= critical : value >= critical);
  const barColor = isCrit ? "bg-accent" : isWarn ? "bg-warning" : "bg-primary";

  const textColor = isCrit ? "text-accent" : isWarn ? "text-warning" : "text-muted-foreground";

  return (
    <div className="flex flex-col gap-0.5">
      <div className={`flex justify-between text-[9px] uppercase tracking-wider ${textColor}`}>
        <span>{label}</span>
        <span>{value}/{max}</span>
      </div>
      <div className="h-1.5 rounded-full bg-secondary overflow-hidden">
        <div className={`h-full rounded-full ${barColor} transition-all duration-500`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
