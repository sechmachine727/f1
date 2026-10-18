/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from "react";
import { Drive } from "@/components/Drive";
import { api, metric, text, useLive } from "@/lib/live";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip } from "recharts";
type Tab = "drive" | "pit" | "history" | "analysis" | "setups" | "settings";
const tabs: { id: Tab; label: string }[] = [
  { id: "drive", label: "Drive" }, { id: "pit", label: "Pit wall" }, { id: "history", label: "History" },
  { id: "analysis", label: "Analysis" }, { id: "setups", label: "Setups" }, { id: "settings", label: "Settings" },
];
const nav = "px-3 py-1.5 text-sm rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#4fd48a]";
function PitWall() {
  const s = useLive();
  const cars = Array.from({ length: 24 }, (_, i) => i).filter((i) => metric(s, "car.speed_mps", i) || i === (s.context?.player_car_index ?? 0));
  return <div className="min-h-screen bg-[#0b0d11] text-neutral-100 p-4">
    <div className="flex justify-between border border-[#232830] bg-[#11141a] px-4 py-2"><h2 className="text-lg font-semibold">Pit wall · Drivers</h2><span className="text-sm text-neutral-400">{s.context ? "Live · selected session" : "Waiting for telemetry"}</span></div>
    <table className="w-full mt-3 text-sm border border-[#232830]"><thead className="bg-[#151920] text-neutral-400"><tr><th className="text-left p-2">Pos</th><th className="text-left p-2">Car</th><th className="text-right p-2">Speed</th><th className="text-right p-2">Wear</th><th className="text-right p-2">Fuel</th><th className="text-right p-2">Tyre</th></tr></thead>
      <tbody>{cars.map((i) => <tr key={i} className="border-t border-[#232830]"><td className="p-2">{text(metric(s, "timing.car_position", i))}</td><td className="p-2">Car {i}{i === s.context?.player_car_index ? " · You" : ""}</td><td className="p-2 text-right tabular-nums">{text(metric(s, "car.speed_mps", i))}</td><td className="p-2 text-right tabular-nums">{text(metric(s, "tyre.fl.wear_pct", i))}</td><td className="p-2 text-right tabular-nums">{text(metric(s, "fuel.finish_margin_laps", i))}</td><td className="p-2 text-right">{text(metric(s, "tyre.compound", i))}</td></tr>)}</tbody></table>
    
  </div>;
}
function History() {
  const [items, setItems] = useState<any[]>([]); const [season, setSeason] = useState<string>("2026"); const [error, setError] = useState("");
  useEffect(() => { const q = season === "unassigned" ? "" : "?season=" + season; api<any>("/sessions" + q).then((d) => setItems(d.items ?? [])).catch((e) => setError(String(e))); }, [season]);
  return <div className="min-h-screen bg-[#0b0d11] text-neutral-100 p-4"><div className="flex gap-2">{["2025", "2026", "unassigned"].map((t) => <button key={t} onClick={() => setSeason(t)} className={nav + (season === t ? " bg-[#1d3b2c] text-[#4fd48a]" : " bg-[#151920] text-neutral-300")}>{t === "unassigned" ? "Unassigned" : t + " season"}</button>)}</div>
    {error && <p role="alert" className="mt-3 text-[#f2483c]">{error}</p>}
    <ul className="mt-3 divide-y divide-[#232830] border border-[#232830]">{items.length === 0 ? <li className="p-4 text-neutral-400">No recorded sessions in this view.</li> : items.map((s) => <li key={s.id} className="p-3 flex justify-between"><span>{s.display_name || s.id} · track {s.track_id ?? "unknown"}</span><span className="text-neutral-400">{s.recording_state} · {s.completeness}</span></li>)}</ul>
    </div>;
}
function Analysis() {
  const [sessions, setSessions] = useState<any[]>([]); const [series, setSeries] = useState<any[]>([]); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => { api<any>("/sessions").then((d) => setSessions(d.items ?? [])).catch((e) => setError(String(e))); }, []);
  async function run() {
    if (!sessions[0]) return; setBusy(true); setError("");
    try { const s = sessions[0]; const d = await api<any>("/sessions/" + s.id); const r = await api<any>("/analysis/query", { method: "POST", body: JSON.stringify({ session_id: s.id, branch_id: d.surviving_branch_id, car_indices: [0], channels: ["car.speed_mps"], from_session_time_ms: 0, to_session_time_ms: 1e12, max_points: 300, alignment: "time" }) });
      setSeries((r.series[0]?.points ?? []).map((p: any) => ({ t: Number(p.session_time_ms ?? 0) / 1000, v: p.measurement.value })));
    } catch (e) { setError(String(e)); } finally { setBusy(false); }
  }
  return <div className="min-h-screen bg-[#0b0d11] text-neutral-100 p-4"><div className="flex gap-3 items-center"><button onClick={run} disabled={busy} className="bg-[#1d3b2c] text-[#4fd48a] px-3 py-1.5 text-sm">{busy ? "Loading…" : "Load selected session"}</button><span className="text-sm text-neutral-400">{sessions[0]?.id ?? "No session"}</span></div>
    {error && <p role="alert" className="mt-3 text-[#f2483c]">{error}</p>}
    <div className="h-[420px] mt-4 border border-[#232830] bg-[#11141a] p-3"><ResponsiveContainer width="100%" height="100%"><LineChart data={series}><XAxis dataKey="t" stroke="#777" /><YAxis stroke="#777" /><Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} /><Line dataKey="v" stroke="#4fd48a" dot={false} /></LineChart></ResponsiveContainer></div>
    </div>;
}
function Setups() {
  const [items, setItems] = useState<any[]>([]); const [error, setError] = useState("");
  useEffect(() => { api<any>("/setups").then((d) => setItems(d.items ?? [])).catch((e) => setError(String(e))); }, []);
  return <div className="min-h-screen bg-[#0b0d11] text-neutral-100 p-4"><h2 className="text-lg">Setups</h2>{error && <p className="text-[#f2483c]">{error}</p>}{items.length === 0 ? <p className="mt-3 text-neutral-400">No setups saved.</p> : items.map((r) => <article key={r.id} className="mt-3 border border-[#232830] p-3"><p>{r.output?.driver_message ?? "Saved recommendation"}</p><p className="text-[12px] text-neutral-500">{r.sources?.length ?? 0} sources · {r.experiments?.length ?? 0} experiments</p></article>)}</div>;
}
function Settings() {
  const [cfg, setCfg] = useState<any>(null); const [error, setError] = useState("");
  useEffect(() => { api<any>("/config").then(setCfg).catch((e) => setError(String(e))); }, []);
  return <div className="min-h-screen bg-[#0b0d11] text-neutral-100 p-4"><h2 className="text-lg">Settings</h2>{error && <p className="text-[#f2483c]">{error}</p>}{cfg && <dl className="mt-3 grid grid-cols-2 gap-2 text-sm"><dt className="text-neutral-400">HTTP</dt><dd>{cfg.active_config.http.host}:{cfg.active_config.http.port}</dd><dt className="text-neutral-400">UDP</dt><dd>{cfg.active_config.udp.host}:{cfg.active_config.udp.port}</dd><dt className="text-neutral-400">Recording</dt><dd>{cfg.active_config.recording.enabled ? "enabled" : "disabled"} · cap {Math.round(cfg.active_config.recording.cap_bytes / 1e9)} GB</dd><dt className="text-neutral-400">AI endpoint</dt><dd>{cfg.active_config.agent.endpoint_id ?? "not configured"}</dd><dt className="text-neutral-400">Search</dt><dd>{cfg.active_config.search.provider}</dd><dt className="text-neutral-400">Voice</dt><dd>{cfg.active_config.voice.enabled ? "enabled" : "disabled"}</dd></dl>}</div>;
}
export default function Index() {
  const [tab, setTab] = useState<Tab>("drive");
  useEffect(() => { document.title = "F1 Telemetry"; }, []);
  return <div className="min-h-screen bg-[#0b0d11]"><nav aria-label="Workspaces" className="flex gap-1 border-b border-[#232830] bg-[#0d1015] px-3 py-2">{tabs.map((t) => <button key={t.id} onClick={() => setTab(t.id)} aria-current={tab === t.id ? "page" : undefined} className={nav + (tab === t.id ? " bg-[#1d3b2c] text-[#4fd48a]" : " text-neutral-300 hover:bg-[#151920]")}>{t.label}</button>)}</nav>
    {tab === "drive" ? <Drive /> : tab === "pit" ? <PitWall /> : tab === "history" ? <History /> : tab === "analysis" ? <Analysis /> : tab === "setups" ? <Setups /> : <Settings />}</div>;
}
