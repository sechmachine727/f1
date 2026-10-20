// Window arithmetic for the shared time-axis charts. Times are session
// milliseconds. 100 percent means the complete selected range.
export type Window = { start: number; end: number };

/** The narrowest window a zoom or pan may produce. */
export const MIN_SPAN_MS = 200;

export function fullWindow(min: number, max: number): Window {
  return { start: min, end: Math.max(min, max) };
}

export function span(w: Window): number {
  return w.end - w.start;
}

export function clampWindow(w: Window, min: number, max: number): Window {
  const total = Math.max(0, max - min);
  if (total === 0) return { start: min, end: min };
  let width = Math.min(Math.max(w.end - w.start, Math.min(MIN_SPAN_MS, total)), total);
  let start = Math.min(Math.max(w.start, min), max - width);
  if (start < min) {
    start = min;
    width = total;
  }
  return { start, end: start + width };
}

/** Zoom about a time; factor above one narrows the window. */
export function zoomAt(w: Window, at: number, factor: number, min: number, max: number): Window {
  const total = Math.max(0, max - min);
  if (total === 0 || factor <= 0) return fullWindow(min, max);
  const width = span(w);
  const next = Math.min(total, Math.max(Math.min(MIN_SPAN_MS, total), width / factor));
  const anchor = Math.min(Math.max(at, w.start), w.end);
  const ratio = width === 0 ? 0.5 : (anchor - w.start) / width;
  return clampWindow({ start: anchor - ratio * next, end: anchor + (1 - ratio) * next }, min, max);
}

export function panBy(w: Window, deltaMs: number, min: number, max: number): Window {
  return clampWindow({ start: w.start + deltaMs, end: w.end + deltaMs }, min, max);
}

export function isAtEnd(w: Window, max: number, tolerance = 1): boolean {
  return w.end >= max - tolerance;
}

export function zoomPercent(w: Window, min: number, max: number): number {
  const total = max - min;
  if (total <= 0) return 100;
  return Math.round((total / Math.max(span(w), 1)) * 100);
}

/** Session time as m:ss.mmm for the range readout and tooltips. */
export function formatSessionTime(ms: number): string {
  const clamped = Math.max(0, ms);
  const minutes = Math.floor(clamped / 60000);
  const seconds = (clamped % 60000) / 1000;
  return minutes + ":" + seconds.toFixed(3).padStart(6, "0");
}

