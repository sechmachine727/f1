import { describe, expect, it } from "vitest";
import { UI_CHANNELS, type Metric } from "@/lib/live";
import { TRACE_CHANNELS, pointAt, trimBefore } from "@/lib/liveSamples";
import type { SamplePoint } from "@/lib/samples";

const metric = (channel: string, car: number, value: number | string | boolean, availability = "available"): Metric => ({
  id: channel + "@" + car, channel, car_index: car,
  measurement: { value, availability, unit: "", as_of_ms: 1, basis: "observed", source_id: "t", confidence: "high", reason: null },
});

const points: SamplePoint[] = [
  [0, 0.2, 0, 3, 100, 0, false, false],
  [1000, 0.4, 0, 4, 90, 0, false, false],
  [2000, 0.6, 0, 5, 80, 0, false, false],
];

describe("selected-car live samples", () => {
  it("trims the buffer at the current lap boundary", () => {
    expect(trimBefore(points, 1500).map((p) => p[0])).toEqual([2000]);
    expect(trimBefore(points, 0)).toBe(points);
    expect(trimBefore([], 10)).toEqual([]);
  });

  it("builds a tuple from streamed metrics, anchored to the session clock", () => {
    const metrics = new Map<string, Metric>([
      ["timing.current_lap_ms@1", metric("timing.current_lap_ms", 1, 2500)],
      ["car.throttle_fraction@1", metric("car.throttle_fraction", 1, 0.75)],
      ["car.brake_fraction@1", metric("car.brake_fraction", 1, 0)],
      ["car.gear@1", metric("car.gear", 1, 6)],
      ["energy.store_j@1", metric("energy.store_j", 1, 1200000)],
      ["energy.deploy_mode@1", metric("energy.deploy_mode", 1, "Boost")],
      ["overtake.eligible@1", metric("overtake.eligible", 1, false)],
    ]);
    expect(pointAt(metrics, 1, 90000)).toEqual([92500, 0.75, 0, 6, 1200000, 3, false, null]);
  });

  it("returns nothing when the high-rate inputs are missing", () => {
    expect(pointAt(new Map(), 0, 0)).toBeNull();
  });

  it("does not change the main 24-car subscription", () => {
    expect(UI_CHANNELS).not.toContain("car.throttle_fraction");
    expect(UI_CHANNELS).not.toContain("car.brake_fraction");
    expect(TRACE_CHANNELS).toContain("car.throttle_fraction");
    expect(TRACE_CHANNELS).toContain("car.brake_fraction");
  });
});

