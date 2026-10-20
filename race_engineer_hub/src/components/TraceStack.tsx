import { useEffect, useMemo, useRef, useState } from "react";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { bandLegend, deploymentBands, overtakeBands, type Band } from "@/lib/bands";
import {
  clampWindow, formatSessionTime, fullWindow, isAtEnd, panBy, span, zoomAt, zoomPercent, type Window,
} from "@/lib/chartWindow";
import { visiblePoints, type SamplePoint } from "@/lib/samples";

export type TraceSeries = { key: string; name: string; color: string; points: SamplePoint[] };

type Row = { t: number } & Record<string, number | null>;

const jsonFormat = (v: number) => (Math.abs(v) >= 1000000 ? (v / 1000000).toFixed(2) + " MJ" : Math.round(v) + " J");

function mergeRows(series: TraceSeries[]): Row[] {
  const byTime = new Map<number, Row>();
  for (const s of series) {
    for (const p of s.points) {
      const row = byTime.get(p[0]) ?? ({ t: p[0] } as Row);
      row[s.key + ".thr"] = p[1];
      row[s.key + ".brk"] = p[2];
      row[s.key + ".gear"] = p[3];
      row[s.key + ".ers"] = p[4];
      byTime.set(p[0], row);
    }
  }
  return [...byTime.values()].sort((a, b) => a.t - b.t);
}

function TraceTooltip({ active, payload, series, season }: { active?: boolean; payload?: { payload: Row }[]; series: TraceSeries[]; season: number | null }) {
  if (!active || !payload?.length) return null;
  const row = payload[0].payload;
  return (
    <div className="rounded-sm border border-[#2b323c] bg-[#0f1218] px-2 py-1.5 text-[11px] text-neutral-200 shadow">
      <div className="mb-1 tabular-nums text-neutral-400">{formatSessionTime(row.t)}</div>
      {series.map((s) => {
        const thr = row[s.key + ".thr"], brk = row[s.key + ".brk"], gear = row[s.key + ".gear"], ers = row[s.key + ".ers"];
        return (
          <div key={s.key} className="flex items-baseline gap-2">
            <span style={{ color: s.color }}>{s.name}</span>
            <span className="tabular-nums">
              {thr == null ? "—" : Math.round(thr * 100) + "%"} throttle · {brk == null ? "—" : Math.round(brk * 100) + "%"} brake
              {gear == null ? "" : " · " + Math.round(gear) + "g"}
              {ers == null ? "" : " · " + jsonFormat(ers)}
            </span>
          </div>
        );
      })}
      {season === 2026 ? null : <div className="text-neutral-500">Energy state follows the deployment band.</div>}
    </div>
  );
}

function BandStrip({ bands, title }: { bands: Band[]; title: string }) {
  const legend = bandLegend(bands);
  return (
    <div className="mt-1">
      <div className="relative h-2.5 w-full overflow-hidden rounded-sm bg-[#1a1f27]" role="img" aria-label={title + ": " + legend.map((l) => l.label).join(", ")}>
        {bands.map((b, i) => (
          <div key={i} title={b.label} className="absolute top-0 h-full" style={{ left: b.start + "%", width: Math.max(0.4, b.end - b.start) + "%", background: b.color }} />
        ))}
      </div>
      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-neutral-400">
        {legend.length === 0 ? <span>No state reported in this window</span> : legend.map((l) => (
          <span key={l.label} className="flex items-center gap-1"><span className="inline-block h-2 w-2 rounded-sm" style={{ background: l.color }} />{l.label}</span>
        ))}
      </div>
    </div>
  );
}

type Props = {
  series: TraceSeries[];
  season: number | null;
  live?: boolean;
  empty?: string;
};

export function TraceStack({ series, season, live = false, empty }: Props) {
  const [win, setWin] = useState<Window | null>(null);
  const [follow, setFollow] = useState(true);
  const [readout, setReadout] = useState<number | null>(null);
  const plotRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ x: number; start: number; end: number } | null>(null);

  const range = useMemo(() => {
    let min = Infinity, max = -Infinity;
    for (const s of series) for (const p of s.points) {
      if (p[0] < min) min = p[0];
      if (p[0] > max) max = p[0];
    }
    return min === Infinity ? { min: 0, max: 0 } : { min, max };
  }, [series]);
  const { min, max } = range;
  const effective = win ? clampWindow(win, min, max) : fullWindow(min, max);
  const percent = zoomPercent(effective, min, max);

  useEffect(() => {
    if (!live || !follow) return;
    setWin((w) => {
      if (!w) return null;
      const width = span(w);
      return { start: Math.max(min, max - width), end: max };
    });
  }, [max, min, live, follow]);

  const applyWindow = (next: Window) => {
    setWin(next);
    setFollow(isAtEnd(next, max));
  };
  const zoom = (factor: number, at?: number) => {
    const centre = at ?? (effective.start + effective.end) / 2;
    applyWindow(zoomAt(effective, centre, factor, min, max));
  };
  const reset = () => { setWin(null); setFollow(true); };
  const followLatest = () => {
    const width = span(effective);
    setFollow(true);
    setWin(width >= max - min ? null : { start: Math.max(min, max - width), end: max });
  };

  // Wheel zoom needs a non-passive listener to stop the page scrolling.
  useEffect(() => {
    const el = plotRef.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (max === min) return;
      e.preventDefault();
      const rect = el.getBoundingClientRect();
      const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / Math.max(1, rect.width)));
      const at = effective.start + ratio * span(effective);
      applyWindow(zoomAt(effective, at, e.deltaY < 0 ? 1.25 : 0.8, min, max));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  });

  const onKeyDown = (e: React.KeyboardEvent) => {
    const step = span(effective) * 0.1;
    if (e.key === "+" || e.key === "=") { zoom(1.5); e.preventDefault(); }
    else if (e.key === "-" || e.key === "_") { zoom(1 / 1.5); e.preventDefault(); }
    else if (e.key === "ArrowLeft") { applyWindow(panBy(effective, -step, min, max)); e.preventDefault(); }
    else if (e.key === "ArrowRight") { applyWindow(panBy(effective, step, min, max)); e.preventDefault(); }
    else if (e.key === "0") { reset(); e.preventDefault(); }
    else if (e.key.toLowerCase() === "f" && live) { followLatest(); e.preventDefault(); }
  };

  const rows = useMemo(() => mergeRows(series.map((s) => ({ ...s, points: visiblePoints(s.points, effective.start, effective.end) }))), [series, effective.start, effective.end]);
  const reference = series[0]?.points ?? [];
  const deploy = useMemo(() => deploymentBands(reference, season, effective.start, effective.end).map((b) => ({
    ...b,
    start: ((b.start - effective.start) / Math.max(1, span(effective))) * 100,
    end: ((b.end - effective.start) / Math.max(1, span(effective))) * 100,
  })), [reference, season, effective.start, effective.end]);
  const overtake = useMemo(() => {
    const bands = overtakeBands(reference, season, effective.start, effective.end);
    const width = Math.max(1, span(effective));
    return bands.map((b) => ({
      ...b,
      start: ((b.start - effective.start) / width) * 100,
      end: ((b.end - effective.start) / width) * 100,
    }));
  }, [reference, season, effective.start, effective.end]);

  const hasPoints = rows.length > 0;
  // A constant series would otherwise collapse to a single tick.
  const ersValues = rows.flatMap((r) => series.map((s) => r[s.key + ".ers"])).filter((v): v is number => typeof v === "number");
  const ersMin = ersValues.length ? Math.min(...ersValues) : 0;
  const ersMax = ersValues.length ? Math.max(...ersValues) : 1;
  const ersPad = ersMax - ersMin > 1 ? (ersMax - ersMin) * 0.08 : Math.max(100000, Math.abs(ersMax) * 0.05);
  const axisCommon = {
    dataKey: "t" as const,
    type: "number" as const,
    domain: [effective.start, effective.end] as [number, number],
    allowDataOverflow: true,
    stroke: "#4b5563",
    tick: { fontSize: 10, fill: "#9ca3af" },
    tickFormatter: (v: number) => formatSessionTime(v),
  };

  return (
    <div className="border border-[#232830] bg-[#11141a] p-3" role="group" aria-label="Throttle, brake, gear and energy traces" tabIndex={0}
      onKeyDown={onKeyDown}
      onPointerMove={(e) => {
        if (!drag.current || !plotRef.current) return;
        const width = Math.max(1, plotRef.current.getBoundingClientRect().width);
        const delta = ((drag.current.x - e.clientX) / width) * (drag.current.end - drag.current.start);
        applyWindow(panBy({ start: drag.current.start, end: drag.current.end }, delta, min, max));
      }}
      onPointerUp={() => { drag.current = null; }}
      onPointerLeave={() => { drag.current = null; setReadout(null); }}
      style={{ outlineOffset: 2 }}
    >
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className="text-[11px] uppercase tracking-wider text-neutral-400">Telemetry</span>
        <span className="text-[11px] tabular-nums text-neutral-300">{formatSessionTime(effective.start)} – {formatSessionTime(effective.end)}</span>
        <span className="text-[11px] text-neutral-400">{percent}%</span>
        <div className="ml-auto flex items-center gap-1">
          <button type="button" className="px-2 text-sm text-neutral-300 hover:bg-[#1a2027]" aria-label="Zoom out" onClick={() => zoom(1 / 1.5)}>−</button>
          <button type="button" className="px-2 text-sm text-neutral-300 hover:bg-[#1a2027]" aria-label="Zoom in" onClick={() => zoom(1.5)}>+</button>
          <button type="button" className="px-2 text-[11px] text-neutral-300 hover:bg-[#1a2027]" onClick={reset}>Reset</button>
          {live ? <button type="button" disabled={follow} onClick={followLatest} className="px-2 text-[11px] text-[#8ef0b4] disabled:opacity-40">Follow latest</button> : null}
        </div>
      </div>

      {!hasPoints ? (
        <p className="py-8 text-center text-sm text-neutral-400">{empty ?? "No samples in this window."}</p>
      ) : (
        <>
          <div ref={plotRef} className="cursor-grab select-none"
            onPointerDown={(e) => {
              drag.current = { x: e.clientX, start: effective.start, end: effective.end };
              (e.target as Element).setPointerCapture?.(e.pointerId);
            }}>
            <div className="text-[10px] uppercase tracking-wider text-neutral-400">Throttle / brake</div>
            <div className="h-[110px] border-b border-[#232830]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={rows} margin={{ top: 4, right: 8, left: 0, bottom: 0 }} syncId="trace"
                  onMouseMove={(s) => { const t = (s as { activePayload?: { payload: Row }[] }).activePayload?.[0]?.payload.t; setReadout(t ?? null); }}>
                  <CartesianGrid stroke="#232830" strokeDasharray="3 3" />
                  <XAxis {...axisCommon} hide />
                  <YAxis domain={[0, 100]} ticks={[0, 50, 100]} width={40} stroke="#4b5563" tick={{ fontSize: 10, fill: "#9ca3af" }} tickFormatter={(v) => v + "%"} />
                  <Tooltip content={<TraceTooltip series={series} season={season} />} cursor={{ stroke: "#6b7280" }} />
                  {series.map((s) => (
                    <Line key={s.key + "thr"} dataKey={s.key + ".thr"} stroke={s.color} dot={false} strokeWidth={2} type="monotone" isAnimationActive={false} connectNulls={false} name={s.name + " throttle"} />
                  ))}
                  {series.map((s) => (
                    <Line key={s.key + "brk"} dataKey={s.key + ".brk"} stroke={s.color} strokeDasharray="4 3" dot={false} strokeWidth={2} type="monotone" isAnimationActive={false} connectNulls={false} name={s.name + " brake"} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
            <div className="mt-1 flex items-center gap-3 text-[10px] text-neutral-400">
              <span>Throttle solid · brake dashed</span>
              {readout != null ? <span className="tabular-nums">{formatSessionTime(readout)}</span> : null}
            </div>

            <div className="mt-1 text-[10px] uppercase tracking-wider text-neutral-400">Gear</div>
            <div className="h-[64px] border-b border-[#232830]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={rows} margin={{ top: 2, right: 8, left: 0, bottom: 0 }} syncId="trace">
                  <CartesianGrid stroke="#232830" strokeDasharray="3 3" />
                  <XAxis {...axisCommon} hide />
                  <YAxis domain={[-1, 8]} ticks={[1, 4, 7]} width={40} stroke="#4b5563" tick={{ fontSize: 10, fill: "#9ca3af" }} />
                  <Tooltip content={<TraceTooltip series={series} season={season} />} cursor={{ stroke: "#6b7280" }} />
                  {series.map((s) => (
                    <Line key={s.key + "gear"} dataKey={s.key + ".gear"} stroke={s.color} dot={false} strokeWidth={1.5} type="stepAfter" isAnimationActive={false} connectNulls={false} name={s.name + " gear"} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>

            <div className="mt-1 text-[10px] uppercase tracking-wider text-neutral-400">ERS store</div>
            <div className="h-[92px]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={rows} margin={{ top: 2, right: 8, left: 0, bottom: 0 }} syncId="trace">
                  <CartesianGrid stroke="#232830" strokeDasharray="3 3" />
                  <XAxis {...axisCommon} />
                  <YAxis width={46} domain={[ersMin - ersPad, ersMax + ersPad]} stroke="#4b5563" tick={{ fontSize: 10, fill: "#9ca3af" }} tickFormatter={(v: number) => (v / 1000000).toFixed(2) + "M"} />
                  <Tooltip content={<TraceTooltip series={series} season={season} />} cursor={{ stroke: "#6b7280" }} />
                  {series.map((s) => (
                    <Line key={s.key + "ers"} dataKey={s.key + ".ers"} stroke={s.color} dot={false} strokeWidth={2} type="monotone" isAnimationActive={false} connectNulls={false} name={s.name + " energy"} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>

          <div className="mt-1 text-[10px] uppercase tracking-wider text-neutral-400">Energy state</div>
          <BandStrip bands={deploy} title={season === 2026 ? "Boost and deployment" : "Deployment"} />
          {season === 2026 ? <><div className="mt-2 text-[10px] uppercase tracking-wider text-neutral-400">Overtake</div><BandStrip bands={overtake} title="Overtake availability" /></> : null}

          <div
            className="mt-3 h-3 w-full rounded-sm bg-[#1a1f27]"
            role="slider" tabIndex={0} aria-label="Visible range" aria-valuemin={min} aria-valuemax={max}
            aria-valuenow={Math.round(effective.start)} aria-valuetext={formatSessionTime(effective.start)}
            onPointerDown={(e) => {
              const rect = e.currentTarget.getBoundingClientRect();
              const width = span(effective);
              const at = min + ((e.clientX - rect.left) / Math.max(1, rect.width)) * (max - min);
              applyWindow(clampWindow({ start: at - width / 2, end: at + width / 2 }, min, max));
            }}
          >
            <div className="relative h-full">
              <div className="absolute top-0 h-full rounded-sm bg-[#3d5c48]"
                style={{ left: ((effective.start - min) / Math.max(1, max - min)) * 100 + "%", width: (span(effective) / Math.max(1, max - min)) * 100 + "%" }} />
            </div>
          </div>
        </>
      )}
    </div>
  );
}

