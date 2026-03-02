import { ExpandableReportPanel } from "./ExpandableReportPanel";
import type { DamageReport } from "@/hooks/useAeroTelemetry";

export function DamageReportPanel({ report }: { report: DamageReport }) {
  return (
    <ExpandableReportPanel
      title="Damage Engineer"
      responses={report.responses}
      emptyMessage="Awaiting damage analysis..."
    />
  );
}
