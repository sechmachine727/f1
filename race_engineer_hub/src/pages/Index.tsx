import { TireTelemetry } from "@/components/TireTelemetry";
import { EngineTelemetry } from "@/components/EngineTelemetry";
import { AeroTelemetry } from "@/components/AeroTelemetry";
import { AlertBox, type Alert } from "@/components/AlertBox";
import { EngineerReport, type Instruction } from "@/components/EngineerReport";
import { Flag, Timer, Gauge, Clock } from "lucide-react";

const tireAlerts: Alert[] = [
  { level: "warning", message: "FR surface temp 101°C — approaching optimal window ceiling", time: "Q3 OUT" },
  { level: "info", message: "Tyre prep lap completed — target window 95-105°C", time: "Q3 OUT" },
  { level: "info", message: "New set of softs fitted — 2 sets remaining", time: "Q3 PIT" },
];

const engineAlerts: Alert[] = [
  { level: "info", message: "Engine mode QUALI — max power deployment enabled", time: "Q3 OUT" },
  { level: "info", message: "Full ERS harvest completed on out-lap", time: "Q3 OUT" },
  { level: "info", message: "Battery SOC 98% — ready for push lap", time: "Q3 HOT" },
];

const aeroAlerts: Alert[] = [
  { level: "info", message: "Low downforce qualifying trim — beam wing open", time: "Q3 OUT" },
  { level: "info", message: "DRS detection point in 400m — maintain gap to car ahead", time: "Q3 HOT" },
];

const instructions: Instruction[] = [
  { priority: "high", message: "This is the final run. Push lap is NOW. Everything on the table. Maximum attack.", category: "QUALI" },
  { priority: "high", message: "Purple S1 needed. Brake later into Turn 1 — you have the grip. Commit to the apex.", category: "SECTOR 1" },
  { priority: "normal", message: "Norris just set 1:23.6. You need a 1:23.4 for pole. It's there in Sector 3.", category: "TARGET" },
  { priority: "normal", message: "Track temp dropping — 0.2s available in rear traction zones. Use it.", category: "TRACK" },
  { priority: "high", message: "DRS open from detection point. Stay within 1s of car ahead on out-lap for slipstream on main straight.", category: "DRS" },
  { priority: "normal", message: "Battery is full. Deploy everything. No need to harvest. One-shot lap.", category: "ERS" },
];

const Index = () => {
  return (
    <div className="h-screen overflow-hidden bg-background p-3 flex flex-col">
      {/* Header */}
      <header className="flex items-center justify-between mb-3 pb-2 border-b border-border/50 shrink-0">
        <div className="flex items-center gap-3">
          <Flag className="h-4 w-4 text-accent" />
          <h1 className="font-display text-sm md:text-base font-bold tracking-wider text-foreground">
            QUALIFYING — PIT WALL
          </h1>
          <span className="font-display text-[10px] font-bold tracking-wider text-accent bg-accent/10 border border-accent/30 px-2 py-0.5 rounded">
            Q3
          </span>
        </div>
        <div className="flex items-center gap-4 text-[10px] text-muted-foreground uppercase tracking-wider font-display">
          <div className="flex items-center gap-1">
            <Clock className="h-3 w-3 text-warning animate-pulse" />
            <span className="text-warning">02:34 LEFT</span>
          </div>
          <div className="flex items-center gap-1">
            <Gauge className="h-3 w-3 text-primary" />
            <span>P2 — PROVISIONAL</span>
          </div>
          <div className="flex items-center gap-1">
            <Timer className="h-3 w-3 text-primary" />
            <span className="text-primary font-bold">1:23.812</span>
          </div>
        </div>
      </header>

      {/* Telemetry Grid + Alerts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3 mb-3 shrink-0">
        <div className="flex flex-col gap-2">
          <TireTelemetry />
          <AlertBox alerts={tireAlerts} />
        </div>
        <div className="flex flex-col gap-2">
          <EngineTelemetry />
          <AlertBox alerts={engineAlerts} />
        </div>
        <div className="flex flex-col gap-2">
          <AeroTelemetry />
          <AlertBox alerts={aeroAlerts} />
        </div>
      </div>

      {/* Engineer Report */}
      <div className="min-h-0 flex-1 overflow-auto">
        <EngineerReport instructions={instructions} />
      </div>
    </div>
  );
};

export default Index;
