import { Bot, Loader2 } from "lucide-react";
import type { DamageReport } from "@/hooks/useAeroTelemetry";

export function DamageReportPanel({ report }: { report: DamageReport }) {
  return (
    <div className="bg-card border border-border/50 rounded-md overflow-hidden">
      <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30">
        <Bot className="h-3 w-3 text-muted-foreground" />
        <span className="font-display text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
          Damage Engineer
        </span>
      </div>
      <div className="h-36 overflow-y-auto px-3 py-2">
        {report.response ? (
          <p className="text-[11px] text-card-foreground leading-tight whitespace-pre-wrap">
            {report.response}
          </p>
        ) : (
          <div className="flex items-center gap-2 py-1">
            <Loader2 className="h-3.5 w-3.5 text-muted-foreground animate-spin" />
            <span className="text-[11px] text-muted-foreground">
              Awaiting damage analysis...
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
