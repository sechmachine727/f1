/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip, ReferenceLine } from "recharts";
import { api, availabilityLabel, metric, text, useLive, value, type Snapshot } from "@/lib/live";
import { clearSelection, select, useSelection } from "@/lib/selection";
import { EngineerCard } from "@/components/EngineerCard";

export const nav =
  "px-3 py-1.5 text-sm rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#4fd48a]";
const panel = "border border-[#232830] bg-[#11141a]";
const label = "text-[11px] uppercase tracking-wider text-neutral-400";
const wheel = ["fl", "fr", "rl", "rr"] as const;
const wheelLabel: Record<string, string> = { fl: "FL", fr: "FR", rl: "RL", rr: "RR" };

function seat(s: Snapshot, i: number): string {
  const name = text(metric(s, "car.driver_name", i));
  return name && name !== "—" ? name : `Car ${i}`;
}

function kmh(m: ReturnType<typeof metric>): string {
  const v = value(m);
  return v == null ? "—" : String(Math.round(v * 3.6));
}
function pct(m: ReturnType<typeof metric>): string {
  const v = value(m);
  return v == null ? "—" : v.toFixed(1);
}
function lapCount(m: ReturnType<typeof metric>): string {
  const v = value(m);
  return v == null ? "—" : v.toFixed(1);
}

function Banner({ label: text, detail }: { label: string; detail?: string }) {
  return (
    <div className="flex items-center gap-3 border border-[#3b4a2f] bg-[#16201a] px-4 py-2 text-sm">
      <span className="rounded-sm bg-[#2a4a33] px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider text-[#8ef0b4]">Recorded</span>
      <span className="text-neutral-200">{text}</span>
      {detail ? <span className="text-neutral-400">{detail}</span> : null}
      <button className={"ml-auto " + nav + " text-neutral-300 hover:bg-[#1a2027]"} onClick={clearSelection}>
        Back to live
      </button>
    </div>
  );
}

function Empty({ children }: { children: string }) {
  return <p className="px-4 py-6 text-sm text-neutral-400">{children}</p>;
}

type Candidate = {
  id: string; stops: { lap: number; compound: string }[]; legality: string; reasons: string[] | null;
  total_time_ms: { availability: string; value: number | null }; pit_loss_ms: { availability: string; value: number | null };
};

export function PitWall() {
  const s = useLive();
  const sel = useSelection();
  const [view, setView] = useState<"drivers" | "teams">("drivers");
  const [detail, setDetail] = useState<number | null>(null);
  const [laps, setLaps] = useState<any[]>([]);
  const [candidates, setCandidates] = useState<Candidate[] | null>(null);
  const [strategyError, setStrategyError] = useState("");
  const player = s.context?.player_car_index ?? 0;
  const selectedCar = detail ?? player;

  const cars = useMemo(() => {
    const seen: number[] = [];
    for (let i = 0; i < 24; i++) {
      if (metric(s, "car.speed_mps", i) || metric(s, "car.team_id", i) || i === player) seen.push(i);
    }
    return seen;
  }, [s.seq, player]);

  // Team totals only sum cars that actually reported a value; coverage is shown.
  const teams = useMemo(() => {
    const byTeam = new Map<string, { cars: number[]; wear: number[]; speed: number[] }>();
    for (const i of cars) {
      const team = text(metric(s, "car.team_id", i));
      const key = team === "—" ? "unknown" : team;
      const entry = byTeam.get(key) ?? { cars: [], wear: [], speed: [] };
      entry.cars.push(i);
      const w = value(metric(s, "tyre.fl.wear_pct", i));
      if (w != null) entry.wear.push(w);
      const v = value(metric(s, "car.speed_mps", i));
      if (v != null) entry.speed.push(v);
      byTeam.set(key, entry);
    }
    return [...byTeam.entries()];
  }, [s.seq, cars]);

  useEffect(() => {
    if (!sel.sessionId) {
      setLaps([]);
      return;
    }
    api<any>(`/sessions/${sel.sessionId}/laps?branch=${encodeURIComponent(sel.branchId ?? "")}`)
      .then((d) => setLaps(d.items ?? []))
      .catch(() => setLaps([]));
  }, [sel.sessionId, sel.branchId]);

  const requestPit = useCallback(async () => {
    if (!s.context || sel.sessionId) return;
    setStrategyError("");
    const lap = Number(text(metric(s, "timing.current_lap_num", player)));
    const first = Number.isFinite(lap) && lap > 0 ? lap + 2 : 2;
    try {
      const res = await api<any>("/strategy/evaluate", {
        method: "POST",
        body: JSON.stringify({
          session_id: s.context.session_id, branch_id: s.context.branch_id,
          observation_seq: String(s.seq), pit_windows: [
            { first_lap: first, last_lap: first + 2, compound: "C3" },
            { first_lap: first + 4, last_lap: first + 6, compound: "C4" },
          ],
        }),
      });
      setCandidates(res.candidates ?? []);
    } catch (e) {
      setStrategyError(String(e));
      setCandidates(null);
    }
  }, [s.context, s.seq, sel.sessionId, player]);

  // The stint strip comes from catalogued laps for the selected branch.
  const stint = laps.filter((l) => l.car_index === selectedCar).slice(-12);
  const pace = stint.filter((l) => l.lap_time_ms?.availability === "available").map((l) => ({ lap: l.lap, ms: l.lap_time_ms.value as number }));

  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 p-4 space-y-3">
      {sel.sessionId ? <Banner label={sel.label || sel.sessionId} detail={sel.season ? String(sel.season) : undefined} /> : null}
      <div className={panel + " flex items-center justify-between px-4 py-2"}>
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-semibold">Pit wall</h2>
          <div className="flex gap-1" role="tablist" aria-label="Comparison view">
            {(["drivers", "teams"] as const).map((t) => (
              <button key={t} role="tab" aria-selected={view === t} onClick={() => setView(t)}
                className={nav + (view === t ? " bg-[#1d3b2c] text-[#4fd48a]" : " text-neutral-300 hover:bg-[#151920]")}>
                {t === "drivers" ? "Drivers" : "Teams"}
              </button>
            ))}
          </div>
        </div>
        <span className="text-sm text-neutral-400">
          {sel.sessionId ? "Recorded · selection drives history views" : s.context ? "Live" : "Waiting for telemetry"}
        </span>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] gap-3">
        <section className={panel}>
          {view === "drivers" ? (
            <table className="w-full text-sm">
              <thead className="bg-[#151920] text-neutral-400">
                <tr>{["Pos", "Driver", "Gap", "Speed", "Wear", "Compound", "Fuel"].map((h, i) => (
                  <th key={h} className={"p-2 " + (i < 2 ? "text-left" : "text-right")}>{h}</th>
                ))}</tr>
              </thead>
              <tbody>
                {cars.length === 0 ? <tr><td colSpan={7}><Empty>No cars reported yet. Start the game with UDP enabled.</Empty></td></tr> : null}
                {cars.map((i) => {
                  const active = i === selectedCar;
                  return (
                    <tr key={i} onClick={() => setDetail(i)} aria-selected={active}
                      className={"cursor-pointer border-t border-[#232830] " + (active ? "bg-[#18211b]" : "hover:bg-[#151920]")}>
                      <td className="p-2 tabular-nums">{text(metric(s, "timing.car_position", i))}</td>
                      <td className="p-2">{seat(s, i)}{i === player ? <span className="ml-2 rounded-sm bg-[#24362a] px-1 text-[10px] text-[#8ef0b4]">You</span> : null}</td>
                      <td className="p-2 text-right tabular-nums">{text(metric(s, "car.gap_ahead_s", i))}</td>
                      <td className="p-2 text-right tabular-nums">{kmh(metric(s, "car.speed_mps", i))}</td>
                      <td className="p-2 text-right tabular-nums">{pct(metric(s, "tyre.fl.wear_pct", i))}</td>
                      <td className="p-2 text-right">{text(metric(s, "tyre.compound", i))}</td>
                      <td className="p-2 text-right tabular-nums">{lapCount(metric(s, "fuel.finish_margin_laps", i))}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-[#151920] text-neutral-400"><tr><th className="p-2 text-left">Team</th><th className="p-2 text-left">Cars</th><th className="p-2 text-right">Coverage</th><th className="p-2 text-right">Mean wear</th><th className="p-2 text-right">Mean speed</th></tr></thead>
              <tbody>
                {teams.length === 0 ? <tr><td colSpan={5}><Empty>No team identifiers reported yet.</Empty></td></tr> : null}
                {teams.map(([team, entry]) => (
                  <tr key={team} className="border-t border-[#232830]">
                    <td className="p-2">{team === "unknown" ? "Unknown team" : `Team ${team}`}</td>
                    <td className="p-2 text-neutral-400">{entry.cars.map((i) => seat(s, i)).join(", ")}</td>
                    <td className="p-2 text-right tabular-nums">{entry.wear.length}/{entry.cars.length} wear</td>
                    <td className="p-2 text-right tabular-nums">{entry.wear.length ? (entry.wear.reduce((a, b) => a + b, 0) / entry.wear.length).toFixed(1) : "—"}</td>
                    <td className="p-2 text-right tabular-nums">{entry.speed.length ? (entry.speed.reduce((a, b) => a + b, 0) / entry.speed.length).toFixed(1) : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className={panel + " p-4 space-y-3"}>
          <div>
            <div className={label}>Selected driver</div>
            <div className="text-xl font-semibold">{seat(s, selectedCar)}</div>
            <div className="text-[12px] text-neutral-400">
              {availabilityLabel(metric(s, "car.speed_mps", selectedCar))} · compound {text(metric(s, "tyre.compound", selectedCar))}
            </div>
          </div>
          <div>
            <div className={label}>Stint</div>
            {stint.length === 0 ? (
              <p className="text-[12px] text-neutral-400">No catalogued laps for this car in the selected session.</p>
            ) : (
              <div className="flex gap-1" role="list" aria-label="Last laps">
                {stint.map((l) => (
                  <span key={l.id} role="listitem" title={l.lap_time_ms?.availability === "available" ? `${Math.round(l.lap_time_ms.value)} ms` : "invalid or incomplete"}
                    className={"h-6 w-6 rounded-sm text-[10px] leading-6 text-center " + (l.valid && l.complete ? "bg-[#2a4a33] text-[#c7f5d6]" : "bg-[#3a2a2a] text-[#f3c2c2]")}>
                    {l.lap}
                  </span>
                ))}
              </div>
            )}
          </div>
          <EngineerCard />
          <div>
            <div className={label}>Wear by corner</div>
            <div className="grid grid-cols-4 gap-2 mt-1">
              {wheel.map((w) => (
                <div key={w} className="tabular-nums"><div className="text-[10px] text-neutral-400">{wheelLabel[w]}</div>{text(metric(s, `tyre.${w}.wear_pct`, selectedCar))}</div>
              ))}
            </div>
          </div>
        </section>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        <section className={panel + " p-4 self-start"}>
          <div className="flex items-baseline justify-between"><div className={label}>Pace by lap</div><span className="text-[11px] text-neutral-400">{pace.length} eligible laps</span></div>
          <div className="h-[120px] mt-2">
            {pace.length < 2 ? <Empty>Needs two catalogued laps.</Empty> : (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={pace}><XAxis dataKey="lap" stroke="#6b7280" tick={{ fontSize: 11 }} /><YAxis hide domain={["dataMin - 200", "dataMax + 200"]} />
                  <Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} /><Line dataKey="ms" stroke="#5fd68f" dot={false} strokeWidth={2} /></LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </section>
        <section className={panel + " p-4 self-start"}>
          <div className="flex items-baseline justify-between"><div className={label}>Pit alternatives</div>
            <button onClick={requestPit} disabled={!!sel.sessionId || !s.context} className={nav + " bg-[#1d3b2c] text-[#4fd48a] disabled:opacity-40"}>Evaluate</button>
          </div>
          {strategyError ? <p role="alert" className="mt-2 text-[12px] text-[#f2483c]">{strategyError}</p> : null}
          {candidates === null ? <Empty>{sel.sessionId ? "Evaluate on the live session." : "No evaluation yet."}</Empty> : (
            <ul className="mt-2 divide-y divide-[#232830]">
              {candidates.map((c) => (
                <li key={c.id} className="py-2 text-sm">
                  <div className="flex justify-between"><span>{!c.stops?.length ? "Stay out" : c.stops.map((st) => `L${st.lap} ${st.compound}`).join(" · ")}</span>
                    <span className={c.legality === "legal" ? "text-[#5fd68f]" : c.legality === "conditional" ? "text-[#f0a03c]" : "text-[#f25a5c]"}>{c.legality}</span></div>
                  <div className="text-[12px] text-neutral-400">
                    {c.total_time_ms.availability === "available" ? `${Math.round(c.total_time_ms.value as number)} ms projected` : "Projection unavailable"}
                    {" · pit loss "}{c.pit_loss_ms.availability === "available" ? `${Math.round(c.pit_loss_ms.value as number)} ms` : "unavailable"}
                  </div>
                  {c.reasons?.length ? <div className="text-[12px] text-neutral-400">{c.reasons.join("; ")}</div> : null}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}

function useSessions(season: string) {
  const [items, setItems] = useState<any[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    const q = season === "unassigned" ? "" : "?season=" + season;
    api<any>("/sessions" + q)
      .then((d) => {
        setItems(d.items ?? []);
        setError("");
      })
      .catch((e) => setError(String(e)));
  }, [season]);
  return { items, error };
}

export function History({ onOpen }: { onOpen: () => void }) {
  const [season, setSeason] = useState("2026");
  const { items, error } = useSessions(season);
  const sel = useSelection();
  const [notice, setNotice] = useState("");
  const [confirming, setConfirming] = useState<string | null>(null);
  const cleanupRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const noticeRef = useRef<HTMLParagraphElement>(null);

  // Cleanup is manual and per-session: it lists the catalogued recording files,
  // asks for confirmation, and reports per-file refusals from the server.
  async function removeRecording(sessionId: string) {
    setNotice("");
    try {
      const detail = await api<any>("/sessions/" + sessionId);
      const ids = (detail.recordings ?? []).map((r: any) => r.id);
      if (ids.length === 0) {
        setNotice("This session has no recording files.");
        return;
      }
      const res = await api<any>("/history/cleanup", {
        method: "POST",
        body: JSON.stringify({ recording_ids: ids, session_ids: [], confirmed: true }),
      });
      const failed = res.failures ?? [];
      setNotice(
        failed.length
          ? "Removed " + res.deleted_ids.length + "; kept " + failed.length + ": " + failed.map((f: any) => f.message).join("; ")
          : "Removed " + res.deleted_ids.length + " recording file" + (res.deleted_ids.length === 1 ? "" : "s") + ".",
      );
    } catch (e) {
      setNotice(String(e));
    } finally {
      setConfirming(null);
      requestAnimationFrame(() => noticeRef.current?.focus());
    }
  }

  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 p-4 space-y-3">
      <div className="flex items-center gap-2" role="tablist" aria-label="Season">
        {["2025", "2026", "unassigned"].map((t) => (
          <button key={t} role="tab" aria-selected={season === t} onClick={() => setSeason(t)}
            className={nav + (season === t ? " bg-[#1d3b2c] text-[#4fd48a]" : " bg-[#151920] text-neutral-300")}>
            {t === "unassigned" ? "Unassigned" : t + " season"}
          </button>
        ))}
      </div>
      {error ? <p role="alert" className="text-[#f2483c]">{error}</p> : null}
      {notice ? <p ref={noticeRef} role="status" tabIndex={-1} className="text-[13px] text-neutral-300">{notice}</p> : null}
      <ul className={panel + " divide-y divide-[#232830]" }>
        {items.length === 0 ? <li><Empty>No recorded sessions in this view.</Empty></li> : null}
        {items.map((s) => (
          <li key={s.id} className="flex items-stretch">
            <button
              className="flex w-full items-center justify-between px-4 py-3 text-left hover:bg-[#151920] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#4fd48a]"
              aria-pressed={sel.sessionId === s.id}
              onClick={async () => {
                const detail = await api<any>("/sessions/" + s.id);
                select({
                  sessionId: s.id,
                  branchId: detail.surviving_branch_id,
                  label: s.display_name || "Session " + s.id.slice(0, 8),
                  season: s.regulation_season,
                  recorded: true,
                });
                onOpen();
              }}
            >
              <span>
                <span className="block">{s.display_name || "Session " + s.id.slice(0, 8)}</span>
                <span className="text-[12px] text-neutral-400">
                  track {s.track_id ?? "unknown"} · {s.completed_laps} laps · {s.mode_family}
                </span>
              </span>
              <span className="text-[12px] text-neutral-400">{s.recording_state} · {s.completeness}</span>
            </button>
            {confirming === s.id ? (
              <span className="flex items-center gap-2 px-3 text-[12px]">
                <span className="text-neutral-300">Delete recording files?</span>
                <button className={nav + " bg-[#4a1d22] text-[#ffb4b4]"} onClick={() => removeRecording(s.id)}>Delete</button>
                <button autoFocus className={nav + " text-neutral-300"} onClick={() => { setConfirming(null); requestAnimationFrame(() => cleanupRefs.current[s.id]?.focus()); }}>Cancel</button>
              </span>
            ) : (
              <button
                ref={(el) => { cleanupRefs.current[s.id] = el; }}
                className={nav + " px-3 text-[12px] text-neutral-400 hover:bg-[#151920]"}
                aria-label={"Delete recordings for " + (s.display_name || s.id)}
                onClick={() => setConfirming(s.id)}
              >
                Cleanup
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

export function Analysis() {
  const sel = useSelection();
  const live = useLive();
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [branchId, setBranchId] = useState<string>("");
  const [laps, setLaps] = useState<any[]>([]);
  const [picked, setPicked] = useState<string[]>([]);
  const [series, setSeries] = useState<any[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [job, setJob] = useState<any>(null);
  const { items } = useSessions("");

  useEffect(() => {
    if (!sel.sessionId) return;
    setSessionId(sel.sessionId);
    setBranchId(sel.branchId ?? "");
  }, [sel.sessionId, sel.branchId]);

  useEffect(() => {
    if (!sessionId) return;
    api<any>(`/sessions/${sessionId}/laps?branch=${encodeURIComponent(branchId)}`)
      .then((d) => setLaps(d.items ?? []))
      .catch((e) => setError(String(e)));
  }, [sessionId, branchId]);

  async function compare() {
    if (!sessionId || picked.length < 2) return;
    setBusy(true);
    setError("");
    try {
      const res = await api<any>("/analysis/compare", {
        method: "POST",
        body: JSON.stringify({ session_id: sessionId, branch_id: branchId, lap_ids: picked, channels: ["car.speed_mps"], max_points: 400 }),
      });
      setSeries(res.series ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function exportCsv() {
    if (!sessionId) return;
    setError("");
    try {
      const created = await api<any>("/exports", {
        method: "POST",
        body: JSON.stringify({
          session_id: sessionId, branch_id: branchId,
          range: { from_seq: "0", to_seq: "999999999999" },
          cars: [live.context?.player_car_index ?? 0], channels: ["car.speed_mps"], format: "csv",
        }),
      });
      setJob(created);
      const tick = setInterval(async () => {
        const j = await api<any>("/jobs/" + created.id);
        setJob(j);
        if (j.state === "succeeded" || j.state === "failed" || j.state === "cancelled") clearInterval(tick);
      }, 800);
    } catch (e) {
      setError(String(e));
      setJob(null);
    }
  }

  const merged = useMemo(() => {
    const rows = new Map<number, Record<string, number | null>>();
    for (const s of series) {
      for (const p of s.points ?? []) {
        const d = Math.round(p.distance_m ?? 0);
        const row = rows.get(d) ?? { distance: d };
        row[s.lap_id ?? s.channel] = p.measurement.availability === "available" ? (p.measurement.value as number) : null;
        rows.set(d, row);
      }
    }
    return [...rows.values()].sort((a, b) => (a.distance as number) - (b.distance as number));
  }, [series]);

  const keys = series.map((s) => s.lap_id ?? s.channel);

  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 p-4 space-y-3">
      {sel.sessionId ? <Banner label={sel.label} /> : null}
      <div className={panel + " flex flex-wrap items-center gap-3 px-4 py-2"}>
        <label className="text-sm text-neutral-400" htmlFor="an-session">Session</label>
        <select id="an-session" className="bg-[#151920] px-2 py-1 text-sm" value={sessionId ?? ""}
          onChange={async (e) => {
            const id = e.target.value;
            setPicked([]);
            setSeries([]);
            if (!id) { setSessionId(null); return; }
            const d = await api<any>("/sessions/" + id);
            setSessionId(id);
            setBranchId(d.surviving_branch_id);
          }}>
          <option value="">Select a session</option>
          {items.map((s) => <option key={s.id} value={s.id}>{s.display_name || s.id.slice(0, 8)}</option>)}
        </select>
        <button onClick={compare} disabled={busy || picked.length < 2} className={nav + " bg-[#1d3b2c] text-[#4fd48a] disabled:opacity-40"}>
          {busy ? "Comparing…" : "Compare laps"}
        </button>
        <button onClick={exportCsv} disabled={!sessionId} className={nav + " bg-[#151920] text-neutral-300 disabled:opacity-40"}>Export CSV</button>
        {job ? (
          <span className="text-[12px] text-neutral-400" role="status">
            {job.state}
            {job.error ? ` · ${job.error.message}` : ""}
            {job.download_ids?.length ? <a className="ml-2 text-[#8ef0b4] underline" href={"/api/v1/downloads/" + job.download_ids[0]}>Download</a> : null}
          </span>
        ) : null}
      </div>
      {error ? <p role="alert" className="text-[#f2483c]">{error}</p> : null}
      <div className="grid grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)] gap-3">
        <section className={panel + " max-h-[420px] overflow-auto self-start w-full"}>
          <div className={label + " px-3 pt-3"}>Laps</div>
          {laps.length === 0 ? <Empty>Select a session to list laps.</Empty> : null}
          <ul className="p-2">
            {laps.map((l) => {
              const on = picked.includes(l.id);
              return (
                <li key={l.id}>
                  <button
                    aria-pressed={on}
                    onClick={() => setPicked((p) => (on ? p.filter((x) => x !== l.id) : p.length >= 4 ? p : [...p, l.id]))}
                    className={"flex w-full justify-between px-2 py-1 text-left text-[13px] " + (on ? "bg-[#1d3b2c] text-[#c7f5d6]" : "hover:bg-[#151920]")}>
                    <span>Lap {l.lap} · car {l.car_index}</span>
                    <span className="tabular-nums text-neutral-400">
                      {l.lap_time_ms?.availability === "available" ? `${(l.lap_time_ms.value / 1000).toFixed(3)} s` : l.lap_time_ms?.availability ?? "—"}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </section>
        <section className={panel + " p-3"}>
          <div className="flex items-baseline justify-between"><div className={label}>Distance-aligned speed</div><span className="text-[11px] text-neutral-400">{picked.length}/4 selected</span></div>
          <div className="h-[340px] mt-2">
            {keys.length === 0 ? <Empty>Pick two to four laps, then compare. Invalid laps and gaps stay visible.</Empty> : (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={merged}>
                  <XAxis dataKey="distance" stroke="#6b7280" tick={{ fontSize: 11 }} />
                  <YAxis hide domain={["dataMin - 2", "dataMax + 2"]} />
                  <Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} />
                  <ReferenceLine y={0} stroke="#232830" />
                  {keys.map((k, i) => <Line key={k} dataKey={k} dot={false} strokeWidth={2} stroke={["#5fd68f", "#f0a03c", "#7fb0ff", "#f25a5c"][i % 4]} connectNulls={false} />)}
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </section>
      </div>
      {sessionId ? <ReplayPanel sessionId={sessionId} branchId={branchId} active={items.find((s) => s.id === sessionId)?.recording_state === "active"} /> : null}
    </div>
  );
}

function ReplayPanel({ sessionId, branchId, active }: { sessionId: string; branchId: string; active: boolean }) {
  const [replay, setReplay] = useState<any>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Playback runs on the server; poll the playhead while unpaused so the
  // displayed time follows it without opening a second stream.
  useEffect(() => {
    if (!replay || replay.paused) return;
    const id = setInterval(() => {
      api<any>("/replays/" + replay.id).then(setReplay).catch(() => null);
    }, 500);
    return () => clearInterval(id);
  }, [replay?.id, replay?.paused]);

  async function start() {
    setBusy(true);
    setError("");
    try {
      const r = await api<any>("/replays", {
        method: "POST",
        body: JSON.stringify({ session_id: sessionId, branch_id: branchId, speed: 1, skip_long_pauses: false }),
      });
      setReplay(r);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function patch(p: Record<string, unknown>) {
    if (!replay) return;
    try {
      setReplay(await api<any>("/replays/" + replay.id, { method: "PATCH", body: JSON.stringify(p) }));
    } catch (e) {
      setError(String(e));
    }
  }

  async function close() {
    if (!replay) return;
    await api<any>("/replays/" + replay.id, { method: "DELETE" }).catch(() => null);
    setReplay(null);
  }

  return (
    <section className={panel + " p-4"}>
      <div className="flex items-baseline justify-between">
        <div className={label}>Replay</div>
        {replay ? (
          <span className="text-[12px] text-neutral-400">
            {replay.paused ? "Paused" : "Playing"} · {replay.speed}x · {(replay.playhead_session_time_ms / 1000).toFixed(1)} s
          </span>
        ) : null}
      </div>
      {error ? <p role="alert" className="mt-2 text-[12px] text-[#f2483c]">{error}</p> : null}
      {active ? (
        <p className="mt-2 text-[12px] text-neutral-400">Replay is available after the recording finishes.</p>
      ) : !replay ? (
        <button onClick={start} disabled={busy || !sessionId} className={"mt-2 " + nav + " bg-[#1d3b2c] text-[#4fd48a] disabled:opacity-40"}>
          {busy ? "Opening…" : "Open replay"}
        </button>
      ) : (
        <div className="mt-2 space-y-2">
          <div className="flex gap-2">
            <button className={nav + " bg-[#151920] text-neutral-200"} onClick={() => patch({ paused: !replay.paused })}>
              {replay.paused ? "Play" : "Pause"}
            </button>
            {[0.25, 1, 4, 8].map((s) => (
              <button key={s} onClick={() => patch({ speed: s })}
                className={nav + (replay.speed === s ? " bg-[#1d3b2c] text-[#4fd48a]" : " bg-[#151920] text-neutral-300")}>
                {s}x
              </button>
            ))}
            <button className={nav + " text-neutral-400"} onClick={close}>Close</button>
          </div>
          <label className="block text-[12px] text-neutral-400" htmlFor="replay-seek">Playhead (seconds)</label>
          <input
            id="replay-seek"
            type="range"
            min={0}
            max={Math.max(60, Math.round(replay.playhead_session_time_ms / 1000) + 60)}
            defaultValue={Math.round(replay.playhead_session_time_ms / 1000)}
            onMouseUp={(e) => patch({ position_ms: Number((e.target as HTMLInputElement).value) * 1000 })}
            onKeyUp={(e) => patch({ position_ms: Number((e.target as HTMLInputElement).value) * 1000 })}
            className="w-full accent-[#5fd68f]"
          />
          <p className="text-[11px] text-neutral-400">Replay runs in its own reducer. The live source, recording and live engineer calls are untouched.</p>
        </div>
      )}
    </section>
  );
}

export function Setups() {
  const [items, setItems] = useState<any[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    api<any>("/setups").then((d) => setItems(d.items ?? [])).catch((e) => setError(String(e)));
  }, []);
  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 p-4 space-y-3">
      <h2 className="text-lg">Setups</h2>
      {error ? <p role="alert" className="text-[#f2483c]">{error}</p> : null}
      {items.length === 0 ? <p className="text-neutral-400">No setups saved.</p> : items.map((r) => (
        <article key={r.id} className={panel + " p-4"}>
          <p>{r.output?.driver_message ?? "Saved recommendation"}</p>
          {r.output?.setup_changes?.length ? (
            <ul className="mt-2 text-[13px] text-neutral-300">
              {r.output.setup_changes.map((c: any) => <li key={c.parameter}>{c.parameter}: {c.from_value ?? "—"} → {c.to_value} {c.unit} · {c.handling_effect} ({c.tradeoff})</li>)}
            </ul>
          ) : null}
          <p className="mt-2 text-[12px] text-neutral-400">{r.sources?.length ?? 0} sources · {r.experiments?.length ?? 0} experiments</p>
          {r.experiments?.length ? (
            <ul className="mt-1 text-[12px] text-neutral-400">{r.experiments.map((e: any) => <li key={e.id}>{e.conditions}: {e.outcome}</li>)}</ul>
          ) : null}
        </article>
      ))}
    </div>
  );
}

export function Settings() {
  const [cfg, setCfg] = useState<any>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    api<any>("/config").then(setCfg).catch((e) => setError(String(e)));
  }, []);
  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 p-4 space-y-3">
      <h2 className="text-lg">Settings</h2>
      {error ? <p role="alert" className="text-[#f2483c]">{error}</p> : null}
      {cfg ? (
        <dl className={panel + " grid grid-cols-2 gap-x-6 gap-y-2 p-4 text-sm"}>
          <dt className="text-neutral-400">HTTP</dt><dd>{cfg.active_config.http.host}:{cfg.active_config.http.port}</dd>
          <dt className="text-neutral-400">UDP</dt><dd>{cfg.active_config.udp.host}:{cfg.active_config.udp.port}</dd>
          <dt className="text-neutral-400">Data directory</dt><dd>{cfg.active_config.data_dir}</dd>
          <dt className="text-neutral-400">Recording</dt><dd>{cfg.active_config.recording.enabled ? "enabled" : "disabled"} · cap {Math.round(cfg.active_config.recording.cap_bytes / 1e9)} GB · reserve {Math.round(cfg.active_config.recording.free_reserve_bytes / 1e9)} GB</dd>
          <dt className="text-neutral-400">AI endpoint</dt><dd>{cfg.active_config.agent.endpoint_id ?? "not configured"}</dd>
          <dt className="text-neutral-400">Search</dt><dd>{cfg.active_config.search.provider}</dd>
          <dt className="text-neutral-400">Voice</dt><dd>{cfg.active_config.voice.enabled ? "enabled" : "disabled"}</dd>
          <dt className="text-neutral-400">Restart required</dt><dd>{cfg.restart_required?.length ? cfg.restart_required.join(", ") : "none"}</dd>
        </dl>
      ) : null}
    </div>
  );
}
