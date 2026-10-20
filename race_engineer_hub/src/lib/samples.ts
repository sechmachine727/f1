import { downsample } from "@/lib/downsample";

// Stored tuple: [session_time_ms, throttle, brake, gear, ers_j, deploy_mode,
// overtake_available, overtake_active]. Unavailable values are null.
export type SamplePoint = [
  number,
  number | null,
  number | null,
  number | null,
  number | null,
  number | null,
  boolean | null,
  boolean | null,
];

export type SampleLap = { lap: number; start_ms: number; lap_ms: number; points: SamplePoint[] };
export type SampleCar = { car_index: number; name: string; team_id: number; restricted: boolean; laps: SampleLap[] };
export type SessionTelemetry = {
  session_id: string;
  branch_id: string;
  regulation_season: number | null;
  cars: SampleCar[];
  laps_available: number;
  sample_method: string;
};

/** Flatten the stored laps into one time-ordered series. */
export function carPoints(car: SampleCar, laps?: number[]): SamplePoint[] {
  const wanted = laps && laps.length ? new Set(laps) : null;
  return car.laps
    .filter((l) => !wanted || wanted.has(l.lap))
    .flatMap((l) => l.points)
    .sort((a, b) => a[0] - b[0]);
}

export function rangeOf(points: SamplePoint[]): { min: number; max: number } {
  if (points.length === 0) return { min: 0, max: 0 };
  return { min: points[0][0], max: points[points.length - 1][0] };
}

/** Points for the chosen laps, timed from each lap's own start. */
export function lapRelativePoints(car: SampleCar, lapNumbers: number[]): { lap: number; points: SamplePoint[] }[] {
  return car.laps
    .filter((l) => lapNumbers.includes(l.lap))
    .sort((a, b) => a.lap - b.lap)
    .map((l) => ({ lap: l.lap, points: l.points.map((p) => [p[0] - l.start_ms, ...p.slice(1)] as SamplePoint) }));
}

/** Points inside the window, reduced to roughly the plot width. */
export function visiblePoints(points: SamplePoint[], start: number, end: number, target = 600): SamplePoint[] {
  const lo = Math.min(start, end), hi = Math.max(start, end);
  return downsample(points.filter((p) => p[0] >= lo && p[0] <= hi), lo, hi, target);
}

/** The sample nearest a time, used for the pointer readout. */
export function nearestPoint(points: SamplePoint[], at: number): SamplePoint | null {
  if (points.length === 0) return null;
  let lo = 0, hi = points.length - 1;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (points[mid][0] < at) lo = mid + 1;
    else hi = mid;
  }
  const after = points[lo];
  const before = points[Math.max(0, lo - 1)];
  return Math.abs(before[0] - at) <= Math.abs(after[0] - at) ? before : after;
}

