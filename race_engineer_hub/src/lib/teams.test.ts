import { describe, expect, it } from "vitest";
import { NEUTRAL_TEAM, contrastOnSurface, teamInk, teamTint } from "@/lib/teams";

describe("team identity colour", () => {
  it("uses the appendix colour for a known team", () => {
    expect(teamInk(1)).toBe("#e8002d");
    expect(teamInk(8)).toBe("#ff8000");
  });

  it("falls back to neutral for unknown, custom and missing teams", () => {
    expect(teamInk(104)).toBe(NEUTRAL_TEAM);
    expect(teamInk(null)).toBe(NEUTRAL_TEAM);
    expect(teamInk(undefined)).toBe(NEUTRAL_TEAM);
    expect(teamInk(9999)).toBe(NEUTRAL_TEAM);
  });

  it("keeps every identity ink readable on the dark surface", () => {
    for (let id = 0; id <= 9; id++) {
      expect(contrastOnSurface(teamInk(id))).toBeGreaterThanOrEqual(3);
    }
    expect(contrastOnSurface(teamInk(9999))).toBeGreaterThanOrEqual(3);
  });

  it("tints the header without borrowing a status colour", () => {
    expect(teamTint(1)).not.toBe(teamInk(1));
    expect(teamTint(9999)).toBe(teamTint(null));
  });
});

