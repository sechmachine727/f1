import { ExpandableReportPanel } from "./ExpandableReportPanel";
import type { RaceEngineerReport } from "@/hooks/useRaceEngineerReport";

export function RaceEngineerPanel({ report }: { report: RaceEngineerReport }) {
  return (
    <ExpandableReportPanel
      title="Race Engineer"
      responses={report.responses}
      emptyMessage="Awaiting engineer reports..."
    />
  );
}
