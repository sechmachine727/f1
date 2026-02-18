import { Zap } from "lucide-react";
import { TelemetryCard, Metric, BarGauge } from "./TelemetryCard";
import { usePowerUnitTelemetry } from "@/hooks/usePowerUnitTelemetry";

const ERS_MAX_ENERGY_J = 4_000_000; // 4 MJ per F1 regulations

export function EngineTelemetry() {
  const data = usePowerUnitTelemetry();

  if (!data) {
    return (
      <TelemetryCard
        title="Power Unit"
        icon={<Zap className="h-4 w-4 text-primary" />}
        status="nominal"
      >
        <div className="flex items-center justify-center py-6 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          Waiting for telemetry…
        </div>
      </TelemetryCard>
    );
  }

  const engineTempWarn = data.engineTemp > 120;
  const engineTempCrit = data.engineTemp > 130;
  const status = engineTempCrit || data.engineDamage > 20
    ? "critical"
    : engineTempWarn || data.engineDamage > 5
      ? "warning"
      : "nominal";

  const iceBHP = Math.round(data.icePowerKW * 1.341);
  const mgukBHP = Math.round(data.mgukPowerKW * 1.341);

  return (
    <TelemetryCard
      title="Power Unit"
      icon={<Zap className="h-4 w-4 text-primary" />}
      status={status}
    >
      <div className="grid grid-cols-3 gap-x-3 gap-y-1">
        <Metric label="RPM" value={data.rpm.toLocaleString()} />
        <Metric label="Engine Temp" value={data.engineTemp} unit="°C" warn={engineTempWarn} critical={engineTempCrit} />
        <Metric label="Gear" value={data.gear <= 0 ? (data.gear === 0 ? "N" : "R") : data.gear} />
        <Metric label="Fuel" value={data.fuelInTank} unit="kg" warn={data.fuelRemainingLaps < 3} critical={data.fuelRemainingLaps < 1} />
        <Metric label="Fuel +/- Laps" value={data.fuelRemainingLaps > 0 ? `+${data.fuelRemainingLaps}` : data.fuelRemainingLaps} />
        <Metric label="Fuel Mix" value={data.fuelMix} />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={iceBHP} max={1000} label="ICE Power (bhp)" />
        <BarGauge value={mgukBHP} max={163} label="MGU-K (bhp)" />
      </div>

      <div className="grid grid-cols-2 gap-2">
        <BarGauge value={data.batteryPct} max={100} label="Battery SOC %" warn={30} critical={15} invertThresholds />
        <BarGauge value={data.fuelInTank} max={110} label="Fuel (kg)" warn={16.5} critical={5.5} invertThresholds />
      </div>

      <div className="flex items-center justify-between pt-2 border-t border-border/50 text-[10px] text-muted-foreground uppercase tracking-wider">
        <span>Mix: {data.fuelMix}</span>
        <span>ERS: {data.ersDeployMode}</span>
      </div>
    </TelemetryCard>
  );
}
