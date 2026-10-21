import { metric, value, type Snapshot } from "@/lib/live";

export type Sample = { t: number; v: number | null };

export type MetricSpec = {
  id: string;
  label: string;
  unit: string;
  digits: number;
  read?: (s: Snapshot, player: number) => number | null;
};

/** Longest retained history. Range selection only moves the view. */
export const RETENTION_MS = 10 * 60 * 1000;
export const DEFAULT_RANGE_MS = 2 * 60 * 1000;
export const RANGE_OPTIONS = [30_000, 60_000, 120_000, 300_000, 600_000];

const maxOf = (s: Snapshot, player: number, channels: string[]) => {
  const values = channels.map((c) => value(metric(s, c, player))).filter((v): v is number => v != null);
  return values.length ? Math.max(...values) : null;
};

export const METRICS: MetricSpec[] = [
  { id: "lap_time", label: "Lap time", unit: "s", digits: 3 },
  { id: "tyre_wear", label: "Tyre wear", unit: "%", digits: 1,
    read: (s, p) => maxOf(s, p, ["fl", "fr", "rl", "rr"].map((w) => "tyre." + w + ".wear_pct")) },
  { id: "gap_ahead", label: "Gap ahead", unit: "s", digits: 3, read: (s, p) => value(metric(s, "timing.gap_ahead_s", p)) },
  { id: "gap_leader", label: "Gap to leader", unit: "s", digits: 3, read: (s, p) => value(metric(s, "timing.gap_leader_s", p)) },
  { id: "ers", label: "ERS store", unit: "MJ", digits: 2, read: (s, p) => {
    const j = value(metric(s, "energy.store_j", p));
    return j == null ? null : j / 1_000_000;
  } },
  { id: "fuel", label: "Fuel", unit: "kg", digits: 1, read: (s, p) => value(metric(s, "fuel.mass_kg", p)) },
  { id: "engine", label: "Engine temp", unit: "°C", digits: 0, read: (s, p) => value(metric(s, "car.engine_temp_c", p)) },
  { id: "surface", label: "Tyre surface", unit: "°C", digits: 0,
    read: (s, p) => maxOf(s, p, ["fl", "fr", "rl", "rr"].map((w) => "tyre." + w + ".surface_c")) },
  { id: "core", label: "Tyre core", unit: "°C", digits: 0,
    read: (s, p) => maxOf(s, p, ["fl", "fr", "rl", "rr"].map((w) => "tyre." + w + ".core_c")) },
  { id: "speed", label: "Speed", unit: "km/h", digits: 0, read: (s, p) => {
    const m = value(metric(s, "car.speed_mps", p));
    return m == null ? null : m * 3.6;
  } },
  { id: "throttle", label: "Throttle", unit: "%", digits: 0, read: (s, p) => {
    const v = value(metric(s, "car.throttle_fraction", p));
    return v == null ? null : v * 100;
  } },
  { id: "brake", label: "Brake", unit: "%", digits: 0, read: (s, p) => {
    const v = value(metric(s, "car.brake_fraction", p));
    return v == null ? null : v * 100;
  } },
  { id: "brake_temp", label: "Brake temp", unit: "°C", digits: 0,
    read: (s, p) => maxOf(s, p, ["fl", "fr", "rl", "rr"].map((w) => "brake." + w + ".temp_c")) },
];

export function specOf(id: string): MetricSpec {
  return METRICS.find((m) => m.id === id) ?? METRICS[0];
}

/** Append one sample and drop only history older than the retention window. */
export function appendSample(series: Sample[], t: number, v: number | null): void {
  const last = series[series.length - 1];
  if (last && t < last.t) return;
  series.push({ t, v });
  const cutoff = t - RETENTION_MS;
  let drop = 0;
  while (drop < series.length && series[drop].t < cutoff) drop++;
  if (drop > 0) series.splice(0, drop);
}

export function windowSamples(series: Sample[], end: number, range: number): Sample[] {
  const start = end - range;
  return series.filter((s) => s.t >= start && s.t <= end);
}

export function latestSample(series: Sample[]): Sample | null {
  for (let i = series.length - 1; i >= 0; i--) {
    if (series[i].v != null) return series[i];
  }
  return null;
}

export function formatAgo(deltaMs: number): string {
  const seconds = Math.max(0, Math.round(deltaMs / 1000));
  if (seconds === 0) return "now";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return "-" + m + ":" + String(s).padStart(2, "0");
}

export function formatValue(v: number, digits: number): string {
  return v.toFixed(digits);
}

