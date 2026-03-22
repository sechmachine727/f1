import React from "react";
import { useAutoScroll } from "@/hooks/useAutoScroll";
import type { SessionData, WeatherForecastSample } from "@/hooks/useSessionTelemetry";
import type { LapWearRecord, TyreTelemetryData, TyresReportEntry } from "@/hooks/useTyreTelemetry";
import type { Alert } from "@/components/AlertBox";
import { BarGauge } from "./TelemetryCard";
import ReactMarkdown from "react-markdown";
import {
  AlertTriangle,
  ArrowDownToLine,
  Bot,
  CheckCircle,
  Circle,
  CloudSun,
  Droplets,
  Info,
  Thermometer,
  TrendingDown,
} from "lucide-react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ReferenceLine,
  ResponsiveContainer,
} from "recharts";

// ── Constants ──────────────────────────────────────────────────────────────

const POSITIONS = [
  { key: "fl" as const, label: "FL" },
  { key: "fr" as const, label: "FR" },
  { key: "rl" as const, label: "RL" },
  { key: "rr" as const, label: "RR" },
];

const COMPOUND_COLORS: Record<string, string> = {
  soft: "#FF3333",
  medium: "#FFC906",
  hard: "#FFFFFF",
  inter: "#39B54A",
  wet: "#0072CE",
};

const WHEEL_COLORS = {
  fl: "#FF3333",
  fr: "#3B82F6",
  rl: "#22C55E",
  rr: "#F97316",
};

const WEATHER_LABELS: Record<number, string> = {
  0: "Clear",
  1: "Light Cloud",
  2: "Overcast",
  3: "Light Rain",
  4: "Heavy Rain",
  5: "Storm",
};

const TEMP_CHANGE_LABELS: Record<number, string> = {
  0: "\u2191",
  1: "\u2193",
  2: "\u2192",
};

// ── Alert rendering (matching TyreAlertPanel) ──────────────────────────────

type UnifiedItem =
  | { kind: "alert"; level: Alert["level"]; message: string; time: string; sortKey: string }
  | { kind: "engineer"; text: string; time: string; sortKey: string };

function extractSortKey(time: string): string {
  return time.slice(0, 5);
}

function mergeItems(alerts: Alert[], responses: TyresReportEntry[]): UnifiedItem[] {
  const alertItems: UnifiedItem[] = alerts.map((a) => ({
    kind: "alert" as const,
    level: a.level,
    message: a.message,
    time: a.time,
    sortKey: extractSortKey(a.time),
  }));
  const engineerItems: UnifiedItem[] = responses.map((r) => ({
    kind: "engineer" as const,
    text: r.text,
    time: r.time,
    sortKey: extractSortKey(r.time),
  }));

  const result: UnifiedItem[] = [];
  let ai = 0;
  let ei = 0;
  while (ai < alertItems.length && ei < engineerItems.length) {
    if (alertItems[ai].sortKey <= engineerItems[ei].sortKey) {
      result.push(alertItems[ai++]);
    } else {
      result.push(engineerItems[ei++]);
    }
  }
  while (ai < alertItems.length) result.push(alertItems[ai++]);
  while (ei < engineerItems.length) result.push(engineerItems[ei++]);
  return result;
}

const alertIcons = {
  info: <Info className="h-4 w-4 text-info shrink-0" />,
  warning: <AlertTriangle className="h-4 w-4 text-warning shrink-0" />,
  critical: <AlertTriangle className="h-4 w-4 text-accent shrink-0" />,
  pit: <ArrowDownToLine className="h-4 w-4 text-primary shrink-0" />,
};

const rowStyles = {
  info: "border-info/20",
  warning: "border-warning/20",
  critical: "border-accent/20 bg-accent/5",
  pit: "border-primary/20 bg-primary/5",
};

// ── Sub-panel wrapper ──────────────────────────────────────────────────────

function Panel({ title, icon, children, className = "" }: {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`bg-card border border-border/50 rounded-md overflow-hidden flex flex-col ${className}`}>
      <div className="flex items-center gap-1.5 px-3 py-1.5 border-b border-border/50 bg-secondary/30 shrink-0">
        {icon}
        <span className="font-display text-xs font-bold tracking-widest uppercase text-foreground">{title}</span>
      </div>
      <div className="flex-1 overflow-y-auto p-2">
        {children}
      </div>
    </div>
  );
}

// ── Sub-panel 1: Tyres Telemetry ───────────────────────────────────────────

function TyresTelemetryPanel({ data }: { data: TyreTelemetryData }) {
  const compoundText = data.compound
    ? `${data.compoundVisual.toUpperCase()} (${data.compound})`
    : "\u2014";
  const compoundColor = COMPOUND_COLORS[data.compoundVisual] ?? undefined;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between text-[10px] text-muted-foreground uppercase tracking-wider">
        <span style={compoundColor ? { color: compoundColor } : undefined} className="font-bold">
          {compoundText}
        </span>
        <span>Age: {data.tyresAgeLaps} laps</span>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {POSITIONS.map((p) => {
          const t = data.tyres[p.key];
          const life = Math.max(0, Math.round(100 - t.wear));
          return (
            <div key={p.key} className="bg-secondary/50 rounded p-2 flex flex-col gap-1 border border-border/50">
              <div className="flex items-center justify-between">
                <span className="font-display text-[10px] font-bold tracking-wider text-foreground">{p.label}</span>
                <span className={`font-display text-sm font-bold ${t.surfaceTemp > 108 ? "text-accent" : t.surfaceTemp > 103 ? "text-warning" : "text-primary"}`}>
                  {t.surfaceTemp}°
                </span>
              </div>
              <div className="grid grid-cols-2 gap-x-3 text-[9px] text-muted-foreground">
                <span>Inner: <span className="text-foreground">{t.innerTemp}°C</span></span>
                <span>Pressure: <span className="text-foreground">{t.pressure.toFixed(1)} bar</span></span>
                <span>Brake: <span className={`${t.brakeTemp > 900 ? "text-accent" : t.brakeTemp > 700 ? "text-warning" : "text-foreground"}`}>{t.brakeTemp}°C</span></span>
              </div>
              <BarGauge value={life} max={100} label="Life" warn={40} critical={25} invertThresholds />
              <BarGauge value={t.damage} max={255} label="Damage" warn={50} critical={150} />
              <BarGauge value={t.blisters} max={255} label="Blisters" warn={50} critical={150} />
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Sub-panel 2: Current Weather ───────────────────────────────────────────

function CurrentWeatherPanel({ session }: { session: SessionData }) {
  return (
    <div className="flex flex-col gap-3 py-1">
      <div className="flex items-center gap-2">
        <CloudSun className="h-5 w-5 text-primary" />
        <span className="font-display text-base font-bold text-foreground">
          {WEATHER_LABELS[session.weather] ?? "Unknown"}
        </span>
      </div>
      <div className="grid grid-cols-2 gap-2 text-sm">
        <div className="flex items-center gap-1.5">
          <Thermometer className="h-4 w-4 text-primary" />
          <span className="text-muted-foreground">Air</span>
          <span className="font-display font-bold text-foreground">{session.airTemp}°C</span>
        </div>
        <div className="flex items-center gap-1.5">
          <Thermometer className="h-4 w-4 text-warning" />
          <span className="text-muted-foreground">Track</span>
          <span className="font-display font-bold text-foreground">{session.trackTemp}°C</span>
        </div>
      </div>
    </div>
  );
}

// ── Sub-panel 3: Weather Forecast ──────────────────────────────────────────

function WeatherForecastPanel({ forecast }: { forecast: WeatherForecastSample[] }) {
  if (forecast.length === 0) {
    return <span className="text-[11px] text-muted-foreground">No forecast available</span>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[10px]">
        <thead>
          <tr className="text-muted-foreground uppercase tracking-wider border-b border-border/30">
            <th className="text-left py-1 px-1 font-display">Offset</th>
            <th className="text-left py-1 px-1 font-display">Weather</th>
            <th className="text-right py-1 px-1 font-display">Track</th>
            <th className="text-right py-1 px-1 font-display">Air</th>
            <th className="text-right py-1 px-1 font-display">Rain</th>
          </tr>
        </thead>
        <tbody>
          {forecast.map((fc, i) => {
            const rainColor = fc.rainPercentage > 50 ? "text-accent" : fc.rainPercentage > 20 ? "text-warning" : "text-primary";
            return (
              <tr key={i} className="border-b border-border/20">
                <td className="py-1 px-1 text-muted-foreground">+{fc.timeOffset}m</td>
                <td className="py-1 px-1 text-foreground">{WEATHER_LABELS[fc.weather] ?? "?"}</td>
                <td className="py-1 px-1 text-right text-foreground">
                  {fc.trackTemperature}°C {TEMP_CHANGE_LABELS[fc.trackTemperatureChange] ?? ""}
                </td>
                <td className="py-1 px-1 text-right text-foreground">
                  {fc.airTemperature}°C {TEMP_CHANGE_LABELS[fc.airTemperatureChange] ?? ""}
                </td>
                <td className={`py-1 px-1 text-right font-bold ${rainColor}`}>
                  {fc.rainPercentage > 0 ? `${fc.rainPercentage}%` : "\u2014"}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── Sub-panel 4: Historical Tyre Wear Chart ────────────────────────────────

function HistoricalWearChart({ wearHistory, tyreData }: { wearHistory: LapWearRecord[]; tyreData: TyreTelemetryData | null }) {
  // Compute sector fractional offsets within each lap
  const sectorLines = React.useMemo(() => {
    if (!tyreData || !wearHistory.length) return [];
    const trackLength = tyreData.trackLength;
    if (trackLength <= 0) return [];

    const s2Frac = tyreData.sectorBoundaries.sector2Start / trackLength;
    const s3Frac = tyreData.sectorBoundaries.sector3Start / trackLength;

    // Generate sector reference lines for each completed lap in the data
    const firstLap = Math.floor(wearHistory[0].lap);
    const lastLap = Math.floor(wearHistory[wearHistory.length - 1].lap);
    const lines: { x: number; label: string }[] = [];
    for (let lap = firstLap; lap <= lastLap; lap++) {
      lines.push({ x: Math.round((lap + s2Frac) * 100) / 100, label: "S2" });
      lines.push({ x: Math.round((lap + s3Frac) * 100) / 100, label: "S3" });
    }
    return lines;
  }, [tyreData, wearHistory]);

  // Custom x-axis tick: show integer laps as "Lap N", skip fractional
  const formatXTick = (value: number) => {
    if (Math.abs(value - Math.round(value)) < 0.01) return `L${Math.round(value)}`;
    return "";
  };

  if (wearHistory.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-[11px] text-muted-foreground">
        Waiting for telemetry...
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={wearHistory} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.3} />
        <XAxis
          dataKey="lap"
          type="number"
          domain={["dataMin", "dataMax"]}
          tickFormatter={formatXTick}
          tick={{ fontSize: 10, fill: "hsl(var(--muted-foreground))" }}
          label={{ value: "Lap", position: "insideBottom", offset: -2, fontSize: 10, fill: "hsl(var(--muted-foreground))" }}
        />
        <YAxis
          domain={[0, "auto"]}
          tick={{ fontSize: 10, fill: "hsl(var(--muted-foreground))" }}
          label={{ value: "Wear %", angle: -90, position: "insideLeft", fontSize: 10, fill: "hsl(var(--muted-foreground))" }}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: "hsl(var(--card))",
            border: "1px solid hsl(var(--border))",
            borderRadius: "6px",
            fontSize: "11px",
          }}
          labelStyle={{ color: "hsl(var(--foreground))" }}
          labelFormatter={(v: number) => `Lap ${v.toFixed(2)}`}
        />
        <Legend wrapperStyle={{ fontSize: "10px" }} />
        {sectorLines.map((sl, i) => (
          <ReferenceLine
            key={i}
            x={sl.x}
            stroke="hsl(var(--muted-foreground))"
            strokeDasharray="2 4"
            opacity={0.4}
            label={{ value: sl.label, position: "top", fontSize: 8, fill: "hsl(var(--muted-foreground))" }}
          />
        ))}
        <Line type="monotone" dataKey="fl" name="FL" stroke={WHEEL_COLORS.fl} strokeWidth={2} dot={false} isAnimationActive={false} />
        <Line type="monotone" dataKey="fr" name="FR" stroke={WHEEL_COLORS.fr} strokeWidth={2} dot={false} isAnimationActive={false} />
        <Line type="monotone" dataKey="rl" name="RL" stroke={WHEEL_COLORS.rl} strokeWidth={2} dot={false} isAnimationActive={false} />
        <Line type="monotone" dataKey="rr" name="RR" stroke={WHEEL_COLORS.rr} strokeWidth={2} dot={false} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

// ── Sub-panel 5: Projected Tyre Life ───────────────────────────────────────

interface WheelProjection {
  key: string;
  currentWear: number;
  ratePerLap: number;
  projectedLaps: number | null;
}

function computeProjections(data: TyreTelemetryData, wearHistory: LapWearRecord[]): WheelProjection[] {
  // Get current stint records (same compound as current)
  const currentCompound = data.compoundVisual;
  const stintRecords: LapWearRecord[] = [];
  for (let i = wearHistory.length - 1; i >= 0; i--) {
    if (wearHistory[i].compound === currentCompound) {
      stintRecords.unshift(wearHistory[i]);
    } else {
      break;
    }
  }

  return (["fl", "fr", "rl", "rr"] as const).map((key) => {
    const currentWear = data.tyres[key].wear;
    if (stintRecords.length < 2) {
      return { key: key.toUpperCase(), currentWear, ratePerLap: 0, projectedLaps: null };
    }

    const first = stintRecords[0][key];
    const last = stintRecords[stintRecords.length - 1][key];
    const lapSpan = stintRecords[stintRecords.length - 1].lap - stintRecords[0].lap;
    const ratePerLap = lapSpan > 0 ? (last - first) / lapSpan : 0;
    const remaining = ratePerLap > 0 ? (100 - currentWear) / ratePerLap : null;

    return {
      key: key.toUpperCase(),
      currentWear,
      ratePerLap,
      projectedLaps: remaining !== null ? Math.max(0, Math.round(remaining)) : null,
    };
  });
}

function ProjectedLifePanel({ data, wearHistory }: { data: TyreTelemetryData; wearHistory: LapWearRecord[] }) {
  const projections = computeProjections(data, wearHistory);
  const hasProjection = projections.some((p) => p.projectedLaps !== null);

  if (!hasProjection) {
    return (
      <div className="flex items-center justify-center h-full text-[11px] text-muted-foreground">
        Need more lap data for projection
      </div>
    );
  }

  const minLaps = Math.min(
    ...projections.filter((p) => p.projectedLaps !== null).map((p) => p.projectedLaps!)
  );

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 text-sm">
        <TrendingDown className={`h-4 w-4 ${minLaps < 5 ? "text-accent" : minLaps < 15 ? "text-warning" : "text-primary"}`} />
        <span className="text-muted-foreground">Est. remaining:</span>
        <span className={`font-display font-bold text-lg ${minLaps < 5 ? "text-accent" : minLaps < 15 ? "text-warning" : "text-primary"}`}>
          ~{minLaps} laps
        </span>
      </div>
      <table className="w-full text-[11px]">
        <thead>
          <tr className="text-muted-foreground uppercase tracking-wider border-b border-border/30">
            <th className="text-left py-1 font-display">Wheel</th>
            <th className="text-right py-1 font-display">Wear</th>
            <th className="text-right py-1 font-display">Rate/Lap</th>
            <th className="text-right py-1 font-display">Laps Left</th>
          </tr>
        </thead>
        <tbody>
          {projections.map((p) => {
            const lapsColor = p.projectedLaps === null
              ? "text-muted-foreground"
              : p.projectedLaps < 5
                ? "text-accent"
                : p.projectedLaps < 15
                  ? "text-warning"
                  : "text-primary";
            const isLimiting = p.projectedLaps === minLaps;
            return (
              <tr key={p.key} className={`border-b border-border/20 ${isLimiting ? "bg-accent/5" : ""}`}>
                <td className="py-1.5 font-display font-bold" style={{ color: WHEEL_COLORS[p.key.toLowerCase() as keyof typeof WHEEL_COLORS] }}>
                  {p.key}
                </td>
                <td className="py-1.5 text-right text-foreground">{p.currentWear.toFixed(1)}%</td>
                <td className="py-1.5 text-right text-foreground">
                  {p.ratePerLap > 0 ? `${p.ratePerLap.toFixed(2)}%` : "\u2014"}
                </td>
                <td className={`py-1.5 text-right font-bold ${lapsColor}`}>
                  {p.projectedLaps !== null ? `~${p.projectedLaps}` : "\u2014"}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── Sub-panel 6: Tyres Agent ───────────────────────────────────────────────

function TyresAgentPanel({ alerts, engineerResponses }: {
  alerts: Alert[];
  engineerResponses: TyresReportEntry[];
}) {
  const items = mergeItems(alerts, engineerResponses);
  const scrollRef = useAutoScroll<HTMLDivElement>(items.length);

  return (
    <div ref={scrollRef} className="flex-1 overflow-y-auto">
      {items.map((item, i) => {
        if (item.kind === "alert") {
          return (
            <div key={`a-${i}`} className={`flex items-start gap-2 px-4 py-3 border-b last:border-b-0 ${rowStyles[item.level]}`}>
              {alertIcons[item.level]}
              <span className="text-sm leading-relaxed text-card-foreground flex-1">{item.message}</span>
              <span className="text-[10px] text-muted-foreground tracking-wider shrink-0">{item.time}</span>
            </div>
          );
        }
        return (
          <div key={`e-${i}`} className="flex items-start gap-2 px-4 py-3 border-b last:border-b-0 border-border/20">
            <Bot className="h-4 w-4 text-info shrink-0 mt-0.5" />
            <div className="prose prose-sm prose-invert max-w-none text-sm leading-relaxed text-card-foreground flex-1 [&>p]:m-0">
              <ReactMarkdown>{item.text}</ReactMarkdown>
            </div>
            <span className="text-[10px] text-muted-foreground tracking-wider shrink-0">{item.time}</span>
          </div>
        );
      })}
      {items.length === 0 && (
        <div className="flex items-center gap-2 px-4 py-4">
          <CheckCircle className="h-4 w-4 text-primary" />
          <span className="text-sm text-muted-foreground">No alerts</span>
        </div>
      )}
    </div>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────

interface DetailedTyresViewProps {
  open: boolean;
  onClose: () => void;
  tyreData: TyreTelemetryData | null;
  wearHistory: LapWearRecord[];
  session: SessionData | null;
  tyreAlerts: Alert[];
  tyreActiveCount: number;
  engineerResponses: TyresReportEntry[];
}

export function DetailedTyresView({
  open,
  onClose,
  tyreData,
  wearHistory,
  session,
  tyreAlerts,
  tyreActiveCount,
  engineerResponses,
}: DetailedTyresViewProps) {
  if (!open) return null;

  const hasCritical = tyreAlerts.some((a) => a.level === "critical");
  const counterColor = tyreActiveCount === 0 ? "text-primary" : hasCritical ? "text-accent" : "text-warning";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-card border border-border/50 rounded-md overflow-hidden w-[95vw] max-w-6xl max-h-[90vh] shadow-2xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center gap-1.5 px-4 py-2 border-b border-border/50 bg-secondary/30 shrink-0">
          <Circle className="h-4 w-4 text-primary" />
          <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">
            Tyres View
          </span>
          <span className={`ml-auto font-display text-[10px] font-bold ${counterColor}`}>
            {tyreActiveCount} active alerts
          </span>
        </div>

        {/* Grid content */}
        <div className="flex-1 overflow-y-auto p-3">
          <div className="grid grid-cols-2 gap-3" style={{ gridTemplateRows: "auto auto minmax(10rem, 1fr)" }}>
            {/* Row 1 left: Tyres Telemetry */}
            <Panel
              title="Tyres Telemetry"
              icon={<Circle className="h-3.5 w-3.5 text-primary" />}
              className="row-span-1"
            >
              {tyreData ? (
                <TyresTelemetryPanel data={tyreData} />
              ) : (
                <span className="text-[11px] text-muted-foreground">Waiting for telemetry...</span>
              )}
            </Panel>

            {/* Row 1 right: Weather (stacked) */}
            <div className="flex flex-col gap-3">
              <Panel
                title="Current Weather"
                icon={<CloudSun className="h-3.5 w-3.5 text-primary" />}
              >
                {session ? (
                  <CurrentWeatherPanel session={session} />
                ) : (
                  <span className="text-[11px] text-muted-foreground">Waiting for session data...</span>
                )}
              </Panel>
              <Panel
                title="Weather Forecast"
                icon={<Droplets className="h-3.5 w-3.5 text-primary" />}
              >
                <WeatherForecastPanel forecast={session?.weatherForecast ?? []} />
              </Panel>
            </div>

            {/* Row 2 left: Historical Chart */}
            <Panel
              title="Historical Tyre Wear"
              icon={<TrendingDown className="h-3.5 w-3.5 text-primary" />}
              className="min-h-[14rem]"
            >
              <HistoricalWearChart wearHistory={wearHistory} tyreData={tyreData} />
            </Panel>

            {/* Row 2 right: Projected Life */}
            <Panel
              title="Projected Tyre Life"
              icon={<TrendingDown className="h-3.5 w-3.5 text-warning" />}
            >
              {tyreData ? (
                <ProjectedLifePanel data={tyreData} wearHistory={wearHistory} />
              ) : (
                <span className="text-[11px] text-muted-foreground">Waiting for telemetry...</span>
              )}
            </Panel>

            {/* Row 3: Tyres Agent (full width) */}
            <Panel
              title="Tyres Agent"
              icon={<Circle className="h-3.5 w-3.5 text-primary" />}
              className="col-span-2 max-h-[20rem]"
            >
              <TyresAgentPanel alerts={tyreAlerts} engineerResponses={engineerResponses} />
            </Panel>
          </div>
        </div>

        {/* Footer */}
        <div
          className="px-4 py-2 border-t border-border/50 bg-secondary/30 text-center cursor-pointer shrink-0"
          onClick={onClose}
        >
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-display">
            Click to close
          </span>
        </div>
      </div>
    </div>
  );
}
