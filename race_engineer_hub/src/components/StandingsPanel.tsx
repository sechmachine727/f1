import { useEffect, useRef } from "react";
import type { StandingsEntry } from "@/hooks/useTimingData";

function formatGap(ms: number): string {
  if (ms <= 0) return "---";
  const totalSeconds = ms / 1000;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes > 0) {
    return `+${minutes}:${seconds.toFixed(3).padStart(6, "0")}`;
  }
  return `+${seconds.toFixed(3)}`;
}

interface StandingsPanelProps {
  standings: StandingsEntry[];
}

export function StandingsPanel({ standings }: StandingsPanelProps) {
  const playerRef = useRef<HTMLTableRowElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (playerRef.current) {
      playerRef.current.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }, [standings]);

  if (standings.length === 0) {
    return (
      <div className="bg-card/50 border border-border/50 rounded-lg p-3 font-display">
        <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Standings</h3>
        <div className="text-[10px] text-muted-foreground">Waiting for data...</div>
      </div>
    );
  }

  return (
    <div className="bg-card/50 border border-border/50 rounded-lg p-3 font-display flex flex-col min-h-0">
      <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-2">Standings</h3>
      <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto scrollbar-thin">
        <table className="w-full text-[10px] tabular-nums">
          <thead>
            <tr className="text-muted-foreground uppercase tracking-wider">
              <th className="text-left font-medium pb-0.5 w-6">P</th>
              <th className="text-left font-medium pb-0.5">Driver</th>
              <th className="text-right font-medium pb-0.5">Gap</th>
            </tr>
          </thead>
          <tbody>
            {standings.map((entry) => {
              const isRetired = entry.resultStatus === 3 || entry.resultStatus === 4 || entry.resultStatus === 5;
              return (
                <tr
                  key={entry.position}
                  ref={entry.isPlayer ? playerRef : undefined}
                  className={`${
                    entry.isPlayer
                      ? "bg-accent/15 text-accent font-bold"
                      : isRetired
                        ? "text-muted-foreground/40 line-through"
                        : "text-foreground"
                  }`}
                >
                  <td className="text-left py-0.5">{entry.position}</td>
                  <td className="text-left py-0.5">{entry.abbreviation}</td>
                  <td className="text-right py-0.5">
                    {entry.position === 1 ? "LEADER" : formatGap(entry.gapToLeaderMs)}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
