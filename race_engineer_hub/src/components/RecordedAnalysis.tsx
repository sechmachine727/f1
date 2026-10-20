import { useMemo, useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api } from "@/lib/live";
import { fmtLap, sessionLabel, useSessionAnalytics, useSessionTelemetry, type RecordedLap } from "@/lib/recorded";
import { useSelection } from "@/lib/selection";
import { lapRelativePoints } from "@/lib/samples";
import { TraceStack, type TraceSeries } from "@/components/TraceStack";
import { teamInk } from "@/lib/teams";
import { ZoomBox } from "@/components/ZoomBox";

const panel = "border border-[#232830] bg-[#11141a]";
const label = "text-[11px] uppercase tracking-wider text-neutral-400";
const colors = ["#5fd68f", "#f0a03c", "#7fb0ff", "#f25a5c"];

type CompareResult = {
  laps: RecordedLap[]; sector_deltas: Record<string, number>[]; corner_deltas: Record<string, number>[];
  theoretical_best_ms: number; differences: string[]; comparable: boolean;
};

type Invocation = {
  id: string; status: string;
  output: { rationale_summary?: string; driver_message?: string | null; evidence_ids?: string[]; sources?: { url?: string; title?: string }[] } | null;
  error: { code: string; message: string } | null;
};

export function RecordedAnalysis() {
  const sel = useSelection();
  const { data, error } = useSessionAnalytics(sel.sessionId);
  const [picked, setPicked] = useState<string[]>([]);
  const [result, setResult] = useState<CompareResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [question, setQuestion] = useState("");
  const [answer, setAnswer] = useState<{ message: string; evidence: string[]; sources: string[] } | null>(null);
  const [askError, setAskError] = useState("");
  const [exportState, setExportState] = useState("");
  const [download, setDownload] = useState("");
  const [replay, setReplay] = useState<{ id: string; paused: boolean; playhead_session_time_ms: number } | null>(null);

  const laps = useMemo(() => data?.laps.filter((l) => l.complete && l.lap_ms > 0) ?? [], [data]);
  const key = (l: RecordedLap) => `${l.car_index}:${l.lap}`;
  const toggle = (l: RecordedLap) => setPicked((p) => p.includes(key(l)) ? p.filter((x) => x !== key(l)) : p.length >= 4 ? p : [...p, key(l)]);
  const name = (car: number) => data?.participants.find((p) => p.car_index === car)?.name || `Car ${car + 1}`;

  async function compare() {
    if (!data || picked.length < 2) return;
    setBusy(true); setNotice("");
    try {
      const res = await api<CompareResult>(`/sessions/${data.session_id}/compare?laps=${encodeURIComponent(picked.join(","))}`);
      setResult(res);
    } catch (e) { setNotice(String(e)); } finally { setBusy(false); }
  }

  async function ask() {
    if (!data || !question.trim()) return;
    setAskError(""); setAnswer(null);
    try {
      const inv = await api<{ id: string }>("/engineer/requests", { method: "POST", body: JSON.stringify({
        profile: "post_race", session_id: data.session_id, branch_id: sel.branchId ?? "",
        observation_seq: "0", request_text: `${question.trim()}\nSelected laps: ${picked.join(", ") || "none"}`,
      }) });
      const id = inv.id;
      for (let i = 0; i < 30; i++) {
        await new Promise((r) => setTimeout(r, 1000));
        const list = await api<{ items?: Invocation[] }>("/engineer/invocations?limit=5");
        const found = (list.items ?? []).find((x) => x.id === id);
        if (found?.status === "accepted" && found.output) {
          setAnswer({ message: found.output.rationale_summary || found.output.driver_message || "No summary returned.", evidence: found.output.evidence_ids ?? [], sources: (found.output.sources ?? []).map((s) => s.url || s.title || "source") });
          return;
        }
        if (found?.error) { setAskError(`${found.error.code}: ${found.error.message}`); return; }
      }
      setAskError("The invocation did not finish within 30 seconds.");
    } catch (e) { setAskError(String(e)); }
  }

  async function exportRange() {
    if (!data) return;
    setExportState("queued"); setDownload("");
    try {
      const job = await api<{ id: string }>("/exports", { method: "POST", body: JSON.stringify({
        session_id: data.session_id, branch_id: sel.branchId ?? "",
        range: { from_seq: "0", to_seq: "999999999999" },
        cars: [...new Set(laps.map((l) => l.car_index))], channels: ["car.speed_mps"], format: "csv",
      }) });
      for (let i = 0; i < 30; i++) {
        await new Promise((r) => setTimeout(r, 800));
        const j = await api<{ state: string; download_ids?: string[]; error?: { message: string } }>("/jobs/" + job.id);
        setExportState(j.error ? j.error.message : j.state);
        if (j.state === "succeeded") { setDownload("/api/v1/downloads/" + (j.download_ids?.[0] ?? "")); return; }
        if (j.state === "failed" || j.state === "cancelled") return;
      }
      setExportState("still running");
    } catch (e) { setExportState(String(e)); }
  }

  async function openReplay() {
    if (!data) return;
    try {
      setReplay(await api("/replays", { method: "POST", body: JSON.stringify({ session_id: data.session_id, branch_id: sel.branchId ?? "", speed: 1, skip_long_pauses: false }) }));
    } catch (e) { setNotice(String(e)); }
  }

  const consistency = useMemo(() => {
    const times = laps.filter((l) => picked.includes(key(l))).map((l) => l.lap_ms);
    if (times.length < 2) return null;
    const mean = times.reduce((a, b) => a + b, 0) / times.length;
    return Math.sqrt(times.reduce((a, b) => a + (b - mean) ** 2, 0) / times.length) / 1000;
  }, [laps, picked]);

  const pickedPairs = picked.map((k) => { const [car, lap] = k.split(":").map(Number); return { car, lap }; });
  const pickedCars = [...new Set(pickedPairs.map((p) => p.car))];
  const pickedLaps = pickedPairs.map((p) => p.lap);
  const traces = useSessionTelemetry(
    data?.session_id ?? null, sel.branchId, pickedCars,
    pickedLaps.length ? Math.min(...pickedLaps) : undefined,
    pickedLaps.length ? Math.max(...pickedLaps) : undefined,
  );
  const traceSeries = useMemo<TraceSeries[]>(() => {
    const out: TraceSeries[] = [];
    for (const car of traces.data?.cars ?? []) {
      for (const lap of lapRelativePoints(car, pickedPairs.filter((p) => p.car === car.car_index).map((p) => p.lap))) {
        out.push({ key: "c" + car.car_index + "l" + lap.lap, name: `${car.name} L${lap.lap}`, color: teamInk(car.team_id), points: lap.points });
      }
    }
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [traces.data, picked.join(",")]);

  if (!sel.sessionId) return null;
  if (error) return <p className="p-4 text-[#f25a5c]">{error}</p>;
  if (!data) return <p className="p-4 text-sm text-neutral-400">Loading recorded session…</p>;

  const chartData = result ? result.laps.map((l) => ({ name: `${name(l.car_index)} L${l.lap}`, time: l.lap_ms / 1000 })) : [];

  return (
    <div className="min-h-[calc(100vh-49px)] space-y-3 bg-[#0b0d11] p-4 text-neutral-100">
      <div className={panel + " flex flex-wrap items-center gap-3 px-4 py-2"}>
        <h2 className="text-lg">Analysis</h2>
        <span className="text-sm text-neutral-400">{sessionLabel[data.session_type] ?? data.session_type} · track {data.track_id ?? "?"} · session {data.session_id.slice(0, 12)}</span>
        <button onClick={compare} disabled={busy || picked.length < 2} className="ml-auto rounded-sm bg-[#1d3b2c] px-3 py-1.5 text-sm text-[#4fd48a] disabled:opacity-40">{busy ? "Comparing…" : "Compare selected laps"}</button>
        <button onClick={exportRange} className="rounded-sm bg-[#151920] px-3 py-1.5 text-sm text-neutral-300">Export CSV</button>
        {exportState ? <span className="text-[12px] text-neutral-400" role="status">{exportState}{download ? <a className="ml-2 text-[#8ef0b4] underline" href={download}>Download</a> : null}</span> : null}
      </div>
      {notice ? <p role="alert" className="text-sm text-[#f25a5c]">{notice}</p> : null}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[320px_minmax(0,1fr)]">
        <section className={panel + " max-h-[520px] overflow-auto"}>
          <div className={label + " px-3 pt-3"}>Laps · pick two to four</div>
          <ul className="p-2">
            {laps.map((l) => (
              <li key={key(l)}><button aria-pressed={picked.includes(key(l))} onClick={() => toggle(l)}
                className={"flex w-full justify-between px-2 py-1 text-left text-[13px] " + (picked.includes(key(l)) ? "bg-[#1d3b2c] text-[#c7f5d6]" : "hover:bg-[#151920]")}>
                <span>{name(l.car_index)} · L{l.lap}</span><span className="tabular-nums text-neutral-400">{fmtLap(l.lap_ms)}</span></button></li>
            ))}
          </ul>
        </section>
        <div className="space-y-3">
          <section className={panel + " p-3"}>
            <div className={label}>Lap time comparison</div>
            {result ? <ZoomBox count={chartData.length} labels={chartData.map((d) => d.name)}>{(from, to) => <div className="h-[300px]"><ResponsiveContainer width="100%" height="100%"><LineChart data={chartData.slice(from, to + 1)}>
              <CartesianGrid stroke="#232830" strokeDasharray="3 3" /><XAxis dataKey="name" stroke="#6b7280" tick={{ fontSize: 11 }} /><YAxis hide domain={["dataMin - 200", "dataMax + 200"]} />
              <Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} /><Line dataKey="time" stroke="#5fd68f" strokeWidth={2} dot={false} />
            </LineChart></ResponsiveContainer></div>}</ZoomBox> : <p className="py-6 text-sm text-neutral-400">Pick laps and compare to see aligned time, sectors and theoretical best.</p>}
          </section>
          <section className={panel + " p-3"}>
            <div className={label}>Time-aligned inputs · each lap from its own start</div>
            <div className="mt-2">
              <TraceStack series={traceSeries} season={data.regulation_season} empty="Pick laps to see throttle, brake, gear and energy." />
            </div>
          </section>
          {result ? <section className={panel + " overflow-auto p-3"}>
            <div className={label}>Sector deltas · theoretical best {fmtLap(result.theoretical_best_ms)}</div>
            <table className="mt-2 w-full text-sm"><thead className="text-neutral-400"><tr><th className="p-2 text-left">Lap</th><th className="p-2 text-right">Lap time</th><th className="p-2 text-right">S1 Δ</th><th className="p-2 text-right">S2 Δ</th><th className="p-2 text-right">S3 Δ</th></tr></thead>
              <tbody>{result.laps.map((l, i) => <tr key={key(l)} className="border-t border-[#232830]"><td className="p-2">{name(l.car_index)} L{l.lap}</td><td className="p-2 text-right tabular-nums">{fmtLap(l.lap_ms)}</td><td className="p-2 text-right tabular-nums">{(result.sector_deltas[i]?.sector1 / 1000).toFixed(3)}</td><td className="p-2 text-right tabular-nums">{(result.sector_deltas[i]?.sector2 / 1000).toFixed(3)}</td><td className="p-2 text-right tabular-nums">{(result.sector_deltas[i]?.sector3 / 1000).toFixed(3)}</td></tr>)}</tbody></table>
            {result.differences.length ? <p className="mt-2 text-[12px] text-neutral-400">Differences: {result.differences.join("; ")}</p> : <p className="mt-2 text-[12px] text-[#5fd68f]">Laps are directly comparable.</p>}
          </section> : null}
          <section className={panel + " p-3"}>
            <div className={label}>Ask the race engineer about this session</div>
            <form className="mt-2 flex gap-2" onSubmit={(e) => { e.preventDefault(); void ask(); }}>
              <label className="sr-only" htmlFor="recorded-question">Question</label>
              <input id="recorded-question" value={question} onChange={(e) => setQuestion(e.target.value)} placeholder="Why is the second stint slower?" maxLength={4000} className="min-w-0 flex-1 border border-[#2a3038] bg-[#151920] px-2 py-1.5 text-sm outline-none focus-visible:border-[#5fd68f]" />
              <button type="submit" disabled={!question.trim()} className="border border-[#2f5a3d] bg-[#1d3b2c] px-3 py-1.5 text-sm text-[#8ef0b4] disabled:opacity-40">Ask</button>
            </form>
            {askError ? <p role="alert" className="mt-2 text-[12px] text-[#f25a5c]">{askError}</p> : null}
            {answer ? <div className="mt-2 text-sm text-neutral-200"><p>{answer.message}</p>{answer.evidence.length ? <p className="mt-1 text-[12px] text-neutral-400">Evidence: {answer.evidence.join(", ")}</p> : null}{answer.sources.length ? <p className="text-[12px] text-neutral-400">Sources: {answer.sources.join(", ")}</p> : null}</div> : null}
            {!answer && !askError ? <p className="mt-1 text-[12px] text-neutral-400">Answers use the existing post-race profile and the selected session only; the authored prompt bundle is unchanged.</p> : null}
          </section>
          <section className={panel + " p-3"}>
            <div className="flex items-baseline justify-between"><span className={label}>Replay and consistency</span>{consistency != null ? <span className="text-[12px] text-neutral-400">Selected-lap σ {consistency.toFixed(3)} s</span> : null}</div>
            {replay ? <div className="mt-2 flex items-center gap-3 text-sm">
              <span className="tabular-nums text-neutral-200">{replay.paused ? "Paused" : "Playing"} · {(replay.playhead_session_time_ms / 1000).toFixed(1)} s</span>
              <button className="underline text-neutral-300" onClick={async () => setReplay(await api("/replays/" + replay.id, { method: "PATCH", body: JSON.stringify({ paused: !replay.paused }) }))}>{replay.paused ? "Play" : "Pause"}</button>
              <button className="underline text-neutral-400" onClick={async () => { await api("/replays/" + replay.id, { method: "DELETE" }).catch(() => null); setReplay(null); }}>Close</button>
            </div> : <button onClick={openReplay} className="mt-2 rounded-sm bg-[#151920] px-3 py-1.5 text-sm text-neutral-300">Open replay</button>}
            <p className="mt-1 text-[12px] text-neutral-400">Replay needs the raw recording; charts render from stored analytics without it.</p>
          </section>
        </div>
      </div>
    </div>
  );
}
