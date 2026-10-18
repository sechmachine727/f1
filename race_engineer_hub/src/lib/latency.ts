// Local observation-to-paint probe. The server stamps the last accepted packet
// and the send time; the browser adds its clock offset and the paint delay.
export type LatencyStats = { n: number; p50: number; p95: number; max: number };
const samples: number[] = [];
let offset = 0;

export async function syncClock() {
  try {
    const t0 = Date.now();
    const res = await fetch("/api/v1/time");
    const body = await res.json();
    const t1 = Date.now();
    offset = Number(body.now_ms) - (t0 + t1) / 2;
  } catch {
    offset = 0;
  }
}

export function stats(): LatencyStats {
  if (samples.length === 0) return { n: 0, p50: 0, p95: 0, max: 0 };
  const sorted = [...samples].sort((a, b) => a - b);
  const at = (q: number) => sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * q))];
  return { n: sorted.length, p50: at(0.5), p95: at(0.95), max: sorted[sorted.length - 1] };
}

export function recordRender(observedAtMs?: number) {
  if (!observedAtMs) return;
  requestAnimationFrame(() => {
    samples.push(Date.now() + offset - observedAtMs);
    if (samples.length > 4096) samples.splice(0, 1024);
  });
}

declare global {
  interface Window { __f1latency?: () => LatencyStats }
}
if (typeof window !== "undefined") window.__f1latency = stats;
