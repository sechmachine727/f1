import {
  METRICS, RANGE_OPTIONS, formatAgo, formatValue, latestSample, windowSamples,
  type MetricSpec, type Sample,
} from "@/lib/driveHistory";

const RANGE_LABEL: Record<number, string> = {
  30000: "30 s", 60000: "1 min", 120000: "2 min", 300000: "5 min", 600000: "10 min",
};

type Props = {
  index: number;
  spec: MetricSpec;
  series: Sample[];
  rangeMs: number;
  color: string;
  onMetric: (id: string) => void;
  onRange: (ms: number) => void;
};

/** One compact live graph with a metric selector, range selector and labelled axes. */
export function DriveGraph({ index, spec, series, rangeMs, color, onMetric, onRange }: Props) {
  const end = series.length ? series[series.length - 1].t : 0;
  const visible = windowSamples(series, end, rangeMs);
  const values = visible.map((s) => s.v).filter((v): v is number => v != null);
  const min = values.length ? Math.min(...values) : 0;
  const max = values.length ? Math.max(...values) : 1;
  const span = max - min;
  const pad = span === 0 ? Math.max(0.2, Math.abs(max) * 0.02) : span * 0.1;
  const lo = min - pad;
  const hi = max + pad;
  const x = (t: number) => ((t - (end - rangeMs)) / Math.max(1, rangeMs)) * 100;
  const y = (v: number) => 40 - ((v - lo) / Math.max(1e-9, hi - lo)) * 40;
  const segments: { x: number; y: number }[][] = [];
  let current: { x: number; y: number }[] = [];
  for (const s of visible) {
    if (s.v == null) {
      if (current.length) segments.push(current);
      current = [];
      continue;
    }
    current.push({ x: x(s.t), y: y(s.v) });
  }
  if (current.length) segments.push(current);
  const latest = latestSample(series);
  const name = "Graph " + (index + 1);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col border-t border-[#232830] pt-[0.25em] first:border-t-0 first:pt-0">
      <div className="flex min-w-0 items-center gap-1.5 text-[0.7em]">
        <span className="shrink-0 uppercase tracking-wider text-neutral-500">{name}</span>
        <label className="sr-only" htmlFor={"graph-metric-" + index}>{name + " metric"}</label>
        <select id={"graph-metric-" + index} aria-label={name + " metric"} value={spec.id}
          onChange={(e) => onMetric(e.target.value)}
          className="min-w-0 max-w-[10em] rounded-sm border border-[#2a3038] bg-[#151920] px-1 py-0.5 text-neutral-200">
          {METRICS.map((m) => <option key={m.id} value={m.id}>{m.label}</option>)}
        </select>
        <label className="sr-only" htmlFor={"graph-range-" + index}>{name + " range"}</label>
        <select id={"graph-range-" + index} aria-label={name + " range"} value={rangeMs}
          onChange={(e) => onRange(Number(e.target.value))}
          className="shrink-0 rounded-sm border border-[#2a3038] bg-[#151920] px-1 py-0.5 text-neutral-300">
          {RANGE_OPTIONS.map((ms) => <option key={ms} value={ms}>{RANGE_LABEL[ms]}</option>)}
        </select>
        <span className="ml-auto shrink-0 text-[0.9em] tabular-nums text-neutral-100">
          <span className="text-[1.25em] font-semibold" style={{ color }}>{latest ? formatValue(latest.v as number, spec.digits) : "—"}</span>
          <span className="ml-1 text-[0.7em] text-neutral-400">{spec.unit}</span>
        </span>
      </div>

      <div className="mt-0.5 flex min-h-0 flex-1 gap-1">
        <div className="flex w-[3.4em] shrink-0 flex-col items-end text-[0.6em] leading-tight text-neutral-500">
          <span className="uppercase tracking-wide text-neutral-500">{spec.unit}</span>
          {values.length ? (
            <div className="flex flex-1 flex-col justify-between py-[0.15em] text-right tabular-nums">
              <span>{formatValue(hi, spec.digits)}</span>
              <span>{formatValue((hi + lo) / 2, spec.digits)}</span>
              <span>{formatValue(lo, spec.digits)}</span>
            </div>
          ) : <div className="flex flex-1 items-center justify-end text-neutral-600">—</div>}
        </div>
        <div className="relative min-h-[2.6em] min-w-0 flex-1">
          <div className="absolute inset-0 flex flex-col justify-between">
            {[0, 1, 2].map((i) => <div key={i} className="border-t border-[#232830]" />)}
          </div>
          <svg viewBox="0 0 100 40" preserveAspectRatio="none" className="absolute inset-0 h-full w-full"
            role="img" aria-label={name + ": " + spec.label + " over the last " + RANGE_LABEL[rangeMs]}>
            {segments.map((seg, i) => (
              <polyline key={i} points={seg.map((p) => p.x.toFixed(2) + "," + p.y.toFixed(2)).join(" ")}
                fill="none" stroke={color} strokeWidth="1.6" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
            ))}
            {segments.length === 1 && segments[0].length === 1 ? (
              <circle cx={segments[0][0].x} cy={segments[0][0].y} r="1.4" fill={color} />
            ) : null}
          </svg>
          {visible.length === 0 ? <span className="absolute inset-0 flex items-center justify-center text-[0.68em] text-neutral-500">{spec.id === "lap_time" ? "Waiting for a completed lap" : "Collecting…"}</span> : null}
        </div>
      </div>

      <div className="ml-[3.65em] flex justify-between pr-1 text-[0.6em] tabular-nums text-neutral-500">
        <span>{formatAgo(rangeMs)}</span>
        <span>{formatAgo(rangeMs / 2)}</span>
        <span>now</span>
        <span className="uppercase tracking-wide">time</span>
      </div>
    </div>
  );
}

