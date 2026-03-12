import { useState } from "react";
import { Radio } from "lucide-react";
import { ExpandableReportPanel } from "./ExpandableReportPanel";
import type { ReportEntry } from "./ExpandableReportPanel";
import type { RaceEngineerReport } from "@/hooks/useRaceEngineerReport";

interface RaceEngineerPanelProps {
  report: RaceEngineerReport;
  driverMessages: ReportEntry[];
  className?: string;
}

export function RaceEngineerPanel({ report, driverMessages, className }: RaceEngineerPanelProps) {
  const [driverOnly, setDriverOnly] = useState(false);

  const merged: ReportEntry[] = [
    ...driverMessages,
    ...report.alerts
      .filter((a) => !a.message.startsWith("From Fernando:"))
      .map((a) => ({ text: a.message, time: a.time, source: "alert" as const })),
    ...report.responses.map((r) => ({
      ...r,
      source: r.text.startsWith("To Fernando:") ? "radio" as const : "engineer" as const,
    })),
  ].sort((a, b) => a.time.localeCompare(b.time));

  const filtered = driverOnly
    ? merged.filter((r) => r.source === "driver" || r.source === "radio")
    : merged;

  const filterButton = (
    <button
      onClick={(e) => { e.stopPropagation(); setDriverOnly((v) => !v); }}
      className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-display font-bold tracking-wider uppercase transition-colors ${
        driverOnly
          ? "bg-accent/20 text-accent border border-accent/40"
          : "text-muted-foreground hover:text-foreground border border-transparent"
      }`}
      title="Show only driver radio messages"
    >
      <Radio className="h-3 w-3" />
      Radio
    </button>
  );

  return (
    <ExpandableReportPanel
      title="Race Engineer Agent"
      responses={filtered}
      emptyMessage={driverOnly ? "No driver radio messages yet..." : "Awaiting engineer reports..."}
      className={className}
      headerExtra={filterButton}
    />
  );
}
