import { useState } from "react";
import { Timer, Maximize2 } from "lucide-react";
import type { TimingData } from "@/hooks/useTimingData";
import { formatSectorTime, getSectorColor } from "@/hooks/useTimingData";
import { formatLapTime } from "@/hooks/useSessionTelemetry";

const SECTOR_COLOR_MAP: Record<string, string> = {
  purple: "text-purple-400",
  green: "text-green-400",
  yellow: "text-yellow-400",
  white: "text-muted-foreground",
};

function formatDelta(deltaMs: number | undefined): string {
  if (deltaMs === undefined) return "";
  const sign = deltaMs < 0 ? "-" : "+";
  const abs = Math.abs(deltaMs) / 1000;
  return `${sign}${abs.toFixed(3)}`;
}

function deltaColor(deltaMs: number | undefined): string {
  if (deltaMs === undefined) return "text-muted-foreground";
  return deltaMs < 0 ? "text-green-400" : deltaMs > 0 ? "text-red-400" : "text-muted-foreground";
}

interface LapTimingPanelProps {
  timing: TimingData | null;
  lapCompleted: boolean;
  className?: string;
}

export function LapTimingPanel({ timing, lapCompleted, className }: LapTimingPanelProps) {
  const [expanded, setExpanded] = useState(false);

  const renderTable = (isExpanded: boolean) => {
    if (!timing) {
      return <div className={`${isExpanded ? "text-sm" : "text-[10px]"} text-muted-foreground px-3 py-3`}>Waiting for data...</div>;
    }

    const { personalBest, overallBest, lapHistory } = timing;
    const s1Color = timing.sector1Ms > 0 ? getSectorColor(timing.sector1Ms, personalBest.s1Ms, overallBest.s1Ms) : "white";
    const s2Color = timing.sector2Ms > 0 ? getSectorColor(timing.sector2Ms, personalBest.s2Ms, overallBest.s2Ms) : "white";
    const bestLapMs = personalBest.lapMs;

    return (
      <table className={`${isExpanded ? "text-sm" : "text-[10px]"} tabular-nums`} style={{ borderSpacing: "10px 0", borderCollapse: "separate" }}>
        <tbody>
          <tr
            className={`${
              lapCompleted ? "bg-green-500/15 text-green-400" : timing.currentLapInvalid ? "text-red-400/60" : "bg-accent/10 text-foreground"
            }`}
          >
            <td className="text-left py-px font-bold">{timing.currentLap}</td>
            <td className={`text-right py-px min-w-[3.5rem] ${SECTOR_COLOR_MAP[s1Color]}`}>{formatSectorTime(timing.sector1Ms)}</td>
            <td className={`text-right py-px min-w-[3.5rem] ${SECTOR_COLOR_MAP[s2Color]}`}>{formatSectorTime(timing.sector2Ms)}</td>
            <td className="text-right py-px min-w-[3.5rem] text-muted-foreground">{formatSectorTime(0)}</td>
            <td className={`text-right py-px min-w-[4rem] font-bold ${lapCompleted ? "animate-pulse" : ""}`}>{formatLapTime(timing.currentLapTimeMs)}</td>
            <td className="text-right py-px min-w-[3.5rem] text-muted-foreground"></td>
          </tr>

          {lapHistory.map((lap) => {
            const isBest = bestLapMs > 0 && lap.lapTimeMs === bestLapMs && lap.valid;
            return (
              <tr
                key={lap.lapNum}
                className={`${
                  isBest ? "bg-purple-500/15 text-purple-300" : lap.valid ? "text-foreground" : "text-muted-foreground/50"
                }`}
              >
                <td className="text-left py-px">{lap.lapNum}</td>
                <td className={`text-right py-px min-w-[3.5rem] ${SECTOR_COLOR_MAP[lap.s1Color ?? getSectorColor(lap.s1Ms, personalBest.s1Ms, overallBest.s1Ms)]}`}>
                  {formatSectorTime(lap.s1Ms)}
                </td>
                <td className={`text-right py-px min-w-[3.5rem] ${SECTOR_COLOR_MAP[lap.s2Color ?? getSectorColor(lap.s2Ms, personalBest.s2Ms, overallBest.s2Ms)]}`}>
                  {formatSectorTime(lap.s2Ms)}
                </td>
                <td className={`text-right py-px min-w-[3.5rem] ${SECTOR_COLOR_MAP[lap.s3Color ?? getSectorColor(lap.s3Ms, personalBest.s3Ms, overallBest.s3Ms)]}`}>
                  {formatSectorTime(lap.s3Ms)}
                </td>
                <td className="text-right py-px min-w-[4rem] font-bold">{formatLapTime(lap.lapTimeMs)}</td>
                <td className={`text-right py-px min-w-[3.5rem] ${deltaColor(lap.deltaMs)}`}>{formatDelta(lap.deltaMs)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    );
  };

  return (
    <>
      <div className={`bg-card border border-border/50 rounded-md overflow-hidden flex flex-col min-h-0 ${className ?? ""}`}>
        <div
          className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30 select-none cursor-pointer shrink-0"
          onDoubleClick={() => setExpanded(true)}
        >
          <Timer className="h-4 w-4 text-primary" />
          <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">Lap Timing</span>
          <Maximize2 className="h-2.5 w-2.5 text-muted-foreground/50 ml-1" />
        </div>
        <div className="flex-1 min-h-0 overflow-y-auto scrollbar-thin font-display">
          {renderTable(false)}
        </div>
      </div>

      {expanded && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => setExpanded(false)}
        >
          <div
            className="bg-card border border-border/50 rounded-md overflow-hidden w-[90vw] max-w-3xl shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30">
              <Timer className="h-4 w-4 text-primary" />
              <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">Lap Timing</span>
            </div>
            <div className="max-h-[70vh] overflow-y-auto font-display">
              {renderTable(true)}
            </div>
            <div
              className="px-4 py-2 border-t border-border/50 bg-secondary/30 text-center cursor-pointer"
              onClick={() => setExpanded(false)}
            >
              <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-display">
                Click to close
              </span>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
