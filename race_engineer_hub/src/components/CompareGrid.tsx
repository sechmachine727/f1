import { metric, text, value, type Snapshot } from "@/lib/live";
import { teamInk } from "@/lib/teams";
import type { SamplePoint } from "@/lib/samples";

type Spec = {
  label: string;
  channel: string;
  unit: string;
  better?: "high" | "low";
  format: (v: number) => string;
};

const CORNERS = ["fl", "fr", "rl", "rr"] as const;
const DAMAGE = [
  "car.wing_front_left_damage_pct", "car.wing_front_right_damage_pct", "car.wing_rear_damage_pct",
  "car.floor_damage_pct", "car.diffuser_damage_pct", "car.sidepod_damage_pct",
  "car.gearbox_damage_pct", "car.engine_damage_pct",
];

export const COMPARE_ROWS: Spec[] = [
  { label: "Position", channel: "timing.car_position", unit: "", better: "low", format: (v) => "P" + Math.round(v) },
  { label: "Gap ahead", channel: "timing.gap_ahead_s", unit: " s", better: "low", format: (v) => v.toFixed(3) },
  { label: "Gap to leader", channel: "timing.gap_leader_s", unit: " s", better: "low", format: (v) => v.toFixed(3) },
  { label: "Speed", channel: "car.speed_mps", unit: " km/h", better: "high", format: (v) => String(Math.round(v * 3.6)) },
  { label: "Throttle", channel: "car.throttle_fraction", unit: "", format: (v) => Math.round(v * 100) + "%" },
  { label: "Brake", channel: "car.brake_fraction", unit: "", format: (v) => Math.round(v * 100) + "%" },
  { label: "Gear", channel: "car.gear", unit: "", format: (v) => String(Math.round(v)) },
  { label: "ERS store", channel: "energy.store_j", unit: " MJ", better: "high", format: (v) => (v / 1000000).toFixed(2) },
  { label: "Fuel", channel: "fuel.mass_kg", unit: " kg", better: "high", format: (v) => v.toFixed(1) },
  { label: "Fuel margin", channel: "fuel.finish_margin_laps", unit: " laps", better: "high", format: (v) => v.toFixed(1) },
  { label: "Worst wear", channel: "tyre.fl.wear_pct", unit: "%", better: "low", format: (v) => v.toFixed(1) },
];

function worstWear(s: Snapshot, car: number): number | null {
  const values = CORNERS.map((c) => value(metric(s, "tyre." + c + ".wear_pct", car))).filter((v): v is number => v != null);
  return values.length ? Math.max(...values) : null;
}

function maxDamage(s: Snapshot, car: number): number | null {
  const values = DAMAGE.map((c) => value(metric(s, c, car))).filter((v): v is number => v != null);
  return values.length ? Math.max(...values) : null;
}

function deltaText(spec: Spec, a: number, b: number, nameA: string, nameB: string): string {
  const diff = Math.abs(a - b);
  if (diff === 0) return "level";
  if (spec.better === "high") {
    const winner = a > b ? nameA : nameB;
    return winner + " higher by " + spec.format(diff) + (spec.unit || "");
  }
  if (spec.better === "low") {
    const winner = a < b ? nameA : nameB;
    return winner + " lower by " + spec.format(diff) + (spec.unit || "");
  }
  return spec.format(diff) + (spec.unit || "") + " apart";
}

function tone(spec: Spec, v: number | null, car: number, s: Snapshot): string {
  if (v == null) return "text-neutral-400";
  if (spec.channel.includes("wear_pct") || spec.label === "Worst wear") {
    const wear = spec.label === "Worst wear" ? worstWear(s, car) : v;
    if (wear == null) return "text-neutral-400";
    return wear >= 90 ? "text-[#f25a5c]" : wear >= 75 ? "text-[#f0a03c]" : "text-[#5fd68f]";
  }
  return "text-neutral-100";
}

function CarHeader({ s, car }: { s: Snapshot; car: number }) {
  const name = text(metric(s, "car.driver_name", car));
  const team = value(metric(s, "car.team_id", car));
  const compound = text(metric(s, "tyre.compound", car));
  const color = teamInk(team);
  return (
    <div className="flex items-center gap-2 border-l-2 pl-2" style={{ borderColor: color }}>
      <span className="text-sm font-semibold" style={{ color }}>{name === "—" ? "Car " + (car + 1) : name}</span>
      <span className="text-[11px] text-neutral-400">{compound === "—" ? "compound unknown" : compound}</span>
      <span className="ml-auto text-[11px] text-neutral-400">P{text(metric(s, "timing.car_position", car))}</span>
    </div>
  );
}

/** Two-car live comparison. Values come from the live snapshot; a missing or
 * restricted value suppresses its delta instead of showing a false zero. */
export function CompareGrid({ s, cars, live }: { s: Snapshot; cars: number[]; live?: Map<number, SamplePoint[]> }) {
  if (cars.length === 0) return <p className="py-6 text-sm text-neutral-400">Select a car to inspect its live telemetry.</p>;
  // Throttle and brake stay out of the 24-car subscription; the selected-car
  // buffer carries the newest sample for the two cars on screen.
  const read = (channel: string, car: number): number | null => {
    const snapshot = value(metric(s, channel, car));
    if (snapshot != null) return snapshot;
    const last = live?.get(car)?.at(-1);
    if (!last) return null;
    if (channel === "car.throttle_fraction") return last[1];
    if (channel === "car.brake_fraction") return last[2];
    return null;
  };
  const names = cars.map((c) => {
    const n = text(metric(s, "car.driver_name", c));
    return n === "—" ? "Car " + (c + 1) : n;
  });
  return (
    <div>
      <div className="grid gap-2" style={{ gridTemplateColumns: "repeat(" + cars.length + ", minmax(0, 1fr))" }}>
        {cars.map((car) => <CarHeader key={car} s={s} car={car} />)}
      </div>
      <table className="mt-2 w-full text-sm">
        <tbody>
          {COMPARE_ROWS.map((spec) => {
            const values = cars.map((car) => {
              if (spec.label === "Worst wear") return worstWear(s, car);
              return read(spec.channel, car);
            });
            const both = values.length === 2 && values[0] != null && values[1] != null;
            return (
              <tr key={spec.label} className="border-t border-[#232830]">
                <th scope="row" className="py-1 text-left text-[11px] font-normal uppercase tracking-wider text-neutral-400">{spec.label}</th>
                {cars.map((car, i) => (
                  <td key={car} className={"py-1 text-right tabular-nums " + tone(spec, values[i], car, s)}>
                    {values[i] == null ? "—" : spec.format(values[i] as number)}
                  </td>
                ))}
                {cars.length === 2 ? (
                  <td className="py-1 pl-3 text-right text-[11px] text-neutral-400">
                    {both ? deltaText(spec, values[0] as number, values[1] as number, names[0], names[1]) : "needs both cars"}
                  </td>
                ) : null}
              </tr>
            );
          })}
          <tr className="border-t border-[#232830]">
            <th scope="row" className="py-1 text-left text-[11px] font-normal uppercase tracking-wider text-neutral-400">Deployment</th>
            {cars.map((car) => (
              <td key={car} className="py-1 text-right text-neutral-100">{text(metric(s, "energy.deploy_mode", car))}</td>
            ))}
            {cars.length === 2 ? <td className="py-1 pl-3 text-right text-[11px] text-neutral-400">state only</td> : null}
          </tr>
          <tr className="border-t border-[#232830]">
            <th scope="row" className="py-1 text-left text-[11px] font-normal uppercase tracking-wider text-neutral-400">Damage</th>
            {cars.map((car) => {
              const d = maxDamage(s, car);
              return <td key={car} className={"py-1 text-right tabular-nums " + (d != null && d >= 50 ? "text-[#f25a5c]" : "text-neutral-100")}>{d == null ? "—" : d.toFixed(0) + "%"}</td>;
            })}
            {cars.length === 2 ? <td className="py-1 pl-3 text-right text-[11px] text-neutral-400">highest panel</td> : null}
          </tr>
        </tbody>
      </table>

      <div className="mt-2 grid gap-2" style={{ gridTemplateColumns: "repeat(" + cars.length + ", minmax(0, 1fr))" }}>
        {cars.map((car) => (
          <div key={car}>
            <div className="text-[10px] uppercase tracking-wider text-neutral-400">Tyres · wear / surface</div>
            <div className="grid grid-cols-4 gap-1 text-[11px]">
              {CORNERS.map((c) => {
                const wear = value(metric(s, "tyre." + c + ".wear_pct", car));
                const tempM = metric(s, "tyre." + c + ".surface_c", car);
                const temp = value(tempM);
                const label = c.toUpperCase();
                return (
                  <div key={c} className="tabular-nums">
                    <div className="text-neutral-400">{label}</div>
                    <div className={wear == null ? "text-neutral-400" : wear >= 90 ? "text-[#f25a5c]" : wear >= 75 ? "text-[#f0a03c]" : "text-[#5fd68f]"}>
                      {wear == null ? "—" : wear.toFixed(1)}
                    </div>
                    <div className="text-neutral-300">{temp == null ? "—" : Math.round(temp) + "°"}</div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

