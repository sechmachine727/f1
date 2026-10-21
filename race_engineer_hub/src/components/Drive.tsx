import { useEffect, useMemo, useReducer, useRef, useState } from "react";
import { activeCars, availabilityLabel, frozenSnapshot, isFeedStale, metric, text, useLive, value, type Snapshot } from "@/lib/live";
import { stats } from "@/lib/latency";
import { DriveGraph } from "@/components/DriveGraph";
import { DEFAULT_RANGE_MS, METRICS, appendSample, specOf, type Sample } from "@/lib/driveHistory";
import { completedLaps, useLiveLaps } from "@/lib/liveLaps";
import { wearColor } from "@/lib/wear";

const wheel = ["fl", "fr", "rl", "rr"] as const;
const SLOT_COLORS = ["#7fb0ff", "#f0a03c", "#5fd68f"];
const corners: Record<string, string> = { fl: "Front left", fr: "Front right", rl: "Rear left", rr: "Rear right" };
const recordingLabel: Record<string, string> = { recording: "Recording", stopped_user: "Stopped", stopped_limit: "Rating cap reached", stopped_error: "Recording error", idle: "Not recording" };

function stateLabel(availability?: string) {
  switch (availability) {
    case "stale": return "No update";
    case "restricted": return "Restricted";
    case "not_received": return "No data";
    case "unsupported": return "Unsupported";
    default: return "Unavailable";
  }
}

const withUnit = (v: string, unit: string) => v === "—" ? "—" : v + " " + unit;

export function WearRing({ pct }: { pct: number | null }) {
  const v = pct ?? 0, r = 30, c = 2 * Math.PI * r;
  return (
    <svg width="4.2em" height="4.2em" viewBox="0 0 72 72" role="img" aria-label={pct == null ? "wear unknown" : Math.round(v) + " percent wear"} className="shrink-0">
      <circle cx="36" cy="36" r={r} fill="none" stroke="#2a303a" strokeWidth="7" />
      <circle cx="36" cy="36" r={r} fill="none" stroke={wearColor(pct)} strokeWidth="7" strokeDasharray={String((Math.min(v, 100) / 100) * c) + " " + String(c)} transform="rotate(-90 36 36)" strokeLinecap="round" />
      <text x="36" y="41" textAnchor="middle" className="fill-white text-[1em] font-semibold tabular-nums">{pct == null ? "—" : Math.round(v) + "%"}</text>
    </svg>
  );
}

function tempState(v: number | null, season: number | null, availability?: string) {
  if (v == null) return { cls: "text-neutral-400", label: stateLabel(availability) };
  if (season === 2025) {
    if (v >= 109) return { cls: "text-[#f25a5c]", label: "Critical ↑" };
    if (v >= 104) return { cls: "text-[#f0a03c]", label: "Caution ↑" };
    if (v < 70) return { cls: "text-[#7fb0ff]", label: "Cold ↓" };
    return { cls: "text-[#5fd68f]", label: "Reference" };
  }
  return { cls: "text-neutral-100", label: "Unclassified" };
}

function pressureBar(m: ReturnType<typeof metric>): string {
  const v = value(m);
  return v == null ? "—" : (v / 100000).toFixed(2);
}

function Meter({ fraction, tone = "#5fd68f", label, detail }: { fraction: number | null; tone?: string; label: string; detail: string }) {
  const pct = fraction == null ? null : Math.max(0, Math.min(100, fraction * 100));
  return (
    <div>
      <div className="flex items-baseline justify-between text-[0.72em] text-neutral-400"><span>{label}</span><span className="tabular-nums text-neutral-300">{detail}</span></div>
      <div className="mt-1 h-[0.45em] w-full overflow-hidden rounded-sm bg-[#232830]" role="img" aria-label={label + ": " + detail}>
        {pct != null ? <div className="h-full rounded-sm" style={{ width: pct + "%", background: tone }} /> : null}
      </div>
    </div>
  );
}

function Tyre({ pos, s }: { pos: string; s: Snapshot }) {
  const wear = value(metric(s, "tyre." + pos + ".wear_pct"));
  const surfM = metric(s, "tyre." + pos + ".surface_c"), coreM = metric(s, "tyre." + pos + ".core_c");
  const surf = value(surfM), core = value(coreM);
  const season = s.context?.regulation_season ?? null;
  const surfState = tempState(surf, season, surfM?.availability), coreState = tempState(core, season, coreM?.availability);
  return (
    <section className="flex min-h-0 items-center gap-[0.9em] border-[#232830] px-1 py-[0.35em]">
      <WearRing pct={wear} />
      <div className="min-w-0 flex-1">
        <div className="text-[0.72em] font-semibold uppercase tracking-[0.14em] text-neutral-400">{corners[pos]}</div>
        <div className="grid grid-cols-2 gap-[0.9em]">
          <div>
            <div className="text-[0.66em] uppercase tracking-wider text-neutral-400">Surface</div>
            <div className={"text-[3.5em] font-semibold leading-none tabular-nums " + surfState.cls}>{surf == null ? "—" : Math.round(surf)}<span className="align-top text-[0.5em]">°</span></div>
            <div className="text-[0.68em] text-neutral-400">{surfState.label}</div>
          </div>
          <div>
            <div className="text-[0.66em] uppercase tracking-wider text-neutral-400">Core</div>
            <div className={"text-[3.5em] font-semibold leading-none tabular-nums " + coreState.cls}>{core == null ? "—" : Math.round(core)}<span className="align-top text-[0.5em]">°</span></div>
            <div className="text-[0.68em] text-neutral-400">{core == null ? stateLabel(coreM?.availability) : "Provisional"}</div>
          </div>
        </div>
        <div className="mt-[0.25em] flex gap-3 text-[0.7em] text-neutral-400">
          <span>{withUnit(pressureBar(metric(s, "tyre." + pos + ".pressure_pa")), "bar")}</span>
          <span>Brake {withUnit(text(metric(s, "brake." + pos + ".temp_c")), "°")}</span>
        </div>
      </div>
    </section>
  );
}

function delta(v: number | null): string {
  if (v == null) return "—";
  if (v <= 0.05) return "+0.0";
  return "+" + v.toFixed(1);
}

function driverName(s: Snapshot, car: number): string {
  const name = text(metric(s, "car.driver_name", car));
  return name && name !== "—" ? name : "Car " + (car + 1);
}

function driverCode(s: Snapshot, car: number): string {
  const last = driverName(s, car).trim().split(/\s+/).pop() ?? "";
  const code = last.replace(/[^A-Za-z0-9]/g, "").toUpperCase();
  return code ? code.slice(0, 3) : "C" + (car + 1);
}

// Team IDs follow the game's participant field; colours are the published
// liveries. Unknown or custom teams fall back to a neutral slot.
const teamColour: Record<number, string> = { 0: "#27F4D2", 1: "#E8002D", 2: "#3671C6", 3: "#64C4FF", 4: "#229971", 5: "#FF87BC", 6: "#6692FF", 7: "#B6BABD", 8: "#FF8000", 9: "#52E252" };
const teamCode: Record<number, string> = { 0: "MER", 1: "FER", 2: "RBR", 3: "WIL", 4: "AST", 5: "ALP", 6: "RB", 7: "HAA", 8: "MCL", 9: "SAU" };

function teamOf(s: Snapshot, car: number): { colour: string; code: string } {
  const id = value(metric(s, "car.team_id", car));
  const key = id == null ? -1 : Math.round(id);
  return { colour: teamColour[key] ?? "#76818f", code: teamCode[key] ?? (key >= 0 ? "T" + key : "") };
}

// Position-ordered lookup: the gap behind us is the car behind's own gap
// to the car in front.
function rivals(s: Snapshot) {
  const me = value(metric(s, "timing.car_position"));
  if (me == null) return { me: null, ahead: null, behind: null };
  let ahead: number | null = null, behind: number | null = null;
  for (const i of activeCars(s)) {
    const pos = value(metric(s, "timing.car_position", i));
    if (pos === me - 1) ahead = i;
    if (pos === me + 1) behind = i;
  }
  return { me, ahead, behind };
}

function FieldMap({ s }: { s: Snapshot }) {
  const length = value(metric(s, "session.track_length_m")) ?? 5000;
  // The map only needs one refresh a second. Repainting it at the stream rate
  // was the largest continuously invalidated area on the page.
  const cache = useRef<{ at: number; cars: { i: number; d: number; pos: number }[] }>({ at: 0, cars: [] });
  const cars: { i: number; d: number; pos: number }[] = [];
  for (const i of activeCars(s)) {
    const d = value(metric(s, "timing.lap_distance_m", i));
    const pos = value(metric(s, "timing.car_position", i));
    if (d != null && pos != null) cars.push({ i, d, pos });
  }
  if (Date.now() - cache.current.at >= 1000) cache.current = { at: Date.now(), cars };
  const shown = cache.current.cars.length ? cache.current.cars : cars;
  const player = s.context?.player_car_index ?? 0;
  // Keep near cars apart: three lanes, greedy so no two dots within 5% of the
  // lap share a lane.
  const lanes: number[][] = [[], [], []];
  const laneOf: Record<number, number> = {};
  for (const c of [...shown].sort((a, b) => a.d - b.d)) {
    let lane = lanes.findIndex((used) => used.every((d) => Math.abs(d - c.d) / length > 0.05));
    if (lane < 0) lane = c.pos % 3;
    lanes[lane].push(c.d);
    laneOf[c.i] = lane;
  }
  return (
    <section className="flex min-h-0 shrink-0 flex-col border border-[#262b33] bg-[#11141a] px-3 py-2" style={{ height: "13em" }}>
      <div className="flex items-baseline justify-between">
        <span className="text-[0.69em] uppercase tracking-wider text-neutral-400">Field map</span>
        <span className="text-[0.69em] tabular-nums text-neutral-400">{(length / 1000).toFixed(2)} km · {shown.length} positions</span>
      </div>
      <div className="relative mt-[0.7em] min-h-0 flex-1 rounded-sm border border-[#2a303a] bg-[#0d1015]">
        {[0.25, 0.5, 0.75].map((f) => <span key={f} className="absolute top-0 h-full w-px bg-[#1b2028]" style={{ left: f * 100 + "%" }} />)}
        {shown.map((c) => {
          const me = c.i === player;
          const team = teamOf(s, c.i);
          return (
            <span key={c.i} className="absolute -translate-x-1/2 -translate-y-1/2" title={driverName(s, c.i) + " · " + team.code + " · P" + c.pos} style={{ left: Math.max(2, Math.min(98, (c.d / length) * 100)) + "%", top: [24, 50, 76][laneOf[c.i] ?? 1] + "%", zIndex: me ? 4 : 3 }}>
              <span className="block rounded-full" style={{ width: me ? "0.9em" : "0.5em", height: me ? "0.9em" : "0.5em", background: team.colour, border: me ? "2px solid #f2f6fa" : "1px solid rgba(0,0,0,0.55)" }} />
              <span className="absolute bottom-full left-1/2 -translate-x-1/2 whitespace-nowrap text-[0.55em] font-semibold leading-tight tracking-wide text-neutral-200">{driverCode(s, c.i)}</span>
            </span>
          );
        })}
        {shown.length === 0 ? <span className="absolute inset-0 flex items-center justify-center text-[0.75em] text-neutral-400">Waiting for car positions</span> : null}
      </div>
      <div className="mt-1 flex justify-between text-[0.62em] text-neutral-500"><span>Start</span><span>Half distance</span><span>Line</span></div>
      <div className="mt-[0.3em] flex flex-wrap gap-x-2 gap-y-[0.2em] text-[0.58em] text-neutral-400">
        {[...new Set(shown.map((c) => teamOf(s, c.i).code).filter(Boolean))].map((code) => {
          const sample = shown.find((c) => teamOf(s, c.i).code === code) as { i: number };
          return <span key={code} className="flex items-center gap-1"><span className="inline-block h-[0.55em] w-[0.55em] rounded-full" style={{ background: teamOf(s, sample.i).colour }} />{code}</span>;
        })}
      </div>
    </section>
  );
}

const damageItems: { channel: string; label: string }[] = [
  { channel: "car.wing_front_left_damage_pct", label: "Front wing L" },
  { channel: "car.wing_front_right_damage_pct", label: "Front wing R" },
  { channel: "car.wing_rear_damage_pct", label: "Rear wing" },
  { channel: "car.floor_damage_pct", label: "Floor" },
  { channel: "car.diffuser_damage_pct", label: "Diffuser" },
  { channel: "car.sidepod_damage_pct", label: "Sidepod" },
  { channel: "car.gearbox_damage_pct", label: "Gearbox" },
  { channel: "car.engine_damage_pct", label: "Engine" },
  { channel: "car.drs_fault_pct", label: "DRS fault" },
  { channel: "car.ers_fault_pct", label: "ERS fault" },
];

function tone(v: number): string {
  return v >= 30 ? "#f25a5c" : v >= 10 ? "#f0a03c" : "#5fd68f";
}

function Issues({ s }: { s: Snapshot }) {
  const known = damageItems.some((d) => metric(s, d.channel)?.availability === "available");
  return (
    <section className="flex min-h-0 flex-1 flex-col border border-[#262b33] bg-[#11141a] px-3 py-2">
      <div className="flex items-baseline justify-between">
        <span className="text-[0.69em] uppercase tracking-wider text-neutral-400">Car issues</span>
        <span className="text-[0.69em] text-neutral-400">Pit stops <span className="tabular-nums text-neutral-200">{text(metric(s, "timing.pit_stops"))}</span> · Penalties <span className="tabular-nums text-neutral-200">{text(metric(s, "timing.penalties_s"))} s</span> · Warnings <span className="tabular-nums text-neutral-200">{text(metric(s, "timing.warnings"))}</span></span>
      </div>
      <div className="mt-[0.5em] grid min-h-0 flex-1 grid-cols-2 content-around gap-x-5 gap-y-[0.45em]">
        {damageItems.map((d) => {
          const m = metric(s, d.channel);
          const v = value(m);
          return (
            <div key={d.channel}>
              <div className="flex items-baseline justify-between text-[0.74em]">
                <span className="truncate text-neutral-300">{d.label}</span>
                <span className="tabular-nums" style={{ color: v == null ? "#a3a3a3" : tone(v) }}>{v == null ? stateLabel(m?.availability) : Math.round(v) + "%"}</span>
              </div>
              <div className="mt-[0.2em] h-[0.4em] w-full overflow-hidden rounded-sm bg-[#232830]">
                {v != null ? <div className="h-full rounded-sm" style={{ width: Math.min(100, v) + "%", background: tone(v) }} /> : null}
              </div>
            </div>
          );
        })}
      </div>
      <div className="mt-[0.4em] grid shrink-0 grid-cols-4 gap-x-3 border-t border-[#232830] pt-[0.4em]">
        {wheel.map((w) => (
          <div key={w} className="text-[0.7em]">
            <div className="uppercase tracking-wider text-neutral-500">{w}</div>
            <div className="flex justify-between gap-1 text-neutral-400"><span>Tyre</span><span className="tabular-nums">{withUnit(text(metric(s, "tyre." + w + ".damage_pct")), "%")}</span></div>
            <div className="flex justify-between gap-1 text-neutral-400"><span>Brake</span><span className="tabular-nums">{withUnit(text(metric(s, "brake." + w + ".damage_pct")), "%")}</span></div>
          </div>
        ))}
      </div>
      {!known ? <p className="mt-1 text-[0.72em] text-neutral-400">Damage data unavailable.</p> : null}
    </section>
  );
}

export function Drive() {
  const live = useLive();
  const p = live.context?.player_car_index ?? 0;
  const stale = isFeedStale(live, p);
  const pausedNow = text(metric(live, "session.paused")) === "true";
  const s = useMemo(() => (!live.connected || pausedNow || stale ? frozenSnapshot(live) : live), [live, pausedNow, stale]);
  const speed = value(metric(s, "car.speed_mps", p));
  const gear = value(metric(s, "car.gear", p));
  const lapMs = value(metric(s, "timing.current_lap_ms", p));
  const lastLap = value(metric(s, "timing.lap_ms", p));
  const energy = value(metric(s, "energy.store_j", p));
  const mode = text(metric(s, "energy.deploy_mode", p));
  const fuelLaps = value(metric(s, "fuel.finish_margin_laps", p));
  const fuelKg = value(metric(s, "fuel.mass_kg", p));
  const fuelCapacity = value(metric(s, "fuel.capacity_kg", p));
  const totalLaps = value(metric(s, "session.total_laps"));
  const currentLap = value(metric(s, "timing.current_lap_num", p));
  const pos = text(metric(s, "timing.car_position", p)), lapNo = text(metric(s, "timing.current_lap_num", p));
  const age = text(metric(s, "tyre.age_laps", p));
  const gap = metric(s, "timing.gap_ahead_s", p);
  const leaderGap = value(metric(s, "timing.gap_leader_s", p));
  const safety = text(metric(s, "session.safety_car_status"));
  const weather = text(metric(s, "session.weather"));
  const paused = pausedNow;
  const call = s.events.filter((e) => e.severity !== "routine").slice(-1)[0]?.summary ?? s.alerts.find((a) => a.state === "active")?.summary ?? (s.context ? "No current call" : "Waiting for telemetry");
  const rival = rivals(s);
  const gapBehind = rival.behind != null ? value(metric(s, "timing.gap_ahead_s", rival.behind)) : null;
  // A few representative channels cover the feed; scanning all ~1000 metrics on
  // every 60Hz render was pure garbage.
  const remainingLaps = totalLaps != null && currentLap != null ? Math.max(0, totalLaps - currentLap + 1) : null;

  const [slots, setSlots] = useState([
    { metric: "lap_time", range: DEFAULT_RANGE_MS },
    { metric: "tyre_wear", range: DEFAULT_RANGE_MS },
    { metric: "gap_ahead", range: DEFAULT_RANGE_MS },
  ]);
  const [lagMs, setLagMs] = useState<number | null>(null);
  useEffect(() => {
    const id = setInterval(() => {
      const l = stats();
      setLagMs(l.n > 0 ? Math.round(l.p95) : null);
    }, 2000);
    return () => clearInterval(id);
  }, []);
  const history = useRef<Record<string, Sample[]>>({});
  const peakRef = useRef(0);
  const seenLaps = useRef<Set<number>>(new Set());
  const [, bumpHistory] = useReducer((n: number) => n + 1, 0);
  const snapshotRef = useRef(s);
  snapshotRef.current = s;
  const feedRef = useRef({ connected: live.connected, stale });
  feedRef.current = { connected: live.connected, stale };

  // A flashback keeps the session id but starts a new branch, so branch is part
  // of the reset key too. Otherwise re-run laps are skipped as already seen.
  useEffect(() => {
    history.current = {};
    peakRef.current = 0;
    seenLaps.current.clear();
    bumpHistory();
  }, [live.context?.session_id, live.context?.branch_id]);

  const { laps: liveLaps, sessionId: lapSession } = useLiveLaps(
    live.context?.session_id ?? null,
    live.context?.branch_id ?? null,
    !!live.context && live.connected && !stale,
  );

  // Completed laps enter the lap-time graph once, at the poll that saw them.
  useEffect(() => {
    if (!lapSession || lapSession !== live.context?.session_id) return;
    const now = Date.now();
    for (const lap of completedLaps(liveLaps, p)) {
      const ms = lap.lap_time_ms?.value;
      if (typeof ms !== "number" || seenLaps.current.has(lap.lap)) continue;
      seenLaps.current.add(lap.lap);
      appendSample((history.current["lap_time"] ??= []), now, ms / 1000);
    }
    bumpHistory();
  }, [liveLaps, lapSession, live.context?.session_id, p]);

  // One tick samples every metric, whether or not a slot shows it.
  useEffect(() => {
    const tick = () => {
      const feed = feedRef.current;
      if (!feed.connected || feed.stale) return;
      const snap = snapshotRef.current;
      const now = Date.now();
      for (const spec of METRICS) {
        if (!spec.read) continue;
        const series = (history.current[spec.id] ??= []);
        appendSample(series, now, spec.read(snap, p));
      }
      const e = value(metric(snap, "energy.store_j", p));
      if (e != null && e > peakRef.current) peakRef.current = e;
      bumpHistory();
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [p]);

  const ersPeak = peakRef.current > 0 ? peakRef.current : energy;

  return (
    <div style={{ fontSize: "clamp(16px, 1.48vh, 22px)" }} className="flex min-h-[calc(100vh-49px)] flex-col gap-2 bg-[#0b0d11] px-4 pb-2 pt-2 text-neutral-100 [@media(min-width:1024px)]:h-[calc(100vh-49px)] [@media(min-width:1024px)]:overflow-hidden">
      <header className="flex shrink-0 items-center justify-between border border-[#262b33] bg-[#11141a] px-4 py-1.5">
        <div className="flex items-baseline gap-4"><span className="text-[1.15em] font-bold tracking-wide">{s.context?.track_id != null ? "Track " + s.context.track_id : "Track unknown"}</span><span className="text-[0.85em] text-neutral-400">{String(s.context?.regulation_season ?? "season unknown")} · {s.context?.mode_family ?? "unknown"}</span></div>
        <div className="flex gap-6 text-[0.85em]"><span className="text-neutral-200">{pos === "—" ? "Pos —" : "P" + pos}</span><span className="text-neutral-400">{lapNo === "—" ? "Lap —" : "Lap " + lapNo + (totalLaps != null ? "/" + Math.round(totalLaps) : "")}</span><span className={s.connected && !stale && !paused ? "text-[#5fd68f]" : "text-neutral-400"}>{s.connected ? (paused ? "Paused" : stale ? "Stale · frozen" : lagMs == null ? "Live" : "Live · " + lagMs + " ms") : "Disconnected"}</span><span className="text-neutral-400">{recordingLabel[s.recording] ?? "Recording state unknown"}</span></div>
      </header>

      <section className="flex h-[4.6em] shrink-0 items-center border border-[#262b33] bg-[#10201a] px-5">
        <div className="min-w-0"><div className="text-[0.69em] uppercase tracking-[0.18em] text-[#5fd68f]">Race engineer · current call</div><div className="truncate text-[1.6em] font-semibold leading-tight">{call}</div></div>
        <div className="ml-auto shrink-0 text-right text-[0.76em] text-neutral-400">
          <div>{weather === "—" ? "Weather —" : weather === "0" ? "Clear" : "Weather code " + weather} · {safety === "—" ? "SC —" : safety === "0" ? "SC clear" : "SC deployed"}</div>
          <div>Air {text(metric(s, "session.air_temp_c"))}° · Track {text(metric(s, "session.track_temp_c"))}°</div>
        </div>
      </section>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-2 [@media(min-width:1024px)]:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)]">
        <div className="flex min-h-0 flex-col gap-2">
          <section className="flex shrink-0 flex-col border border-[#262b33] bg-[#11141a] px-3 py-1.5">
            <div className="flex items-center justify-between text-[0.78em] text-neutral-400">
              <span>{availabilityLabel(metric(s, "tyre.compound", p)) === "Live" ? "Tyre " + text(metric(s, "tyre.compound", p)) + (age === "—" ? "" : " · " + age + (age === "1" ? " lap" : " laps")) : "Tyres"}</span>
              <span>°C · wear % · pressure bar</span>
            </div>
            <div className="grid grid-cols-2">
              {wheel.map((w) => <Tyre key={w} pos={w} s={s} />)}
            </div>
          </section>
          <Issues s={s} />
          <FieldMap s={s} />
        </div>

        <div className="flex min-h-0 flex-col gap-2">
          <div className="grid shrink-0 grid-cols-[auto_minmax(0,1fr)_minmax(0,1fr)] items-center gap-4 border border-[#262b33] bg-[#11141a] px-4 py-2">
            <div><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Position</div><div className="text-[2.3em] font-semibold leading-none tabular-nums">{pos === "—" ? "—" : "P" + pos}</div></div>
            <div className="min-w-0 border-l border-[#232830] pl-4">
              <div className="truncate text-[0.69em] uppercase tracking-wider text-neutral-400">Ahead {rival.ahead != null ? "· " + driverName(s, rival.ahead) : ""}</div>
              <div className="text-[2.5em] font-semibold leading-none tabular-nums text-[#f0a03c]">{delta(value(gap))}<span className="ml-1 text-[0.4em] font-normal text-neutral-400">s</span></div>
              <div className="text-[0.65em] text-neutral-500">Leader {leaderGap == null ? "—" : "+" + leaderGap.toFixed(1) + " s"}</div>
            </div>
            <div className="min-w-0 border-l border-[#232830] pl-4">
              <div className="truncate text-[0.69em] uppercase tracking-wider text-neutral-400">Behind {rival.behind != null ? "· " + driverName(s, rival.behind) : ""}</div>
              <div className="text-[2.5em] font-semibold leading-none tabular-nums text-[#7fb0ff]">{delta(gapBehind)}<span className="ml-1 text-[0.4em] font-normal text-neutral-400">s</span></div>
              <div className="text-[0.65em] text-neutral-500">{gapBehind == null ? stateLabel(metric(s, "timing.gap_ahead_s", rival.behind ?? p)?.availability) : "to the car behind"}</div>
            </div>
          </div>

          <div className="grid shrink-0 grid-cols-2 gap-2">
            <div className="border border-[#262b33] bg-[#11141a] px-4 py-2">
              <div className="flex items-baseline justify-between"><span className="text-[0.69em] uppercase tracking-wider text-neutral-400">Fuel margin</span><span className="text-[0.7em] text-neutral-400">{fuelKg == null ? "" : fuelKg.toFixed(1) + " kg"}</span></div>
              <div className="text-[2.4em] font-semibold leading-none tabular-nums">{fuelLaps == null ? "—" : fuelLaps.toFixed(1)}<span className="ml-1 text-[0.4em] font-normal text-neutral-400">laps</span></div>
              <div className="mt-[0.45em] space-y-[0.35em]">
                <Meter fraction={fuelKg != null && fuelCapacity != null && fuelCapacity > 0 ? fuelKg / fuelCapacity : null} tone="#5fd68f" label="Tank" detail={fuelKg != null && fuelCapacity != null && fuelCapacity > 0 ? fuelKg.toFixed(1) + " / " + fuelCapacity.toFixed(1) + " kg" : "Capacity unavailable"} />
                <Meter fraction={fuelLaps != null && remainingLaps != null && remainingLaps > 0 ? fuelLaps / remainingLaps : null} tone={fuelLaps != null && remainingLaps != null && fuelLaps < remainingLaps ? "#f0a03c" : "#5fd68f"} label="Margin vs remaining" detail={remainingLaps != null ? fuelLaps == null ? remainingLaps + " laps left" : fuelLaps.toFixed(1) + " / " + remainingLaps + " laps" : "Lap count unavailable"} />
              </div>
            </div>
            <div className="border border-[#262b33] bg-[#11141a] px-4 py-2">
              <div className="flex items-baseline justify-between"><span className="text-[0.69em] uppercase tracking-wider text-neutral-400">ERS</span><span className="text-[0.7em] text-neutral-400">{mode === "—" ? stateLabel(metric(s, "energy.deploy_mode", p)?.availability) : mode}</span></div>
              <div className="text-[2.4em] font-semibold leading-none tabular-nums text-[#f0a03c]">{energy == null ? "—" : (energy / 1e6).toFixed(2)}<span className="ml-1 text-[0.4em] font-normal text-neutral-400">MJ</span></div>
              <div className="mt-[0.45em] space-y-[0.35em]">
                <Meter fraction={energy != null && ersPeak ? energy / ersPeak : null} tone="#f0a03c" label="Store vs session peak" detail={energy == null ? "Store unavailable" : (energy / 1e6).toFixed(2) + " / " + (ersPeak! / 1e6).toFixed(2) + " MJ"} />
                <div className="flex items-baseline justify-between text-[0.72em] text-neutral-400">
                  <span>Harvest limit</span>
                  <span className="tabular-nums text-neutral-300">{withUnit(value(metric(s, "energy.harvest_limit_j_per_lap", p)) == null ? "—" : ((value(metric(s, "energy.harvest_limit_j_per_lap", p)) as number) / 1e6).toFixed(2), "MJ/lap")}</span>
                </div>
              </div>
            </div>
          </div>

          <div className="grid shrink-0 grid-cols-3 gap-2 border border-[#262b33] bg-[#11141a] px-4 py-2">
            <div>
              <div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Speed</div>
              <div className="text-[2.1em] font-semibold leading-none tabular-nums">{speed == null ? "—" : Math.round(speed * 3.6)}<span className="text-[0.4em] font-normal"> km/h</span></div>
              <div className="text-[0.78em] text-neutral-400">Gear {gear == null ? "—" : Math.round(gear)} · {text(metric(s, "car.drs", p)) === "true" ? "DRS open" : "DRS closed"}</div>
            </div>
            <div className="border-l border-[#232830] pl-3">
              <div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Lap time</div>
              <div className="text-[1.9em] font-semibold leading-none tabular-nums">{lapMs == null ? "—" : (lapMs / 1000).toFixed(2)}<span className="text-[0.4em] font-normal"> s</span></div>
              <div className="text-[0.78em] text-neutral-400">Last {lastLap == null ? "—" : (lastLap / 1000).toFixed(2)} s</div>
            </div>
            <div className="border-l border-[#232830] pl-3">
              <div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Engine</div>
              <div className="text-[1.9em] font-semibold leading-none tabular-nums">{text(metric(s, "car.engine_temp_c", p))}<span className="text-[0.4em] font-normal"> °C</span></div>
              <div className="text-[0.78em] text-neutral-400">RPM {text(metric(s, "car.rpm", p))}</div>
            </div>
          </div>

          <section className="flex min-h-0 flex-1 flex-col gap-1 border border-[#262b33] bg-[#11141a] px-3 py-2">
            {slots.map((slot, i) => {
              const spec = specOf(slot.metric);
              return (
                <DriveGraph key={i} index={i} spec={spec} series={history.current[spec.id] ?? []}
                  rangeMs={slot.range} color={SLOT_COLORS[i] ?? SLOT_COLORS[0]}
                  onMetric={(id) => setSlots((prev) => prev.map((x, j) => j === i ? { ...x, metric: id } : x))}
                  onRange={(ms) => setSlots((prev) => prev.map((x, j) => j === i ? { ...x, range: ms } : x))} />
              );
            })}
          </section>
        </div>
      </div>
    </div>
  );
}
