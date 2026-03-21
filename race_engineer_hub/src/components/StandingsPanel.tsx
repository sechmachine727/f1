import { useEffect, useRef, useState } from "react";
import { Trophy, Maximize2 } from "lucide-react";
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
  sessionType?: string;
  className?: string;
}

export function StandingsPanel({ standings, sessionType, className }: StandingsPanelProps) {
  const isRaceSession = sessionType ? /RACE|SPRINT/.test(sessionType) : true;
  const title = "Standings";
  const [expanded, setExpanded] = useState(false);
  const playerRef = useRef<HTMLTableRowElement>(null);
  const expandedPlayerRef = useRef<HTMLTableRowElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (playerRef.current) {
      playerRef.current.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }, [standings]);

  useEffect(() => {
    if (expanded && expandedPlayerRef.current) {
      expandedPlayerRef.current.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }, [expanded, standings]);

  const renderTable = (isExpanded: boolean) => {
    if (standings.length === 0) {
      return <div className={`${isExpanded ? "text-sm" : "text-[10px]"} text-muted-foreground px-3 py-3`}>Waiting for data...</div>;
    }

    return (
      <table className={`w-full ${isExpanded ? "text-sm" : "text-[10px]"} tabular-nums`}>
        <tbody>
          {standings.map((entry) => {
            const isRetired = entry.resultStatus === 3 || entry.resultStatus === 4 || entry.resultStatus === 5;
            return (
              <tr
                key={entry.position}
                ref={entry.isPlayer ? (isExpanded ? expandedPlayerRef : playerRef) : undefined}
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
                  {entry.position === 1 ? formatLapTime(entry.lastLapTimeMs) : formatGap(isRaceSession ? entry.gapToFrontMs : entry.gapToLeaderMs)}
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
    );
  };

  return (
    <>
      <div className={`bg-card border border-border/50 rounded-md overflow-hidden flex flex-col min-h-0 ${className ?? ""}`}>
        <div
          className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30 select-none cursor-pointer shrink-0"
          onDoubleClick={() => setExpanded(true)}
        >
          <Trophy className="h-4 w-4 text-primary" />
          <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">{title}</span>
          <Maximize2 className="h-2.5 w-2.5 text-muted-foreground/50 ml-1" />
        </div>
        <div ref={scrollRef} className="flex-1 min-h-0 overflow-y-auto scrollbar-thin font-display">
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
              <Trophy className="h-4 w-4 text-primary" />
              <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">{title}</span>
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
