import { useEffect, useRef } from "react";
import { Bot, Loader2 } from "lucide-react";
import type { PuReport } from "@/hooks/usePowerUnitTelemetry";

export function PuReportPanel({ report }: { report: PuReport }) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [report.responses.length]);

  return (
    <div className="bg-card border border-border/50 rounded-md overflow-hidden">
      <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30">
        <Bot className="h-3 w-3 text-muted-foreground" />
        <span className="font-display text-[10px] font-bold tracking-widest uppercase text-muted-foreground">
          Power Unit Engineer
        </span>
        <span className="ml-auto font-display text-[10px] text-muted-foreground">{report.responses.length}</span>
      </div>
      <div ref={scrollRef} className="h-36 overflow-y-auto">
        {report.responses.map((r, i) => (
          <div key={i} className="flex items-start gap-2 px-3 py-2 border-b last:border-b-0 border-border/20">
            <Bot className="h-3.5 w-3.5 text-info shrink-0 mt-0.5" />
            <span className="text-[11px] text-card-foreground leading-tight">{r}</span>
          </div>
        ))}
        {report.responses.length === 0 && (
          <div className="flex items-center gap-2 px-3 py-3">
            <Loader2 className="h-3.5 w-3.5 text-muted-foreground animate-spin" />
            <span className="text-[11px] text-muted-foreground">Awaiting power unit analysis...</span>
          </div>
        )}
      </div>
    </div>
  );
}
