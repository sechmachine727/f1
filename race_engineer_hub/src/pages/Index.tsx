import { TyreTelemetry } from "@/components/TyreTelemetry";
import { EngineTelemetry } from "@/components/EngineTelemetry";
import { AeroTelemetry } from "@/components/AeroTelemetry";
import { TyreAlertPanel } from "@/components/TyreAlertPanel";
import { PuAlertPanel } from "@/components/PuAlertPanel";
import { DamageAlertPanel } from "@/components/DamageAlertPanel";
import { RaceEngineerPanel } from "@/components/RaceEngineerPanel";
import { DriverRadioInput } from "@/components/DriverRadioInput";
import { TrackMap } from "@/components/TrackMap";
import { LapTimingPanel } from "@/components/LapTimingPanel";
import { StandingsPanel } from "@/components/StandingsPanel";
import { useSessionTelemetry, formatTimeLeft } from "@/hooks/useSessionTelemetry";
import { useTyreTelemetry } from "@/hooks/useTyreTelemetry";
import { usePowerUnitTelemetry } from "@/hooks/usePowerUnitTelemetry";
import { useAeroTelemetry } from "@/hooks/useAeroTelemetry";
import { useRaceEngineerReport } from "@/hooks/useRaceEngineerReport";
import { useTimingData } from "@/hooks/useTimingData";
import { Flag, Clock, CloudSun, Thermometer, MapPin, CircleParking } from "lucide-react";
import { useState, useCallback } from "react";
import type { ReportEntry } from "@/components/ExpandableReportPanel";

const WEATHER_LABELS: Record<number, string> = {
  0: "Clear", 1: "Light Cloud", 2: "Overcast",
  3: "Light Rain", 4: "Heavy Rain", 5: "Storm",
};

const Index = () => {
  const session = useSessionTelemetry();
  const { tyreAlerts: { alerts: tyreAlerts, activeCount: tyreActiveCount }, tyresReport } = useTyreTelemetry();
  const { puAlerts: { alerts: puAlerts, activeCount: puActiveCount }, puReport } = usePowerUnitTelemetry();
  const { aeroAlerts: { alerts: aeroAlerts, activeCount: aeroActiveCount }, damageReport } = useAeroTelemetry();
  const raceEngineerReport = useRaceEngineerReport();
  const { timing, standings, lapCompleted } = useTimingData();
  const [driverMessages, setDriverMessages] = useState<ReportEntry[]>([]);
  const handleDriverSend = useCallback((text: string, time: string) => {
    setDriverMessages((prev) => [...prev, { text, time, source: "driver" as const }]);
  }, []);

  const handleSessionReset = useCallback(() => {
    setDriverMessages([]);
  }, []);

  const sessionType = session?.sessionType ?? "—";
  const trackName = session?.trackName ?? "—";
  const timeLeftStr = session ? formatTimeLeft(session.sessionTimeLeft) : "—";
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
          {session && (
            <div className="flex items-center gap-1 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
              <CloudSun className="h-3 w-3 text-primary" />
              <span>{WEATHER_LABELS[session.weather] ?? "Unknown"}</span>
            </div>
          )}
          {session && (
            <div className="flex items-center gap-1 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
              <Thermometer className="h-3 w-3 text-primary" />
              <span>{session.airTemp}°C AIR</span>
              <span className="text-border/80">/</span>
              <MapPin className="h-3 w-3 text-primary" />
              <span>{session.trackTemp}°C TRACK</span>
            </div>
          )}
        </div>
        <div className="flex items-center gap-4 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          {session && session.pitStatus === 1 && (
            <div className="flex items-center gap-1.5 bg-warning/20 border border-warning/50 rounded px-2 py-0.5 animate-pulse">
              <CircleParking className="h-3.5 w-3.5 text-warning" />
              <span className="text-warning font-bold text-[11px]">PIT IN</span>
              {session.pitLaneTimerActive && (
                <span className="text-warning font-bold tabular-nums min-w-[3.5ch] text-right">{(session.pitLaneTimeMs / 1000).toFixed(1)}s</span>
              )}
            </div>
          )}
          {session && session.pitStatus === 2 && (
            <div className="flex items-center gap-1.5 bg-accent/20 border border-accent/50 rounded px-2 py-0.5 animate-pulse">
              <CircleParking className="h-3.5 w-3.5 text-accent" />
              <span className="text-accent font-bold text-[11px]">PIT BOX</span>
              {session.pitStopTimeMs > 0 && (
                <span className="text-accent font-bold tabular-nums">{(session.pitStopTimeMs / 1000).toFixed(1)}s</span>
              )}
            </div>
          )}
          {session && session.pitStatus === 0 && session.numPitStops > 0 && (
            <div className="flex items-center gap-1 bg-primary/10 border border-primary/30 rounded px-2 py-0.5">
              <CircleParking className="h-3 w-3 text-primary" />
              <span className="text-primary font-bold">PIT &times;{session.numPitStops}</span>
            </div>
          )}
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
        </div>
      </header>

      {/* Telemetry Grid + Alerts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3 mb-3 flex-1 min-h-0">
        <div className="flex flex-col gap-2 min-h-0">
          <TyreTelemetry />
          <TyreAlertPanel alerts={tyreAlerts} activeCount={tyreActiveCount} engineerResponses={tyresReport.responses} />
        </div>
        <div className="flex flex-col gap-2 min-h-0">
          <EngineTelemetry />
          <PuAlertPanel alerts={puAlerts} activeCount={puActiveCount} engineerResponses={puReport.responses} />
        </div>
        <div className="flex flex-col gap-2 min-h-0">
          <AeroTelemetry />
          <DamageAlertPanel alerts={aeroAlerts} activeCount={aeroActiveCount} engineerResponses={damageReport.responses} />
        </div>
      </div>

      {/* Bottom: Race Engineer + Driver Radio | Timing + Standings | Track Map */}
      <div className="shrink-0 grid grid-cols-1 lg:grid-cols-4 gap-3 h-[22rem]">
        <div className="lg:col-span-2 flex flex-col gap-2 min-h-0">
          <RaceEngineerPanel report={raceEngineerReport} driverMessages={driverMessages} className="flex-1 min-h-0" />
          <DriverRadioInput onSend={handleDriverSend} onSessionReset={handleSessionReset} />
        </div>
        <div className="flex flex-col gap-2 min-h-0">
          <LapTimingPanel timing={timing} lapCompleted={lapCompleted} />
          <StandingsPanel standings={standings} />
        </div>
        <div className="min-h-0">
          <TrackMap />
        </div>
      </div>
    </div>
  );
};

export default Index;
