import { ExpandableReportPanel } from "./ExpandableReportPanel";
import type { TiresReport } from "@/hooks/useTireTelemetry";

export function TiresReportPanel({ report }: { report: TiresReport }) {
  return (
    <ExpandableReportPanel
      title="Tires Engineer"
      responses={report.responses}
      emptyMessage="Awaiting tire analysis..."
    />
  );
}
