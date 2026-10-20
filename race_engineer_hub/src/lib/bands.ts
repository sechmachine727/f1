import type { SamplePoint } from "@/lib/samples";

export type Band = { start: number; end: number; label: string; color: string };

export const DEPLOY_COLOURS = {
  none: "#3a414d",
  medium: "#5c7cfa",
  hotlap: "#b58cff",
  push: "#ff5fa2",
  available: "#38d6c4",
  active: "#16a99a",
  neutral: "#4b5563",
};

/** Mode 3 is overtake in 2025 and boost in the 2026 pack. */
export function deployLabel(mode: number, season: number | null): string {
  if (mode === 0) return "None";
  if (mode === 1) return "Medium";
  if (mode === 2) return "Hotlap";
  if (mode === 3) {
    if (season === 2025) return "Overtake";
    if (season === 2026) return "Boost";
    return "Mode 3 unclassified";
  }
  return "Unknown mode";
}

function deployColour(mode: number, season: number | null): string {
  if (mode === 0) return DEPLOY_COLOURS.none;
  if (mode === 1) return DEPLOY_COLOURS.medium;
  if (mode === 2) return DEPLOY_COLOURS.hotlap;
  if (mode === 3) return season === 2025 || season === 2026 ? DEPLOY_COLOURS.push : DEPLOY_COLOURS.neutral;
  return DEPLOY_COLOURS.neutral;
}

function segment(points: SamplePoint[], start: number, end: number, value: (p: SamplePoint) => number | null, labels: (v: number) => string, colours: (v: number) => string): Band[] {
  const bands: Band[] = [];
  const visible = points.filter((p) => p[0] >= start && p[0] <= end);
  for (let i = 0; i < visible.length; i++) {
    const mode = value(visible[i]);
    if (mode == null) continue;
    const next = i + 1 < visible.length ? visible[i + 1][0] : end;
    const last = bands[bands.length - 1];
    if (last && last.label === labels(mode) && last.end >= visible[i][0] - 1) {
      last.end = Math.min(next, end);
    } else {
      bands.push({ start: visible[i][0], end: Math.min(next, end), label: labels(mode), color: colours(mode) });
    }
  }
  return bands;
}

/** Deployment-mode band for the session's regulation. */
export function deploymentBands(points: SamplePoint[], season: number | null, start: number, end: number): Band[] {
  return segment(points, start, end, (p) => p[5], (m) => deployLabel(m, season), (m) => deployColour(m, season));
}

/**
 * 2026 overtake availability. Only 2026 sessions carry this state; other
 * seasons return no bands instead of borrowing the deployment meaning.
 */
export function overtakeBands(points: SamplePoint[], season: number | null, start: number, end: number): Band[] {
  if (season !== 2026) return [];
  return segment(
    points, start, end,
    (p) => (p[6] == null ? null : p[7] ? 2 : p[6] ? 1 : 0),
    (v) => (v === 2 ? "Overtake active" : v === 1 ? "Overtake available" : "Overtake unavailable"),
    (v) => (v === 2 ? DEPLOY_COLOURS.active : v === 1 ? DEPLOY_COLOURS.available : DEPLOY_COLOURS.neutral),
  );
}

/** Legend entries present in the window, deduplicated by label. */
export function bandLegend(bands: Band[]): { label: string; color: string }[] {
  const seen = new Map<string, string>();
  for (const b of bands) if (!seen.has(b.label)) seen.set(b.label, b.color);
  return [...seen.entries()].map(([label, color]) => ({ label, color }));
}

