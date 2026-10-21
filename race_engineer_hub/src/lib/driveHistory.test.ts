import { describe, expect, it } from "vitest";
import {
  DEFAULT_RANGE_MS, METRICS, RETENTION_MS, appendSample, formatAgo, latestSample, specOf, windowSamples,
  type Sample,
} from "@/lib/driveHistory";
import type { Metric, Snapshot } from "@/lib/live";

const m = (channel: string, car: number, value: number | null): Metric => ({
  id: channel + "@" + car, channel, car_index: car,
  measurement: { value, availability: value == null ? "not_received" : "available", unit: "", as_of_ms: 1, basis: "observed", source_id: "t", confidence: "high", reason: null },
});

const snap = (items: Metric[]): Snapshot => ({
  context: { session_id: "s", branch_id: "b", track_id: 3, regulation_season: 2025, mode_family: "time_trial", player_car_index: 0 },
  metrics: new Map(items.map((x) => [x.id, x])), events: [], alerts: [], status: null, connected: true, recording: "recording", seq: 1,
});

describe("drive history", () => {
  it("keeps ten minutes and trims only older samples", () => {
    const ring: Sample[] = [];
    appendSample(ring, 0, 1);
    appendSample(ring, RETENTION_MS, 2);
    expect(ring.map((s) => s.v)).toEqual([1, 2]);
    appendSample(ring, RETENTION_MS + 1000, 3);
    expect(ring.map((s) => s.v)).toEqual([2, 3]);
  });

  it("stores gaps and ignores out-of-order samples", () => {
    const ring: Sample[] = [];
    appendSample(ring, 1000, 5);
    appendSample(ring, 2000, null);
    appendSample(ring, 500, 9);
    expect(ring.map((s) => s.v)).toEqual([5, null]);
    expect(latestSample(ring)?.v).toBe(5);
  });

  it("changes the window without touching the retained series", () => {
    const ring: Sample[] = [];
    for (let i = 0; i <= 600; i++) appendSample(ring, i * 1000, i);
    const before = ring.length;
    expect(windowSamples(ring, 600000, 30000)[0].t).toBe(570000);
    expect(windowSamples(ring, 600000, DEFAULT_RANGE_MS)[0].t).toBe(480000);
    expect(windowSamples(ring, 600000, RETENTION_MS).length).toBe(before);
    expect(ring.length).toBe(before);
  });

  it("formats axis time labels", () => {
    expect(formatAgo(0)).toBe("now");
    expect(formatAgo(120000)).toBe("-2:00");
    expect(formatAgo(30000)).toBe("-0:30");
  });
});

describe("metric registry", () => {
  it("reads the derived metrics from one snapshot", () => {
    const s = snap([
      m("car.speed_mps", 0, 100),
      m("tyre.fl.wear_pct", 0, 10), m("tyre.fr.wear_pct", 0, 20), m("tyre.rl.wear_pct", 0, 30), m("tyre.rr.wear_pct", 0, 40),
      m("tyre.fl.surface_c", 0, 80), m("tyre.fr.surface_c", 0, 110),
      m("energy.store_j", 0, 3500000),
      m("car.throttle_fraction", 0, 0.5), m("car.brake_fraction", 0, 0.1),
    ]);
    expect(specOf("speed").read?.(s, 0)).toBe(360);
    expect(specOf("tyre_wear").read?.(s, 0)).toBe(40);
    expect(specOf("surface").read?.(s, 0)).toBe(110);
    expect(specOf("ers").read?.(s, 0)).toBe(3.5);
    expect(specOf("throttle").read?.(s, 0)).toBe(50);
    expect(specOf("brake").read?.(s, 0)).toBe(10);
    expect(METRICS.length).toBeGreaterThanOrEqual(13);
  });

  it("returns null when the channel is unavailable", () => {
    expect(specOf("fuel").read?.(snap([m("fuel.mass_kg", 0, null)]), 0)).toBeNull();
  });
});

