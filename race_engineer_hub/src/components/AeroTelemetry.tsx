import { Wind } from "lucide-react";
import { TelemetryCard, Metric, BarGauge } from "./TelemetryCard";

export function AeroTelemetry() {
  return (
    <TelemetryCard
      title="Aero"
      icon={<Wind className="h-4 w-4 text-primary" />}
      status="nominal"
    >
      <div className="grid grid-cols-3 gap-x-3 gap-y-1">
        <Metric label="Downforce" value={1680} unit="N" />
        <Metric label="Drag Coeff." value={0.82} />
        <Metric label="L/D Ratio" value={4.38} />
        <Metric label="Front Wing" value={6.8} unit="°" />
        <Metric label="Rear Wing" value={11.0} unit="°" />
        <Metric label="Ride Height" value={28} unit="mm" />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={92} max={100} label="DRS Efficiency %" />
        <BarGauge value={338} max={340} label="Top Speed (km/h)" />
      </div>

      <div>
        <BarGauge value={58} max={100} label="Aero Balance (% Front)" warn={70} critical={80} />
      </div>

      <div className="flex items-center justify-between pt-2 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span>DRS: ACTIVE — LOW DRAG</span>
        <span>Beam Wing: OPEN</span>
      </div>
    </TelemetryCard>
  );
}
