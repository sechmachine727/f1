// Team identity colours from the F1 25 appendix team ids. Identity colour marks
// who a car is; the blue/green/orange/red status colours keep their own meaning.
export const SURFACE = "#0b0d11";
export const NEUTRAL_TEAM = "#8b93a1";

const TEAM_COLOURS: Record<number, string> = {
  0: "#27f4d2", // Mercedes
  1: "#e8002d", // Ferrari
  2: "#3671c6", // Red Bull Racing
  3: "#64c4ff", // Williams
  4: "#229971", // Aston Martin
  5: "#ff87bc", // Alpine
  6: "#6692ff", // RB
  7: "#b6babd", // Haas
  8: "#ff8000", // McLaren
  9: "#52e252", // Sauber
};

function channel(v: number) {
  const c = v / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function luminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

export function contrastOnSurface(hex: string): number {
  return (luminance(hex) + 0.05) / (luminance(SURFACE) + 0.05);
}

function mix(hex: string, towards: number, amount: number): string {
  const n = parseInt(hex.slice(1), 16);
  const ch = (v: number) => Math.round(v + (towards - v) * amount);
  const r = ch((n >> 16) & 255), g = ch((n >> 8) & 255), b = ch(n & 255);
  return "#" + [r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("");
}

/** The team colour, lightened only as far as it needs to be readable. */
export function teamInk(teamId: number | null | undefined): string {
  const base = teamId == null ? NEUTRAL_TEAM : TEAM_COLOURS[teamId] ?? NEUTRAL_TEAM;
  let ink = base;
  for (let i = 0; i < 8 && contrastOnSurface(ink) < 3; i++) ink = mix(ink, 255, 0.18);
  return ink;
}

/** A neutral wash for the car header, so text keeps normal contrast. */
export function teamTint(teamId: number | null | undefined): string {
  return mix(teamInk(teamId), 11, 0.82);
}

