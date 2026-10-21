/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LineChart, Line, ResponsiveContainer, XAxis, YAxis, Tooltip, ReferenceLine } from "recharts";
import { activeCars, api, metric, text, useLive, value, type Snapshot } from "@/lib/live";
import { sessionMeta } from "@/lib/recorded";
import { clearSelection, select, toggleCarSelection, useSelection, type Selection } from "@/lib/selection";
import { EngineerCard } from "@/components/EngineerCard";
import { CompareGrid } from "@/components/CompareGrid";
import { TraceStack, type TraceSeries } from "@/components/TraceStack";
import { useLiveSamples } from "@/lib/liveSamples";
import { teamInk } from "@/lib/teams";
import { paceRows, useLiveLaps, type LiveLap } from "@/lib/liveLaps";

export const nav =
  "px-3 py-1.5 text-sm rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#4fd48a]";
const panel = "border border-[#232830] bg-[#11141a]";
const label = "text-[11px] uppercase tracking-wider text-neutral-400";
function LiveTraces({ s, selected, points }: { s: Snapshot; selected: number[]; points: Map<number, TraceSeries["points"]> }) {
  const series: TraceSeries[] = selected.map((car) => ({
    key: "c" + car,
    name: seat(s, car),
    color: teamInk(value(metric(s, "car.team_id", car))),
    points: points.get(car) ?? [],
  }));
  const empty = !s.context
    ? "Waiting for telemetry. The first completed lap starts the traces."
    : "The first completed lap will populate the traces.";
  return <TraceStack series={series} season={s.context?.regulation_season ?? null} live empty={empty} />;
}

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
  const [picked, setPicked] = useState<number[] | null>(null);
  const [recordedLaps, setRecordedLaps] = useState<LiveLap[]>([]);
  const [candidates, setCandidates] = useState<Candidate[] | null>(null);
  const [strategyError, setStrategyError] = useState("");
  const player = s.context?.player_car_index ?? 0;
  const paused = text(metric(s, "session.paused")) === "true";
  const selected = picked ?? [player];
  const selectedCar = selected[0] ?? player;

  const toggleCar = (car: number) => setPicked(toggleCarSelection(selected, car));
  const live = useLiveSamples(s.context?.session_id ?? null, s.context?.branch_id ?? null, selected, !sel.sessionId && !!s.context);
  const { laps: liveLaps } = useLiveLaps(s.context?.session_id ?? null, s.context?.branch_id ?? null, !sel.sessionId && !!s.context);
  const laps = sel.sessionId ? recordedLaps : liveLaps;

  const cars = useMemo(() => {
    return activeCars(s);
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
      setRecordedLaps([]);
      return;
    }
    api<any>(`/sessions/${sel.sessionId}/laps?branch=${encodeURIComponent(sel.branchId ?? "")}`)
      .then((d) => setRecordedLaps(d.items ?? []))
      .catch(() => setRecordedLaps([]));
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
  const currentLapNo = value(metric(s, "timing.current_lap_num", selectedCar));
  const currentLapMs = value(metric(s, "timing.current_lap_ms", selectedCar));
  const pace = paceRows(laps, selectedCar, currentLapNo, currentLapMs, !sel.sessionId);

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
          {sel.sessionId ? "Recorded session" : paused ? "Paused" : s.context ? "Live" : "Waiting for telemetry"}
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
                  const active = selected.includes(i);
                  const color = teamInk(value(metric(s, "car.team_id", i)));
                  return (
                    <tr key={i} onClick={() => toggleCar(i)} aria-selected={active} tabIndex={0} role="button"
                      aria-label={(active ? "Deselect " : "Compare ") + seat(s, i)}
                      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggleCar(i); } }}
                      className={"cursor-pointer border-t border-[#232830] focus-visible:outline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[#4fd48a] " + (active ? "bg-[#18211b]" : "hover:bg-[#151920]")}>
                      <td className="p-2 tabular-nums">{text(metric(s, "timing.car_position", i))}</td>
                      <td className="p-2" style={{ boxShadow: active ? "inset 3px 0 0 " + color : undefined }}><span style={{ color: active ? color : undefined }}>{seat(s, i)}</span>{i === player ? <span className="ml-2 rounded-sm bg-[#24362a] px-1 text-[10px] text-[#8ef0b4]">You</span> : null}</td>
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
            <div className={label}>Selected cars</div>
            <CompareGrid s={s} cars={selected} live={live.points} />
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
        </section>
      </div>

      <LiveTraces s={s} selected={selected} points={live.points} />

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
        <section className={panel + " p-4 self-start"}>
          <div className="flex items-baseline justify-between"><div className={label}>Pace by lap</div><span className="text-[11px] text-neutral-400">{pace.length} eligible laps</span></div>
          <div className="h-[120px] mt-2">
            {pace.length === 0 ? <Empty>Waiting for a completed lap.</Empty> : (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={pace}><XAxis dataKey="lap" stroke="#6b7280" tick={{ fontSize: 11 }} /><YAxis hide domain={["dataMin - 200", "dataMax + 200"]} />
                  <Tooltip contentStyle={{ background: "#11141a", border: "1px solid #232830" }} />
                  <Line dataKey="ms" stroke="#5fd68f" dot={pace.length <= 2} strokeWidth={2} connectNulls={false} />
                  <Line dataKey="prov" stroke="#5fd68f" strokeDasharray="4 3" dot={pace.length <= 2} strokeWidth={2} connectNulls={false} />
                </LineChart>
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
  const [kind, setKind] = useState<"all" | "race" | "qualifying" | "practice">("all");
  const shown = items.filter((s: any) => {
    if (kind === "all") return true;
    const t = s.session_type ?? "unknown";
    if (kind === "qualifying") return t.startsWith("qualifying");
    return t === kind;
  });
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
      <div className="flex items-center gap-2" role="tablist" aria-label="Session type">
        {(["all", "race", "qualifying", "practice"] as const).map((t) => (
          <button key={t} role="tab" aria-selected={kind === t} onClick={() => setKind(t)}
            className={nav + (kind === t ? " bg-[#1d3b2c] text-[#4fd48a]" : " bg-[#151920] text-neutral-300")}>
            {t === "all" ? "All sessions" : t[0].toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>
      {error ? <p role="alert" className="text-[#f2483c]">{error}</p> : null}
      {notice ? <p ref={noticeRef} role="status" tabIndex={-1} className="text-[13px] text-neutral-300">{notice}</p> : null}
      <ul className={panel + " divide-y divide-[#232830]" }>
        {shown.length === 0 ? <li><Empty>No recorded sessions in this view. Start a session with UDP telemetry enabled.</Empty></li> : null}
        {shown.map((s: any, i: number) => {
          const prev = i > 0 ? shown[i - 1] : null;
          const meta = sessionMeta(s);
          const qual = String(s.session_type ?? "").startsWith("qualifying");
          const groupStart = qual && s.weekend_link_id && (!prev || prev.weekend_link_id !== s.weekend_link_id);
          return (
          <>
          {groupStart ? <li key={s.id + "-group"} className="list-none"><div className="bg-[#151920] px-4 py-1.5 text-[11px] uppercase tracking-wider text-neutral-400">Qualifying weekend {String(s.weekend_link_id)}</div></li> : null}
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
                  {(s.session_type ?? "unknown").replace("qualifying_", "Q").replace("practice", "Practice").replace("race", "Race")} · {meta.circuit} · {meta.date} · {meta.duration} · {s.completed_laps} laps · best {s.best_lap_ms ? (s.best_lap_ms / 1000).toFixed(3) + " s" : "—"}
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
          </>
          );
        })}
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

const sourceText: Record<string, string> = { cli: "Command line", env: "Environment", file: "Config file", default: "Default" };
const gb = (n: number) => (n / 1e9).toFixed(n % 1e9 === 0 ? 0 : 1) + " GB";

function ConfigRow({ name, value, source, restartOnly, restart }: { name: string; value: string; source?: string; restartOnly?: boolean; restart?: boolean }) {
  return (
    <div className="flex items-baseline gap-3 border-b border-[#1c2128] py-1.5 last:border-b-0">
      <dt className="w-44 shrink-0 text-neutral-400">{name}</dt>
      <dd className="min-w-0 flex-1 text-neutral-200 [overflow-wrap:anywhere]">{value}</dd>
      {source ? <span className="shrink-0 rounded-sm border border-[#2a3038] px-1.5 text-[11px] text-neutral-400">{sourceText[source] ?? source}</span> : null}
      {restartOnly ? <span className={restart ? "shrink-0 rounded-sm bg-[#3a2a1a] px-1.5 text-[11px] text-[#f0a03c]" : "shrink-0 text-[11px] text-neutral-500"}>{restart ? "Restart to apply" : "Changes need restart"}</span> : null}
    </div>
  );
}

function ConfigSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className={panel + " min-w-0 flex-1 basis-[32rem]"}>
      <h3 className="border-b border-[#232830] px-4 py-2 text-[11px] uppercase tracking-wider text-neutral-400">{title}</h3>
      <dl className="p-4 pt-1 text-sm">{children}</dl>
    </section>
  );
}

export function Settings() {
  const [cfg, setCfg] = useState<any>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    setLoading(true);
    api<any>("/config")
      .then((c) => { setCfg(c); setError(""); })
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, []);
  const c = cfg?.active_config;
  const sources: Record<string, string> = cfg?.sources ?? {};
  const restart: string[] = cfg?.restart_required ?? [];
  const agent = c?.agent;
  const endpoint = agent?.endpoints?.find((e: any) => e.id === agent.endpoint_id);
  const secretName = endpoint?.api_key_env ?? null;
  return (
    <div className="min-h-[calc(100vh-49px)] bg-[#0b0d11] p-4 text-neutral-100">
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-lg">Settings</h2>
        {cfg ? <span className="text-[12px] text-neutral-400">Revision {cfg.revision} · effective values</span> : null}
      </div>
      {error ? (
        <div role="alert" className={panel + " p-4 text-sm"}>
          <p className="text-[#f25a5c]">{error}</p>
          <p className="mt-1 text-neutral-400">Settings could not be read. Check that the server is running and reload.</p>
        </div>
      ) : null}
      {loading && !cfg ? <p className="text-sm text-neutral-400">Loading configuration…</p> : null}
      {c ? (
        <div className="flex flex-wrap gap-3">
          <ConfigSection title="Server">
            <ConfigRow name="Dashboard" value={`${c.http.host}:${c.http.port}`} source={sources.http} restartOnly restart={restart.includes("http")} />
            <ConfigRow name="Telemetry (UDP)" value={`${c.udp.host}:${c.udp.port}`} source={sources.udp} restartOnly restart={restart.includes("udp")} />
            <ConfigRow name="Game address" value={cfg.udp_advertise_host ? `${cfg.udp_advertise_host}:${c.udp.port}` : "unknown"} source={sources.udp_advertise_host} />
            <ConfigRow name="Data directory" value={c.data_dir ?? "default"} source={sources.data_dir} restartOnly restart={restart.includes("data_dir")} />
          </ConfigSection>

          <ConfigSection title="Recording">
            <ConfigRow name="Recording" value={c.recording.enabled ? "Enabled" : "Disabled"} source={sources["recording.enabled"]} />
            <ConfigRow name="Storage cap" value={gb(c.recording.cap_bytes)} source={sources["recording.cap_bytes"]} />
            <ConfigRow name="Free reserve" value={gb(c.recording.free_reserve_bytes)} source={sources["recording.free_reserve_bytes"]} />
            <ConfigRow name="Chunk size" value={`${(c.recording.chunk_bytes / 1048576).toFixed(0)} MiB`} source={sources["recording.chunk_bytes"]} />
            <ConfigRow name="Rotation size" value={`${(c.recording.rotate_bytes / 1048576).toFixed(0)} MiB`} source={sources["recording.rotate_bytes"]} />
            <ConfigRow name="Checkpoint cadence" value={`${c.recording.chunk_flush_ms} ms`} source={sources["recording.chunk_flush_ms"]} />
          </ConfigSection>

          <ConfigSection title="Race engineer">
            <ConfigRow name="Enabled" value={agent?.enabled ? "Yes" : "No"} source={sources["agent.enabled"]} />
            <ConfigRow name="Endpoint" value={agent?.endpoint_id ?? "not configured"} source={sources["agent.endpoint_id"]} />
            <ConfigRow name="URL" value={endpoint?.base_url || "not configured"} source={endpoint ? sources["agent.endpoint"] : undefined} />
            <ConfigRow name="Model" value={endpoint?.model || "not configured"} />
            <ConfigRow name="Protocol" value={endpoint?.protocol || "not configured"} />
            <ConfigRow name="API key" value={secretName ? (cfg.secrets_configured?.[secretName] ? `Configured (${secretName})` : `Missing: ${secretName}`) : "not configured"} />
          </ConfigSection>

          <ConfigSection title="Search and voice">
            <ConfigRow name="Search provider" value={c.search.provider} source={sources["search.provider"]} />
            <ConfigRow name="Search URL" value={c.search.base_url || "provider default"} source={sources["search.base_url"]} />
            <ConfigRow name="Voice" value={c.voice.enabled ? "Enabled" : "Disabled"} source={sources["voice.enabled"]} />
            <ConfigRow name="Voice URL" value={c.voice.transcription_endpoint || "not configured"} source={sources["voice.base_url"]} />
          </ConfigSection>

          <ConfigSection title="Deployment">
            <ConfigRow name="Configuration file" value="set F1TELE_CONFIG or keep it in the data directory" />
            <ConfigRow name="Game on another machine" value="Set F1TELE_UDP_ADVERTISE_HOST and enter the game address above" />
            <ConfigRow name="Restart required" value={restart.length ? restart.join(", ") : "None pending"} />
          </ConfigSection>
        </div>
      ) : null}
    </div>
  );
}
