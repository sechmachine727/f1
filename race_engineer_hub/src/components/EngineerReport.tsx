import { Radio, ArrowRight } from "lucide-react";

export interface Instruction {
  priority: "high" | "normal";
  message: string;
  category: string;
}

export function EngineerReport({ instructions }: { instructions: Instruction[] }) {
  return (
    <div className="bg-card border border-primary/20 rounded-md glow-primary overflow-hidden telemetry-grid">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-border/50 bg-primary/5">
        <Radio className="h-4 w-4 text-primary animate-pulse" />
        <h2 className="font-display text-sm font-bold tracking-widest uppercase text-foreground">
          Qualifying Engineer Report — Driver Comms
        </h2>
      </div>
      <div className="p-4 space-y-2">
        {instructions.map((inst, i) => (
          <div
            key={i}
            className={`flex items-start gap-3 px-3 py-2.5 rounded border ${
              inst.priority === "high"
                ? "border-accent/30 bg-accent/5"
                : "border-border/50 bg-secondary/30"
            }`}
          >
            <ArrowRight className={`h-3.5 w-3.5 mt-0.5 shrink-0 ${inst.priority === "high" ? "text-accent" : "text-primary"}`} />
            <div className="flex-1">
              <span className="text-[11px] text-card-foreground leading-snug">{inst.message}</span>
            </div>
            <span className={`font-display text-[9px] tracking-wider shrink-0 uppercase ${
              inst.priority === "high" ? "text-accent" : "text-muted-foreground"
            }`}>
              {inst.category}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
