import { TireTelemetry } from "@/components/TireTelemetry";
import { EngineTelemetry } from "@/components/EngineTelemetry";
import { AeroTelemetry } from "@/components/AeroTelemetry";
import { AlertBox } from "@/components/AlertBox";
import { DamageReportPanel } from "@/components/DamageReportPanel";
import { PuReportPanel } from "@/components/PuReportPanel";
import { RaceEngineerPanel } from "@/components/RaceEngineerPanel";
import { TiresReportPanel } from "@/components/TiresReportPanel";
import { EngineerReport, type Instruction } from "@/components/EngineerReport";
import { useSessionTelemetry, formatLapTime, formatTimeLeft } from "@/hooks/useSessionTelemetry";
import { useTireTelemetry } from "@/hooks/useTireTelemetry";
import { usePowerUnitTelemetry } from "@/hooks/usePowerUnitTelemetry";
import { useAeroTelemetry } from "@/hooks/useAeroTelemetry";
import { useRaceEngineerReport } from "@/hooks/useRaceEngineerReport";
import { Flag, Timer, Gauge, Clock } from "lucide-react";

const instructions: Instruction[] = [
  { priority: "high", message: "This is the final run. Push lap is NOW. Everything on the table. Maximum attack.", category: "QUALI" },
  { priority: "high", message: "Purple S1 needed. Brake later into Turn 1 — you have the grip. Commit to the apex.", category: "SECTOR 1" },
  { priority: "normal", message: "Norris just set 1:23.6. You need a 1:23.4 for pole. It's there in Sector 3.", category: "TARGET" },
  { priority: "normal", message: "Track temp dropping — 0.2s available in rear traction zones. Use it.", category: "TRACK" },
  { priority: "high", message: "DRS open from detection point. Stay within 1s of car ahead on out-lap for slipstream on main straight.", category: "DRS" },
  { priority: "normal", message: "Battery is full. Deploy everything. No need to harvest. One-shot lap.", category: "ERS" },
];

const Index = () => {
  const session = useSessionTelemetry();
  const { tireAlerts: { alerts: tireAlerts, activeCount: tireActiveCount }, tiresReport } = useTireTelemetry();
  const { puAlerts: { alerts: puAlerts, activeCount: puActiveCount }, puReport } = usePowerUnitTelemetry();
  const { aeroAlerts: { alerts: aeroAlerts, activeCount: aeroActiveCount }, damageReport } = useAeroTelemetry();
  const raceEngineerReport = useRaceEngineerReport();

  const sessionType = session?.sessionType ?? "—";
  const trackName = session?.trackName ?? "—";
  const timeLeftStr = session ? formatTimeLeft(session.sessionTimeLeft) : "—";
  const position = session?.carPosition ?? 0;
  const lastLapStr = session ? formatLapTime(session.lastLapTimeMs) : "—";
  const totalLaps = session?.totalLaps ?? 0;

  return (
    <div className="h-screen overflow-hidden bg-background p-3 flex flex-col">
      {/* Header */}
      <header className="flex items-center justify-between mb-3 pb-2 border-b border-border/50 shrink-0">
        <div className="flex items-center gap-3">
          <Flag className="h-4 w-4 text-accent" />
          <h1 className="font-display text-sm md:text-base font-bold tracking-wider text-foreground">
            {trackName} — PIT WALL
          </h1>
          <span className="font-display text-[10px] font-bold tracking-wider text-accent bg-accent/10 border border-accent/30 px-2 py-0.5 rounded">
            {sessionType}
          </span>
        </div>
        <div className="flex items-center gap-4 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          {session && session.sessionTimeLeft > 0 ? (
            <div className="flex items-center gap-1">
              <Clock className="h-3 w-3 text-warning animate-pulse" />
              <span className="text-warning">{timeLeftStr} LEFT</span>
            </div>
          ) : totalLaps > 0 ? (
            <div className="flex items-center gap-1">
              <Clock className="h-3 w-3 text-primary" />
              <span>{totalLaps} LAPS</span>
            </div>
          ) : null}
          <div className="flex items-center gap-1">
            <Gauge className="h-3 w-3 text-primary" />
            <span>P{position || "—"}</span>
          </div>
          <div className="flex items-center gap-1">
            <Timer className="h-3 w-3 text-primary" />
            <span className="text-primary font-bold">{lastLapStr}</span>
          </div>
        </div>
      </header>

      {/* Telemetry Grid + Alerts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3 mb-3 min-h-0 overflow-auto">
        <div className="flex flex-col gap-2">
          <TireTelemetry />
          <AlertBox alerts={tireAlerts} activeCount={tireActiveCount} />
          <TiresReportPanel report={tiresReport} />
        </div>
        <div className="flex flex-col gap-2">
          <EngineTelemetry />
          <AlertBox alerts={puAlerts} activeCount={puActiveCount} />
          <PuReportPanel report={puReport} />
        </div>
        <div className="flex flex-col gap-2">
          <AeroTelemetry />
          <AlertBox alerts={aeroAlerts} activeCount={aeroActiveCount} />
          <DamageReportPanel report={damageReport} />
        </div>
      </div>

      {/* Race Engineer */}
      <div className="shrink-0">
        <RaceEngineerPanel report={raceEngineerReport} />
      </div>

      {/* Engineer Report */}
      <div className="min-h-0 flex-1 overflow-auto">
        <EngineerReport instructions={instructions} />
      </div>
    </div>
  );
};

export default Index;
