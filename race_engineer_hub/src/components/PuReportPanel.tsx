import { ExpandableReportPanel } from "./ExpandableReportPanel";
import type { PuReport } from "@/hooks/usePowerUnitTelemetry";

export function PuReportPanel({ report }: { report: PuReport }) {
  return (
    <ExpandableReportPanel
      title="Power Unit Engineer"
      responses={report.responses}
      emptyMessage="Awaiting power unit analysis..."
    />
  );
}
