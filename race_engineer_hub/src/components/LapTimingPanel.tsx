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
}

export function LapTimingPanel({ timing, lapCompleted }: LapTimingPanelProps) {
  if (!timing) {
    return (
      <div className="bg-card/50 border border-border/50 rounded-lg p-3 font-display">
        <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Lap Timing</h3>
        <div className="text-[10px] text-muted-foreground">Waiting for data...</div>
      </div>
    );
  }

  const { personalBest, overallBest, lapHistory } = timing;

  // Get S3 from the most recent completed lap in history (packet 11)
  const lastCompletedLap = lapHistory.length > 0 ? lapHistory[0] : null;

  // Determine which sectors are completed this lap
  const s1Done = timing.sector1Ms > 0;
  const s2Done = timing.sector2Ms > 0;

  // S3 display: show from last completed lap if we're on the next lap (sector 0 or 1)
  const showLastS3 = lastCompletedLap && lastCompletedLap.lapNum === timing.currentLap - 1;
  const s3Ms = showLastS3 ? lastCompletedLap.s3Ms : 0;

  const s1Color = s1Done ? getSectorColor(timing.sector1Ms, personalBest.s1Ms, overallBest.s1Ms) : "white";
  const s2Color = s2Done ? getSectorColor(timing.sector2Ms, personalBest.s2Ms, overallBest.s2Ms) : "white";
  const s3Color = s3Ms > 0 ? getSectorColor(s3Ms, personalBest.s3Ms, overallBest.s3Ms) : "white";

  const bestLapMs = personalBest.lapMs;

  return (
    <div className="bg-card/50 border border-border/50 rounded-lg p-3 font-display flex flex-col min-h-0">
      <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Lap Timing</h3>

      {/* Current lap time */}
      <div className="flex items-center justify-between mb-2">
        <span className="text-[10px] text-muted-foreground uppercase tracking-wider">
          Lap {timing.currentLap}
        </span>
        <span
          className={`text-sm font-bold tabular-nums ${
            lapCompleted ? "text-green-400 animate-pulse" : timing.currentLapInvalid ? "text-red-400/60" : "text-foreground"
          }`}
        >
          {formatLapTime(timing.currentLapTimeMs)}
        </span>
      </div>

      {/* Sectors */}
      <div className="grid grid-cols-3 gap-1.5 mb-3">
        <SectorBox label="S1" timeMs={timing.sector1Ms} colorClass={SECTOR_COLOR_MAP[s1Color]} active={timing.currentSector === 0} />
        <SectorBox label="S2" timeMs={timing.sector2Ms} colorClass={SECTOR_COLOR_MAP[s2Color]} active={timing.currentSector === 1} />
        <SectorBox label="S3" timeMs={s3Ms} colorClass={SECTOR_COLOR_MAP[s3Color]} active={timing.currentSector === 2} />
      </div>

      {/* Lap history */}
      <div className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-1">History</div>
      <div className="flex-1 min-h-0 overflow-y-auto scrollbar-thin">
        {lapHistory.length === 0 ? (
          <div className="text-[10px] text-muted-foreground">No completed laps</div>
        ) : (
          <table className="w-full text-[10px] tabular-nums">
            <thead>
              <tr className="text-muted-foreground uppercase tracking-wider">
                <th className="text-left font-medium pb-0.5 w-8">#</th>
                <th className="text-right font-medium pb-0.5">Time</th>
                <th className="text-right font-medium pb-0.5">S1</th>
                <th className="text-right font-medium pb-0.5">S2</th>
                <th className="text-right font-medium pb-0.5">S3</th>
              </tr>
            </thead>
            <tbody>
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
                    <td className="text-right py-0.5">{formatLapTime(lap.lapTimeMs)}</td>
                    <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[getSectorColor(lap.s1Ms, personalBest.s1Ms, overallBest.s1Ms)]}`}>
                      {formatSectorTime(lap.s1Ms)}
                    </td>
                    <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[getSectorColor(lap.s2Ms, personalBest.s2Ms, overallBest.s2Ms)]}`}>
                      {formatSectorTime(lap.s2Ms)}
                    </td>
                    <td className={`text-right py-0.5 ${SECTOR_COLOR_MAP[getSectorColor(lap.s3Ms, personalBest.s3Ms, overallBest.s3Ms)]}`}>
                      {formatSectorTime(lap.s3Ms)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function SectorBox({
  label,
  timeMs,
  colorClass,
  active,
}: {
  label: string;
  timeMs: number;
  colorClass: string;
  active: boolean;
}) {
  return (
    <div className={`bg-background/50 border rounded px-1.5 py-1 text-center ${active ? "border-accent/50" : "border-border/30"}`}>
      <div className="text-[9px] text-muted-foreground uppercase tracking-wider mb-0.5">{label}</div>
      <div className={`text-[11px] font-bold tabular-nums ${colorClass}`}>
        {formatSectorTime(timeMs)}
      </div>
    </div>
  );
}
