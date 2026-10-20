import { describe, expect, it } from "vitest";
import { downsample } from "@/lib/downsample";
import { carPoints, lapRelativePoints, nearestPoint, rangeOf, visiblePoints, type SampleCar, type SamplePoint } from "@/lib/samples";

function ramp(n: number, from = 0, step = 100): SamplePoint[] {
  return Array.from({ length: n }, (_, i) => [from + i * step, 0.5, 0, 5, 1000000, 2, false, false] as SamplePoint);
}

describe("window downsampling", () => {
  it("bounds a long race and keeps both ends", () => {
    const points = ramp(5000);
    const out = downsample(points, 0, 499900, 600);
    expect(out.length).toBeLessThanOrEqual(600);
    expect(out[0]).toBe(points[0]);
    expect(out[out.length - 1]).toBe(points[points.length - 1]);
    for (let i = 1; i < out.length; i++) expect(out[i][0]).toBeGreaterThan(out[i - 1][0]);
  });

  it("keeps a one-sample brake spike", () => {
    const points = ramp(3000);
    points[1500] = [150000, 0.1, 1, 4, 900000, 2, false, false];
    const out = downsample(points, 0, 299900, 600);
    expect(out.some((p) => p[0] === 150000 && p[2] === 1)).toBe(true);
  });

  it("leaves a short range untouched", () => {
    const points = ramp(100);
    expect(downsample(points, 0, 9900, 600)).toBe(points);
  });

  it("filters to the visible window before reducing", () => {
    const points = ramp(1000);
    const out = visiblePoints(points, 10000, 20000, 600);
    expect(out[0][0]).toBe(10000);
    expect(out[out.length - 1][0]).toBe(20000);
  });
});

describe("sample series", () => {
  const car: SampleCar = {
    car_index: 0, name: "LECLERC", team_id: 1, restricted: false,
    laps: [
      { lap: 1, start_ms: 0, lap_ms: 90000, points: ramp(3, 0) },
      { lap: 2, start_ms: 90000, lap_ms: 90000, points: ramp(3, 90000) },
    ],
  };

  it("flattens laps in time order and filters the chosen laps", () => {
    expect(carPoints(car).map((p) => p[0])).toEqual([0, 100, 200, 90000, 90100, 90200]);
    expect(carPoints(car, [2]).map((p) => p[0])).toEqual([90000, 90100, 90200]);
  });

  it("reports the range and the nearest sample", () => {
    const points = carPoints(car);
    expect(rangeOf(points)).toEqual({ min: 0, max: 90200 });
    expect(nearestPoint(points, 140)?.[0]).toBe(100);
    expect(nearestPoint(points, 89990)?.[0]).toBe(90000);
    expect(nearestPoint([], 10)).toBeNull();
  });

  it("times compared laps from their own start without stretching", () => {
    const longer: SampleCar = {
      ...car,
      laps: [
        { lap: 1, start_ms: 90000, lap_ms: 90000, points: [[90000, 0.5, 0, 5, 1, 0, null, null], [179900, 0.5, 0, 5, 1, 0, null, null]] },
        { lap: 2, start_ms: 180000, lap_ms: 91000, points: [[180000, 0.5, 0, 5, 1, 0, null, null], [270900, 0.5, 0, 5, 1, 0, null, null]] },
      ],
    };
    const [one, two] = lapRelativePoints(longer, [1, 2]);
    expect(one.points.map((p) => p[0])).toEqual([0, 89900]);
    expect(two.points.map((p) => p[0])).toEqual([0, 90900]);
  });
});

