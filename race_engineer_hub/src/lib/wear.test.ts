import { describe, expect, it } from "vitest";
import { wearColor } from "@/lib/wear";

describe("tyre wear color", () => {
  it("moves continuously from green at zero to red at one hundred", () => {
    expect(wearColor(0)).toBe("hsl(140.0 68% 52%)");
    expect(wearColor(50)).toBe("hsl(70.0 68% 52%)");
    expect(wearColor(100)).toBe("hsl(0.0 68% 52%)");
    expect(wearColor(null)).toBe("#6b7280");
    expect(wearColor(-10)).toBe(wearColor(0));
    expect(wearColor(140)).toBe(wearColor(100));
  });
});
