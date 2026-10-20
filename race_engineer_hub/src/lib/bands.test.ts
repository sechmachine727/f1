import { describe, expect, it } from "vitest";
import { bandLegend, deployLabel, deploymentBands, overtakeBands } from "@/lib/bands";
import type { SamplePoint } from "@/lib/samples";

const p = (t: number, mode: number | null, ok: boolean | null = null, on: boolean | null = null): SamplePoint =>
  [t, 0.5, 0, 5, 1000000, mode, ok, on];

describe("deployment label", () => {
  it("names mode 3 by the regulation season", () => {
    expect(deployLabel(3, 2025)).toBe("Overtake");
    expect(deployLabel(3, 2026)).toBe("Boost");
    expect(deployLabel(3, null)).toBe("Mode 3 unclassified");
    expect(deployLabel(2, 2025)).toBe("Hotlap");
  });
});

describe("deployment band", () => {
  it("splits a 2025 lap into labelled segments", () => {
    const points = [p(0, 0), p(1000, 1), p(2000, 2), p(3000, 3), p(4000, 0)];
    const bands = deploymentBands(points, 2025, 0, 5000);
    expect(bands.map((b) => b.label)).toEqual(["None", "Medium", "Hotlap", "Overtake", "None"]);
    expect(bands[3].start).toBe(3000);
    expect(bands[3].end).toBe(4000);
    expect(bandLegend(bands).map((l) => l.label)).toEqual(["None", "Medium", "Hotlap", "Overtake"]);
  });

  it("calls the 2026 push mode boost", () => {
    const bands = deploymentBands([p(0, 3), p(1000, 0)], 2026, 0, 2000);
    expect(bands[0].label).toBe("Boost");
  });

  it("keeps an unknown season neutral and labelled", () => {
    const bands = deploymentBands([p(0, 3)], null, 0, 1000);
    expect(bands[0].label).toBe("Mode 3 unclassified");
  });

  it("merges adjacent samples with the same mode", () => {
    const bands = deploymentBands([p(0, 2), p(100, 2), p(200, 2)], 2025, 0, 300);
    expect(bands).toHaveLength(1);
    expect(bands[0].end).toBe(300);
  });
});

describe("overtake band", () => {
  it("separates unavailable, available and active in 2026", () => {
    const points = [p(0, 0, false, false), p(1000, 0, true, false), p(2000, 0, true, true)];
    const bands = overtakeBands(points, 2026, 0, 3000);
    expect(bands.map((b) => b.label)).toEqual(["Overtake unavailable", "Overtake available", "Overtake active"]);
  });

  it("is absent outside 2026", () => {
    expect(overtakeBands([p(0, 0, true, true)], 2025, 0, 1000)).toEqual([]);
    expect(overtakeBands([p(0, 0, true, true)], null, 0, 1000)).toEqual([]);
  });
});

