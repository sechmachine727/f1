import { availabilityLabel, metric, text, useLive, value, type Snapshot } from "@/lib/live";
const wheel = ["fl", "fr", "rl", "rr"] as const;
const corners: Record<string, string> = { fl: "Front left", fr: "Front right", rl: "Rear left", rr: "Rear right" };
function WearRing({ pct }: { pct: number | null }) {
  const v = pct ?? 0, r = 30, c = 2 * Math.PI * r;
  return (
    <svg width="4.5em" height="4.5em" viewBox="0 0 72 72" role="img" aria-label={pct == null ? "wear unknown" : Math.round(v) + " percent wear"}>
      <circle cx="36" cy="36" r={r} fill="none" stroke="#2a303a" strokeWidth="7" />
      <circle cx="36" cy="36" r={r} fill="none" stroke={v >= 90 ? "#f2484b" : v >= 75 ? "#f0a03c" : "#4fd48a"} strokeWidth="7" strokeDasharray={String((Math.min(v, 100) / 100) * c) + " " + String(c)} transform="rotate(-90 36 36)" strokeLinecap="round" />
      <text x="36" y="41" textAnchor="middle" className="fill-white text-[1em] font-semibold tabular-nums">{pct == null ? "—" : Math.round(v) + "%"}</text>
    </svg>
  );
}
function tempState(v: number | null, season: number | null) {
  if (v == null) return { cls: "text-neutral-400", label: "Unavailable" };
  if (season === 2025) {
    if (v >= 109) return { cls: "text-[#f25a5c]", label: "Critical ↑" };
    if (v >= 104) return { cls: "text-[#f0a03c]", label: "Caution ↑" };
    if (v < 70) return { cls: "text-[#7fb0ff]", label: "Cold ↓" };
    return { cls: "text-[#5fd68f]", label: "Reference" };
  }
  return { cls: "text-neutral-100", label: "Unclassified" };
}
function Tyre({ pos, s }: { pos: string; s: Snapshot }) {
  const wear = value(metric(s, "tyre." + pos + ".wear_pct")), surf = value(metric(s, "tyre." + pos + ".surface_c")), core = value(metric(s, "tyre." + pos + ".core_c"));
  const season = s.context?.regulation_season ?? null;
  const surfState = tempState(surf, season), coreState = tempState(core, season);
  return (
    <div className="py-1 flex flex-col justify-center">
      <div className="text-[0.75em] font-semibold tracking-[0.14em] text-neutral-400 uppercase">{corners[pos]}</div>
      <div className="flex items-center gap-4 mt-1">
        <WearRing pct={wear} />
        <div className="grid grid-cols-2 gap-4">
          <div><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Surface</div><div className={"text-[3.5em] leading-none font-semibold tabular-nums " + surfState.cls}>{surf == null ? "—" : Math.round(surf)}<span className="text-2xl align-top">°</span></div><div className="text-[0.75em] text-neutral-400">{surfState.label}</div></div>
          <div><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Core</div><div className={"text-[3.5em] leading-none font-semibold tabular-nums " + coreState.cls}>{core == null ? "—" : Math.round(core)}<span className="text-2xl align-top">°</span></div><div className="text-[0.75em] text-neutral-400">{core == null ? "Unknown" : "Provisional"}</div></div>
        </div>
      </div>
    </div>
  );
}
function pressureBar(m: ReturnType<typeof metric>): string {
  const v = value(m);
  if (v == null) return "—";
  return (v / 100000).toFixed(2);
}

export function Drive() {
  const s = useLive();
  const p = s.context?.player_car_index ?? 0;
  const lap = value(metric(s, "timing.current_lap_ms", p)), gear = value(metric(s, "car.gear", p)), speed = value(metric(s, "car.speed_mps", p));
  const energy = value(metric(s, "energy.store_j", p)), mode = text(metric(s, "energy.deploy_mode", p));
  const pos = text(metric(s, "timing.car_position", p)), lapNo = text(metric(s, "timing.current_lap_num", p));
  const safety = text(metric(s, "session.safety_car_status", p));
  const weather = text(metric(s, "session.weather", p));
  const call = s.events.filter((e) => e.severity !== "routine").slice(-1)[0]?.summary ?? s.alerts.find((a) => a.state === "active")?.summary ?? (s.context ? "No current call" : "Waiting for telemetry");
  return (
    <div style={{ fontSize: "clamp(16px, 1.48vh, 22px)" }} className="min-h-[calc(100vh-49px)] bg-[#0b0d11] text-neutral-100 px-4 pt-2 pb-2 flex flex-col gap-2 [@media(min-width:1024px)]:h-[calc(100vh-49px)] [@media(min-width:1024px)]:overflow-hidden">
      <header className="flex items-center justify-between border border-[#262b33] bg-[#11141a] px-4 py-2 shrink-0">
        <div className="flex items-baseline gap-4"><span className="text-[1.19em] font-bold tracking-wide">{s.context?.track_id != null ? "Track " + s.context.track_id : "Track unknown"}</span><span className="text-[0.88em] text-neutral-400">{String(s.context?.regulation_season ?? "season unknown")} · {s.context?.mode_family ?? "unknown"}</span></div>
        <div className="flex gap-6 text-[0.88em]"><span className="text-neutral-200">{pos === "—" ? "Pos —" : "P" + pos}</span><span className="text-neutral-400">{lapNo === "—" ? "Lap —" : "Lap " + lapNo}</span><span className={s.connected ? "text-[#5fd68f]" : "text-neutral-400"}>{s.connected ? "Live" : "Disconnected"}</span><span className="text-neutral-400">{s.recording === "recording" ? "Recording" : s.recording.replace("_", " ")}</span></div>
      </header>
      <section className="border border-[#262b33] bg-[#10201a] px-5 py-3 h-[88px] shrink-0 flex items-center">
        <div className="min-w-0"><div className="text-[0.69em] uppercase tracking-[0.18em] text-[#5fd68f]">Race engineer · current call</div><div className="text-[1.69em] leading-tight font-semibold truncate">{call}</div></div>
      </section>
      <div className="grid grid-cols-1 [@media(min-width:1024px)]:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)] gap-2 min-h-0 flex-1">
        <section className="border border-[#262b33] bg-[#11141a] px-5 flex flex-col min-h-0">
          <div className="flex justify-between items-center pt-2 text-[0.81em] text-neutral-400"><span>{availabilityLabel(metric(s, "tyre.compound", p)) === "Live" ? "Tyre " + text(metric(s, "tyre.compound", p)) + " · " + text(metric(s, "tyre.age_laps", p)) + " laps" : "Tyres"}</span><span>°C · wear %</span></div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-4 py-2 flex-1 content-center">{wheel.map((w) => <Tyre key={w} pos={w} s={s} />)}</div>
        </section>
        <section className="flex flex-col gap-2 min-h-0 min-w-0">
          <div className="border border-[#262b33] bg-[#11141a] px-4 py-3"><div className="flex justify-between text-[0.81em] text-neutral-400"><span>ERS mode</span><span>Energy</span></div><div className="flex justify-between items-baseline gap-3 min-w-0"><span className="text-[1.625em] font-semibold truncate">{mode}</span><span className="text-[2.125em] font-semibold tabular-nums text-[#f0a03c] whitespace-nowrap">{energy == null ? "—" : (energy / 1e6).toFixed(1)}<span className="text-[0.6em] font-normal"> MJ</span></span></div></div>
          <div className="border border-[#262b33] bg-[#11141a] px-4 py-3 flex-1 flex flex-col min-w-0">
            <div className="grid grid-cols-2 gap-4"><div><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Speed</div><div className="text-[2.125em] font-semibold tabular-nums">{speed == null ? "—" : Math.round(speed * 3.6)}<span className="text-[0.4em] font-normal"> km/h</span></div><div className="text-[0.75em] text-neutral-400">Gear {gear == null ? "—" : Math.round(gear)}</div></div><div><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Lap time</div><div className="text-[1.625em] font-semibold tabular-nums">{lap == null ? "—" : (lap / 1000).toFixed(2)}<span className="text-[0.4em] font-normal"> s</span></div></div></div>
            <div className="mt-auto pt-3 border-t border-[#262b33]"><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Brake temp °C</div><div className="grid grid-cols-4 gap-2 mt-1">{wheel.map((w) => <div key={w} className="tabular-nums"><span className="text-[0.69em] uppercase text-neutral-400 mr-1">{w}</span><span className="text-[1.05em] font-semibold">{text(metric(s, "brake." + w + ".temp_c", p))}</span></div>)}</div></div>
            <div className="pt-2"><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Pressure bar</div><div className="grid grid-cols-4 gap-2 mt-1">{wheel.map((w) => <div key={w} className="tabular-nums"><span className="text-[0.69em] uppercase text-neutral-400 mr-1">{w}</span><span className="text-[1.05em] font-semibold">{pressureBar(metric(s, "tyre." + w + ".pressure_pa", p))}</span></div>)}</div></div>
          </div>
        </section>
      </div>
      <section className="grid grid-cols-3 gap-2 shrink-0 border-t border-[#262b33] pt-2">
        <div className="px-1"><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Ahead / behind</div><div className="text-[1.5em] font-semibold tabular-nums text-neutral-100">— <span className="text-[0.5em] font-normal text-neutral-400">restricted</span></div></div>
        <div className="px-1"><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Fuel margin</div><div className="text-[1.5em] font-semibold tabular-nums text-neutral-100">{text(metric(s, "fuel.finish_margin_laps", p))} <span className="text-[0.5em] font-normal">laps</span></div></div>
        <div className="px-1"><div className="text-[0.69em] uppercase tracking-wider text-neutral-400">Conditions</div><div className="text-[0.95em] text-neutral-200">{weather === "—" ? "Weather —" : weather === "0" ? "Clear" : "Weather code " + weather} · {safety === "—" ? "SC —" : safety === "0" ? "SC clear" : "SC deployed"} · Air {text(metric(s, "session.air_temp_c", p))}° / track {text(metric(s, "session.track_temp_c", p))}°</div></div>
      </section>
      <footer className="flex justify-end text-[0.69em] text-neutral-400 shrink-0"><span>{s.context?.session_id ?? "Session not started"}</span></footer>
    </div>
  );
}
