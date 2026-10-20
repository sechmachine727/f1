import { useEffect, useMemo, useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api } from "@/lib/live";
import { clearSelection, select, useSelection } from "@/lib/selection";
import { carPoints, type SampleCar } from "@/lib/samples";
import { driversOf, fmtGap, fmtLap, sessionLabel, useSessionAnalytics, useSessionTelemetry, type RecordedLap, type SessionAnalytics } from "@/lib/recorded";
import { TraceStack } from "@/components/TraceStack";
import { teamInk } from "@/lib/teams";
import { ZoomBox } from "@/components/ZoomBox";

const panel = "border border-[#232830] bg-[#11141a]";
const label = "text-[11px] uppercase tracking-wider text-neutral-400";
const tabClass = "px-3 py-1.5 text-sm rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#4fd48a]";
const colors = ["#5fd68f", "#f0a03c", "#7fb0ff", "#f25a5c"];

type Tab = "overview" | "laps" | "sectors" | "speed" | "telemetry" | "tyres" | "fuel" | "stints" | "corners";
const tabs: { id: Tab; label: string }[] = [
  { id: "overview", label: "Overview" }, { id: "laps", label: "Laps" }, { id: "sectors", label: "Sectors" },
  { id: "speed", label: "Speed" }, { id: "telemetry", label: "Inputs" }, { id: "tyres", label: "Tyres" }, { id: "fuel", label: "Fuel / ERS" },
  { id: "stints", label: "Stints / runs" }, { id: "corners", label: "Corners" },
];

function nameOf(s: SessionAnalytics, car: number) {
  const p = s.participants.find((x) => x.car_index === car);
  return p?.name || `Car ${car + 1}`;
}

function seriesFor(s: SessionAnalytics, cars: number[], pick: (l: RecordedLap) => number | null) {
  const rows = new Map<number, Record<string, number | null>>();
  for (const car of cars) {
    for (const l of driversOf(s, car)) {
      const v = pick(l);
      if (v == null || !Number.isFinite(v)) continue;
      const row = rows.get(l.lap) ?? { lap: l.lap };
      row[nameOf(s, car)] = v;
      rows.set(l.lap, row);
    }
  }
  return [...rows.values()].sort((a, b) => (a.lap as number) - (b.lap as number));
}

function Chart({ data, unit }: { data: Record<string, number | null>[]; unit: string }) {
  const keys = data.length ? Object.keys(data[0]).filter((k) => k !== "lap") : [];
  if (!data.length) return <p className="px-4 py-6 text-sm text-neutral-400">No recorded laps for this view.</p>;
  return (
    <ZoomBox count={data.length} labels={data.map((d) => "L" + d.lap)}>
      {(from, to) => <div className="h-[340px] px-2 pb-2">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data.slice(from, to + 1)}>
          <CartesianGrid stroke="#232830" strokeDasharray="3 3" />
          <XAxis dataKey="lap" stroke="#6b7280" tick={{ fontSize: 11 }} />
          <YAxis stroke="#6b7280" tick={{ fontSize: 11 }} width={64} tickFormatter={(v) => `${v}${unit}`} domain={["auto", "auto"]} />
          <Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} formatter={(v: number) => [`${Number(v).toFixed(3)} ${unit}`, ""]} />
          <Legend wrapperStyle={{ fontSize: 12 }} />
          {keys.map((k, i) => <Line key={k} dataKey={k} dot={false} strokeWidth={2} stroke={colors[i % 4]} connectNulls={false} />)}
        </LineChart>
      </ResponsiveContainer>
      </div>}
    </ZoomBox>
  );
}

export function RecordedPitWall() {
  const sel = useSelection();
  const { data, error } = useSessionAnalytics(sel.sessionId);
  const [tab, setTab] = useState<Tab>("overview");
  const [cars, setCars] = useState<number[] | null>(null);
  const [fromLap, setFromLap] = useState("");
  const [toLap, setToLap] = useState("");
  const [segments, setSegments] = useState<{ id: string; session_type: string }[]>([]);
  const drivers = useMemo(() => data ? data.participants.map((p) => p.car_index).filter((c) => data.laps.some((l) => l.car_index === c)) : [], [data]);
  const selected = cars ?? (sel.sessionId ? [drivers[0] ?? 0] : []);
  const traces = useSessionTelemetry(
    tab === "telemetry" ? sel.sessionId : null, sel.branchId, selected,
    Number(fromLap) || undefined, Number(toLap) || undefined,
  );
  const traceSeries = (traces.data?.cars ?? []).map((c: SampleCar) => ({
    key: "c" + c.car_index, name: c.name || "Car " + (c.car_index + 1),
    color: teamInk(c.team_id), points: carPoints(c),
  }));
  const toggleCar = (car: number) => {
    const next = selected.includes(car) ? selected.filter((c) => c !== car) : selected.length >= 4 ? selected : [...selected, car];
    setCars(next.length ? next : [car]);
  };
  useEffect(() => {
    if (!data || !data.session_type.startsWith("qualifying") || !data.weekend_link_id) { setSegments([]); return; }
    api<{ items: { id: string; session_type: string; weekend_link_id?: string | null }[] }>("/sessions?limit=100")
      .then((r) => setSegments((r.items ?? []).filter((x) => x.weekend_link_id === data.weekend_link_id && x.session_type.startsWith("qualifying"))))
      .catch(() => setSegments([]));
  }, [data?.session_id, data?.weekend_link_id, data?.session_type]);

  if (!sel.sessionId) return null;
  if (error) return <div className="p-4"><p role="alert" className="text-[#f25a5c]">{error}</p></div>;
  if (!data) return <p className="p-4 text-sm text-neutral-400">Loading recorded session…</p>;
  const bestLaps = selected.map((car) => ({ car, lap: Math.min(...driversOf(data, car).map((l) => l.lap_ms)) })).filter((x) => Number.isFinite(x.lap));
  const fastest = bestLaps.length ? Math.min(...bestLaps.map((x) => x.lap)) : 0;
  const ordered = [...drivers].sort((a, b) => {
    const ba = Math.min(...driversOf(data, a).map((l) => l.lap_ms));
    const bb = Math.min(...driversOf(data, b).map((l) => l.lap_ms));
    return ba - bb;
  });

  return (
    <div className="min-h-[calc(100vh-49px)] space-y-3 bg-[#0b0d11] p-4 text-neutral-100">
      <div className="flex items-center gap-3 border border-[#3b4a2f] bg-[#16201a] px-4 py-2 text-sm">
        <span className="rounded-sm bg-[#2a4a33] px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider text-[#8ef0b4]">Recorded</span>
        <span className="text-neutral-200">{sessionLabel[data.session_type] ?? data.session_type} · track {data.track_id ?? "unknown"} · {String(data.regulation_season ?? "season unknown")}</span>
        {data.session_type === "practice" ? <span className="text-[12px] text-neutral-400">Runs split by pit visits</span> : null}
        {data.session_type.startsWith("qualifying") ? <span className="text-[12px] text-neutral-400">Segment {data.session_type.replace("qualifying_", "").toUpperCase()}</span> : null}
        {segments.length > 1 ? <div className="flex gap-1" role="tablist" aria-label="Qualifying segments">
          {segments.sort((a, b) => a.session_type.localeCompare(b.session_type)).map((seg) => (
            <button key={seg.id} role="tab" aria-selected={seg.id === data.session_id} className={tabClass + (seg.id === data.session_id ? " bg-[#1d3b2c] text-[#4fd48a]" : " text-neutral-300")}
              onClick={async () => {
                const d = await api<{ surviving_branch_id: string }>("/sessions/" + seg.id);
                select({ sessionId: seg.id, branchId: d.surviving_branch_id, label: seg.session_type.replace("qualifying_", "Q").toUpperCase(), season: data.regulation_season, recorded: true });
              }}>{seg.session_type.replace("qualifying_", "Q").toUpperCase()}</button>
          ))}
        </div> : null}
        <button className="ml-auto text-sm text-neutral-300 underline" onClick={clearSelection}>Back to live</button>
      </div>

      <div className={panel + " flex flex-wrap items-center gap-3 px-4 py-2"}>
        <h2 className="text-lg font-semibold">Session analysis</h2>
        <span className="text-sm text-neutral-400">{data.clean_laps} clean laps · best {fmtLap(data.best_lap_ms)} · theoretical {fmtLap(data.theoretical_best_ms)}</span>
        <div className="ml-auto flex flex-wrap gap-1" role="tablist" aria-label="Analysis view">
          {tabs.map((t) => <button key={t.id} role="tab" aria-selected={tab === t.id} onClick={() => setTab(t.id)} className={tabClass + (tab === t.id ? " bg-[#1d3b2c] text-[#4fd48a]" : " text-neutral-300 hover:bg-[#151920]")}>{t.label}</button>)}
        </div>
      </div>

      <div className={panel + " px-4 py-2"}>
        <div className={label + " mb-2"}>Compare drivers (up to four)</div>
        <div className="flex flex-wrap gap-2">
          {drivers.map((car) => (
            <button key={car} onClick={() => toggleCar(car)} aria-pressed={selected.includes(car)}
              className={tabClass + (selected.includes(car) ? " bg-[#1d3b2c] text-[#c7f5d6]" : " bg-[#151920] text-neutral-300")}>
              {nameOf(data, car)}
            </button>
          ))}
        </div>
      </div>

      {tab === "overview" ? (
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <section className={panel}><div className={label + " px-4 pt-3"}>Session classification</div>
            <table className="w-full text-sm"><thead className="text-neutral-400"><tr><th className="p-2 text-left">Pos</th><th className="p-2 text-left">Driver</th><th className="p-2 text-right">Best lap</th><th className="p-2 text-right">Gap</th><th className="p-2 text-right">Laps</th></tr></thead>
              <tbody>{ordered.map((car) => {
                const laps = driversOf(data, car);
                const best = Math.min(...laps.map((l) => l.lap_ms));
                const last = laps[laps.length - 1];
                return <tr key={car} className={"border-t border-[#232830] " + (selected.includes(car) ? "bg-[#18211b]" : "")}><td className="p-2 tabular-nums">{last?.position_end ? last.position_end : "—"}</td><td className="p-2">{nameOf(data, car)}</td><td className="p-2 text-right tabular-nums">{fmtLap(best)}</td><td className="p-2 text-right tabular-nums">{fastest ? fmtGap(best - fastest) : "—"}</td><td className="p-2 text-right tabular-nums">{laps.length}</td></tr>;
              })}</tbody></table>
          </section>
          <section className={panel + " p-3"}><div className={label}>Lap time and gap to leader</div><Chart data={seriesFor(data, selected, (l) => l.lap_ms / 1000)} unit="s" />
            <div className="mt-2 border-t border-[#232830] pt-2"><Chart data={seriesFor(data, selected, (l) => l.gap_leader_start_ms ? l.gap_leader_start_ms / 1000 : null)} unit="s" /></div>
          </section>
        </div>
      ) : null}

      {tab === "laps" ? <section className={panel + " overflow-auto"}>
        <table className="w-full text-sm"><thead className="sticky top-0 bg-[#151920] text-neutral-400"><tr>
          <th className="p-2 text-left">Driver</th><th className="p-2 text-right">Lap</th><th className="p-2 text-right">Time</th><th className="p-2 text-right">S1</th><th className="p-2 text-right">S2</th><th className="p-2 text-right">S3</th><th className="p-2 text-right">Avg km/h</th><th className="p-2 text-right">Top km/h</th><th className="p-2 text-right">Tyre</th><th className="p-2 text-right">Pos</th><th className="p-2 text-right">Flags</th>
        </tr></thead><tbody>
          {selected.flatMap((car) => driversOf(data, car).map((l) => (
            <tr key={`${car}-${l.lap}`} className="border-t border-[#232830]"><td className="p-2">{nameOf(data, car)}</td><td className="p-2 text-right tabular-nums">{l.lap}</td><td className="p-2 text-right tabular-nums">{fmtLap(l.lap_ms)}</td><td className="p-2 text-right tabular-nums">{l.sector1_ms ? (l.sector1_ms / 1000).toFixed(3) : "—"}</td><td className="p-2 text-right tabular-nums">{l.sector2_ms ? (l.sector2_ms / 1000).toFixed(3) : "—"}</td><td className="p-2 text-right tabular-nums">{l.sector3_ms ? (l.sector3_ms / 1000).toFixed(3) : "—"}</td><td className="p-2 text-right tabular-nums">{(l.avg_speed_mps * 3.6).toFixed(0)}</td><td className="p-2 text-right tabular-nums">{(l.top_speed_mps * 3.6).toFixed(0)}</td><td className="p-2 text-right">{l.compound || "—"} · {l.tyre_age_laps}</td><td className="p-2 text-right tabular-nums">{l.position_end}</td><td className="p-2 text-right text-neutral-400">{l.pit_in ? "Pit " : ""}{l.valid ? "" : "Invalid "}{l.track_limits ? "Limits" : ""}</td></tr>
          )))}
        </tbody></table>
      </section> : null}

      {tab === "sectors" ? <section className={panel + " p-3"}><div className={label}>Sector 1 / 2 / 3 by lap</div><Chart data={seriesFor(data, selected, (l) => l.sector1_ms ? l.sector1_ms / 1000 : null)} unit="s" /></section> : null}
      {tab === "speed" ? <section className={panel + " p-3"}><div className={label}>Average and top speed by lap</div><Chart data={seriesFor(data, selected, (l) => l.avg_speed_mps * 3.6)} unit="km/h" /></section> : null}
      {tab === "telemetry" ? <section className={panel + " p-3"}>
        <div className="flex flex-wrap items-center gap-4">
          <div className={label}>Throttle, brake, gear and energy over session time</div>
          <div className="ml-auto flex items-center gap-2 text-[12px] text-neutral-400">
            <label htmlFor="from-lap">Laps from</label>
            <input id="from-lap" inputMode="numeric" value={fromLap} onChange={(e) => setFromLap(e.target.value.replace(/\D/g, ""))} className="w-14 border border-[#2a3038] bg-[#151920] px-2 py-1 text-neutral-100" />
            <label htmlFor="to-lap">to</label>
            <input id="to-lap" inputMode="numeric" value={toLap} onChange={(e) => setToLap(e.target.value.replace(/\D/g, ""))} className="w-14 border border-[#2a3038] bg-[#151920] px-2 py-1 text-neutral-100" />
          </div>
        </div>
        <div className="mt-2">
          <TraceStack series={traceSeries} season={data.regulation_season} empty={
            traces.data && traces.data.laps_available === 0
              ? "Samples are being derived for this recording. Lap aggregates remain available."
              : "No stored samples for the selected cars and lap range."
          } />
        </div>
      </section> : null}
      {tab === "tyres" ? <section className={panel + " p-3"}><div className={label}>Tyre age by lap</div><Chart data={seriesFor(data, selected, (l) => l.tyre_age_laps)} unit=" laps" /></section> : null}
      {tab === "fuel" ? <section className={panel + " p-3"}><div className={label}>Fuel at lap end (kg)</div><Chart data={seriesFor(data, selected, (l) => l.fuel_end_kg || null)} unit=" kg" /></section> : null}
      {tab === "stints" ? <section className={panel + " overflow-auto"}>
        <table className="w-full text-sm"><thead className="bg-[#151920] text-neutral-400"><tr><th className="p-2 text-left">Driver</th><th className="p-2 text-left">Kind</th><th className="p-2 text-right">Laps</th><th className="p-2 text-right">Best</th><th className="p-2 text-right">Median</th><th className="p-2 text-right">Degradation</th><th className="p-2 text-right">Compound</th></tr></thead>
          <tbody>{data.stints.filter((s) => selected.includes(s.car_index)).map((s) => <tr key={`${s.car_index}-${s.kind}-${s.index}`} className="border-t border-[#232830]"><td className="p-2">{nameOf(data, s.car_index)}</td><td className="p-2 text-neutral-400">{s.kind} {s.index + 1}</td><td className="p-2 text-right tabular-nums">{s.start_lap}–{s.end_lap} ({s.laps})</td><td className="p-2 text-right tabular-nums">{fmtLap(s.best_ms)}</td><td className="p-2 text-right tabular-nums">{fmtLap(s.median_ms)}</td><td className="p-2 text-right tabular-nums">{s.degradation_ms_per_lap.toFixed(1)} ms/lap</td><td className="p-2 text-right">{s.compound || "—"}</td></tr>)}</tbody></table>
      </section> : null}
      {tab === "corners" ? <section className={panel + " p-3"}>
        <div className={label}>Provisional corner minimum speed</div>
        <p className="mt-1 text-[12px] text-neutral-400">Derived from circuit geometry and recorded samples; not a calibrated optimum.</p>
        <table className="mt-2 w-full text-sm"><thead className="text-neutral-400"><tr><th className="p-2 text-left">Corner</th>{selected.map((car) => <th key={car} className="p-2 text-right">{nameOf(data, car)}</th>)}</tr></thead>
          <tbody>{[...new Set(selected.flatMap((car) => driversOf(data, car).flatMap((l) => Object.keys(l.corner_speeds ?? {}))))].sort().map((k) => <tr key={k} className="border-t border-[#232830]"><td className="p-2">{k}</td>{selected.map((car) => {
            const laps = driversOf(data, car);
            const vals = laps.map((l) => l.corner_speeds?.[k]?.value).filter((v): v is number => typeof v === "number");
            const best = vals.length ? Math.min(...vals) : null;
            return <td key={car} className="p-2 text-right tabular-nums">{best == null ? "—" : (best * 3.6).toFixed(0) + " km/h"}</td>;
          })}</tr>)}</tbody></table>
      </section> : null}
    </div>
  );
}
