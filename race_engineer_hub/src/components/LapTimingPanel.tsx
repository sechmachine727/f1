import type { TimingData } from "@/hooks/useTimingData";
import { formatSectorTime, getSectorColor } from "@/hooks/useTimingData";
import { formatLapTime } from "@/hooks/useSessionTelemetry";

const SECTOR_COLOR_MAP: Record<string, string> = {
  purple: "text-purple-400",
  green: "text-green-400",
  yellow: "text-yellow-400",
  white: "text-muted-foreground",
};

interface LapTimingPanelProps {
  timing: TimingData | null;
  lapCompleted: boolean;
  className?: string;
}

export function LapTimingPanel({ timing, lapCompleted, className }: LapTimingPanelProps) {
  if (!timing) {
    return (
      <div className={`bg-card/50 border border-border/50 rounded-lg p-3 font-display flex flex-col min-h-0 ${className ?? ""}`}>
        <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Lap Timing</h3>
        <div className="text-[10px] text-muted-foreground">Waiting for data...</div>
      </div>
    );
  }

  const { personalBest, overallBest, lapHistory } = timing;

  const s1Color = timing.sector1Ms > 0 ? getSectorColor(timing.sector1Ms, personalBest.s1Ms, overallBest.s1Ms) : "white";
  const s2Color = timing.sector2Ms > 0 ? getSectorColor(timing.sector2Ms, personalBest.s2Ms, overallBest.s2Ms) : "white";

  const bestLapMs = personalBest.lapMs;

  return (
    <div className={`bg-card/50 border border-border/50 rounded-lg p-3 font-display flex flex-col min-h-0 ${className ?? ""}`}>
      <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Lap Timing</h3>

      <div className="flex-1 min-h-0 overflow-y-auto scrollbar-thin">
        <table className="w-full text-[10px] tabular-nums table-fixed">
          <colgroup>
            <col className="w-6" />
            <col className="w-[4.5rem]" />
            <col className="w-[4.5rem]" />
            <col className="w-[4.5rem]" />
            <col />
          </colgroup>
          <tbody>
            {/* Current lap */}
            <tr
              className={`${
                lapCompleted ? "bg-green-500/15 text-green-400" : timing.currentLapInvalid ? "text-red-400/60" : "bg-accent/10 text-foreground"
              }`}
            >
              <td className="text-left py-0.5 font-bold">{timing.currentLap}</td>
              <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[s1Color]}`}>{formatSectorTime(timing.sector1Ms)}</td>
              <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[s2Color]}`}>{formatSectorTime(timing.sector2Ms)}</td>
              <td className="text-right py-0.5 text-muted-foreground">{formatSectorTime(0)}</td>
              <td className={`text-right py-0.5 font-bold ${lapCompleted ? "animate-pulse" : ""}`}>{formatLapTime(timing.currentLapTimeMs)}</td>
            </tr>

            {/* Completed laps */}
            {lapHistory.map((lap) => {
              const isBest = bestLapMs > 0 && lap.lapTimeMs === bestLapMs && lap.valid;
              return (
                <tr
                  key={lap.lapNum}
                  className={`${
                    isBest ? "bg-purple-500/15 text-purple-300" : lap.valid ? "text-foreground" : "text-muted-foreground/50"
                  }`}
                >
                  <td className="text-left py-0.5">{lap.lapNum}</td>
                  <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[lap.s1Color ?? getSectorColor(lap.s1Ms, personalBest.s1Ms, overallBest.s1Ms)]}`}>
                    {formatSectorTime(lap.s1Ms)}
                  </td>
                  <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[lap.s2Color ?? getSectorColor(lap.s2Ms, personalBest.s2Ms, overallBest.s2Ms)]}`}>
                    {formatSectorTime(lap.s2Ms)}
                  </td>
                  <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[lap.s3Color ?? getSectorColor(lap.s3Ms, personalBest.s3Ms, overallBest.s3Ms)]}`}>
                    {formatSectorTime(lap.s3Ms)}
                  </td>
                  <td className="text-right py-0.5 font-bold">{formatLapTime(lap.lapTimeMs)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
