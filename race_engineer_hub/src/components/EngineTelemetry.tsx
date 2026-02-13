import { Zap } from "lucide-react";
import { TelemetryCard, Metric, BarGauge } from "./TelemetryCard";

export function EngineTelemetry() {
  return (
    <TelemetryCard
      title="Power Unit"
      icon={<Zap className="h-4 w-4 text-primary" />}
      status="nominal"
    >
      <div className="grid grid-cols-3 gap-x-3 gap-y-1">
        <Metric label="RPM" value="12,800" />
        <Metric label="ICE Temp" value={122} unit="°C" warn />
        <Metric label="Oil Temp" value={138} unit="°C" />
        <Metric label="Oil Pres." value={4.5} unit="bar" />
        <Metric label="Water Temp" value={115} unit="°C" />
        <Metric label="Fuel Flow" value={100.0} unit="kg/h" />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={1000} max={1000} label="ICE Power (bhp)" />
        <BarGauge value={163} max={163} label="ERS Deploy (kW)" />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={98} max={100} label="Battery SOC %" warn={30} critical={15} />
        <BarGauge value={8.2} max={12} label="Fuel (kg)" warn={3} critical={1} />
      </div>

      <div className="flex items-center justify-between pt-2 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span>Mode: QUALI — MAX POWER</span>
        <span>ERS: OVERTAKE</span>
      </div>
    </TelemetryCard>
  );
}
