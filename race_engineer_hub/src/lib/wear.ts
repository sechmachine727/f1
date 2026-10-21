/** Continuous tyre-wear scale: 0% green, 50% yellow, 100% red. */
export function wearColor(pct: number | null): string {
  if (pct == null || !Number.isFinite(pct)) return "#6b7280";
  const v = Math.max(0, Math.min(100, pct));
  const hue = 140 * (1 - v / 100);
  return "hsl(" + hue.toFixed(1) + " 68% 52%)";
}
