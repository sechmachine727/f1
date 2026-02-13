import { Wind } from "lucide-react";
import { TelemetryCard, Metric, BarGauge } from "./TelemetryCard";
import { useAeroTelemetry } from "@/hooks/useAeroTelemetry";

export function AeroTelemetry() {
  const data = useAeroTelemetry();

  if (!data) {
    return (
      <TelemetryCard
        title="Aero"
        icon={<Wind className="h-4 w-4 text-primary" />}
        status="nominal"
      >
        <div className="flex items-center justify-center py-6 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          Waiting for telemetry…
        </div>
      </TelemetryCard>
    );
  }

  const maxDamage = Math.max(
    data.frontWingDamage, data.rearWingDamage,
    data.floorDamage, data.diffuserDamage, data.sidepodDamage,
  );
  const status = data.drsFault || maxDamage > 50
    ? "critical"
    : maxDamage > 20
      ? "warning"
      : "nominal";

  return (
    <TelemetryCard
      title="Aero"
      icon={<Wind className="h-4 w-4 text-primary" />}
      status={status}
    >
      <div className="grid grid-cols-3 gap-x-3 gap-y-1">
        <Metric label="Front Wing" value={data.frontWing} />
        <Metric label="Rear Wing" value={data.rearWing} />
        <Metric label="Brake Bias" value={data.brakeBias} unit="%" />
        <Metric label="Ride Ht F" value={data.frontRideHeight.toFixed(1)} unit="mm" />
        <Metric label="Ride Ht R" value={data.rearRideHeight.toFixed(1)} unit="mm" />
        <Metric
          label="DRS"
          value={data.drsFault ? "FAULT" : data.drs ? "OPEN" : data.drsAllowed ? "READY" : "—"}
          critical={data.drsFault}
          warn={data.drs}
        />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={data.speed} max={350} label="Speed (km/h)" />
        <BarGauge value={data.brakeBias} max={100} label="Aero Balance (% Front)" warn={70} critical={80} />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={100 - data.frontWingDamage} max={100} label="Front Wing %" warn={40} critical={25} invertThresholds />
        <BarGauge value={100 - data.rearWingDamage} max={100} label="Rear Wing %" warn={40} critical={25} invertThresholds />
      </div>

      <div className="flex items-center justify-between pt-2 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span>Floor: {data.floorDamage > 0 ? `${data.floorDamage}% DMG` : "OK"}</span>
        <span>Diffuser: {data.diffuserDamage > 0 ? `${data.diffuserDamage}% DMG` : "OK"}</span>
      </div>
    </TelemetryCard>
  );
}
