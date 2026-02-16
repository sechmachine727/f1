import { useEffect, useRef } from "react";
import { AlertTriangle, Info, CheckCircle } from "lucide-react";

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

const rowStyles = {
  info: "border-info/20",
  warning: "border-warning/20",
  critical: "border-accent/20 bg-accent/5",
};

export function AlertBox({ alerts }: { alerts: Alert[] }) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [alerts.length]);

  return (
    <div className="bg-card border border-border/50 rounded-md overflow-hidden">
      <div className="flex items-center gap-1.5 px-3 py-2 border-b border-border/50 bg-secondary/30">
        <AlertTriangle className="h-3 w-3 text-muted-foreground" />
        <span className="font-display text-[10px] font-bold tracking-widest uppercase text-muted-foreground">Alerts</span>
        <span className="ml-auto font-display text-[10px] text-muted-foreground">{alerts.length}</span>
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
  );
}
