import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Info, CheckCircle, Maximize2 } from "lucide-react";

export interface Alert {
  level: "info" | "warning" | "critical";
  message: string;
  time: string;
}

const icons = {
  info: <Info className="h-3.5 w-3.5 text-info shrink-0" />,
  warning: <AlertTriangle className="h-3.5 w-3.5 text-warning shrink-0" />,
  critical: <AlertTriangle className="h-3.5 w-3.5 text-accent shrink-0" />,
};

const expandedIcons = {
  info: <Info className="h-4 w-4 text-info shrink-0" />,
  warning: <AlertTriangle className="h-4 w-4 text-warning shrink-0" />,
  critical: <AlertTriangle className="h-4 w-4 text-accent shrink-0" />,
};

const rowStyles = {
  info: "border-info/20",
  warning: "border-warning/20",
  critical: "border-accent/20 bg-accent/5",
};

export function AlertBox({ alerts, activeCount }: { alerts: Alert[]; activeCount?: number }) {
  const [expanded, setExpanded] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const expandedScrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
    if (expandedScrollRef.current) {
      expandedScrollRef.current.scrollTop = expandedScrollRef.current.scrollHeight;
    }
  }, [alerts.length]);

  const count = activeCount ?? alerts.length;

  return (
    <>
      <div className="bg-card border border-border/50 rounded-md overflow-hidden">
        <div
          className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30 select-none cursor-pointer"
          onDoubleClick={() => setExpanded(true)}
        >
          <AlertTriangle className="h-3 w-3 text-muted-foreground" />
          <span className="font-display text-[10px] font-bold tracking-widest uppercase text-muted-foreground">Alerts</span>
          <Maximize2 className="h-2.5 w-2.5 text-muted-foreground/50 ml-1" />
          <span className="ml-auto font-display text-[10px] text-muted-foreground">{count}</span>
        </div>
        <div ref={scrollRef} className="h-36 overflow-y-auto">
          {alerts.map((a, i) => (
            <div key={i} className={`flex items-start gap-2 px-3 py-2 border-b last:border-b-0 ${rowStyles[a.level]}`}>
              {icons[a.level]}
              <span className="text-[11px] text-card-foreground leading-tight flex-1">{a.message}</span>
              <span className="text-[9px] text-muted-foreground tracking-wider shrink-0">{a.time}</span>
            </div>
          ))}
          {alerts.length === 0 && (
            <div className="flex items-center gap-2 px-3 py-3">
              <CheckCircle className="h-3.5 w-3.5 text-primary" />
              <span className="text-[11px] text-muted-foreground">No alerts</span>
            </div>
          )}
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
              <AlertTriangle className="h-3 w-3 text-muted-foreground" />
              <span className="font-display text-[10px] font-bold tracking-widest uppercase text-muted-foreground">Alerts</span>
              <span className="ml-auto font-display text-[10px] text-muted-foreground">{count}</span>
            </div>
            <div ref={expandedScrollRef} className="max-h-[70vh] overflow-y-auto">
              {alerts.map((a, i) => (
                <div key={i} className={`flex items-start gap-2 px-4 py-3 border-b last:border-b-0 ${rowStyles[a.level]}`}>
                  {expandedIcons[a.level]}
                  <span className="text-sm text-card-foreground leading-relaxed flex-1">{a.message}</span>
                  <span className="text-[10px] text-muted-foreground tracking-wider shrink-0">{a.time}</span>
                </div>
              ))}
              {alerts.length === 0 && (
                <div className="flex items-center gap-2 px-4 py-4">
                  <CheckCircle className="h-4 w-4 text-primary" />
                  <span className="text-sm text-muted-foreground">No alerts</span>
                </div>
              )}
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
