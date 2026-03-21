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

function formatLapTime(ms: number): string {
  if (ms <= 0) return "---";
  const totalSeconds = ms / 1000;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toFixed(3).padStart(6, "0")}`;
}

const COMPOUND_STYLES: Record<string, { letter: string; color: string }> = {
  soft: { letter: "S", color: "#FF3333" },
  medium: { letter: "M", color: "#FFD700" },
  hard: { letter: "H", color: "#FFFFFF" },
  inter: { letter: "I", color: "#39B54A" },
  wet: { letter: "W", color: "#0080FF" },
};

interface StandingsPanelProps {
  standings: StandingsEntry[];
  className?: string;
}

export function StandingsPanel({ standings, className }: StandingsPanelProps) {
  const playerRef = useRef<HTMLTableRowElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (playerRef.current) {
      playerRef.current.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }, [standings]);

  if (standings.length === 0) {
    return (
      <div className="bg-card/50 border border-border/50 rounded-lg px-2 py-1.5 font-display">
        <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-1">Standings</h3>
        <div className="text-[10px] text-muted-foreground">Waiting for data...</div>
      </div>
    );
  }

  return (
    <div className={`bg-card/50 border border-border/50 rounded-lg px-2 py-1.5 font-display flex flex-col min-h-0 ${className ?? ""}`}>
      <h3 className="text-[10px] font-bold tracking-wider text-muted-foreground uppercase mb-1">Standings</h3>
      <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto scrollbar-thin">
        <table className="w-full text-[10px] tabular-nums">
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
                  <td className="text-left py-px">{entry.position}</td>
                  <td className="text-left py-px">{entry.abbreviation}</td>
                  <td className="text-right py-px">
                    {entry.position === 1 ? formatLapTime(entry.lastLapTimeMs) : formatGap(entry.gapToFrontMs)}
                  </td>
                  <td className="text-center py-px w-4">
                    {COMPOUND_STYLES[entry.visualCompound] ? (
                      <span style={{ color: COMPOUND_STYLES[entry.visualCompound].color }}>
                        {COMPOUND_STYLES[entry.visualCompound].letter}
                      </span>
                    ) : null}
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
