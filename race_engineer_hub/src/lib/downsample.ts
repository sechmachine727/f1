import type { SamplePoint } from "@/lib/samples";

/**
 * Reduce a time window to roughly the plot width while keeping the first and
 * last point and the strongest throttle and brake of each bucket. A single
 * braking spike therefore survives the full-race view.
 */
export function downsample(points: SamplePoint[], start: number, end: number, target = 600): SamplePoint[] {
  if (points.length <= target) return points;
  const span = Math.max(1, end - start);
  const buckets = Math.max(1, Math.floor(target / 4));
  const width = span / buckets;
  const out: SamplePoint[] = [];
  let current = -1;
  let first: SamplePoint | null = null, last: SamplePoint | null = null;
  let maxThrottle: SamplePoint | null = null, maxBrake: SamplePoint | null = null;
  const flush = () => {
    if (current < 0) return;
    const seen = new Set<SamplePoint>();
    for (const p of [first, last, maxThrottle, maxBrake]) {
      if (p && !seen.has(p)) {
        seen.add(p);
        out.push(p);
      }
    }
  };
  for (const p of points) {
    const bucket = Math.floor((p[0] - start) / width);
    if (bucket !== current) {
      flush();
      current = bucket;
      first = last = maxThrottle = maxBrake = null;
    }
    last = p;
    if (!first) first = p;
    if (typeof p[1] === "number" && (maxThrottle == null || p[1] > (maxThrottle[1] ?? -1))) maxThrottle = p;
    if (typeof p[2] === "number" && (maxBrake == null || p[2] > (maxBrake[2] ?? -1))) maxBrake = p;
  }
  flush();
  return out.sort((a, b) => a[0] - b[0]);
}

