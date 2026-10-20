import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { CompareGrid } from "@/components/CompareGrid";
import type { Metric, Snapshot } from "@/lib/live";
import type { SamplePoint } from "@/lib/samples";

const metric = (channel: string, car: number, val: number | string | null, availability = "available"): Metric => ({
  id: channel + "@" + car, channel, car_index: car,
  measurement: { value: val, availability, unit: "", as_of_ms: 1, basis: "observed", source_id: "t", confidence: "high", reason: null },
});

const snapshot = (items: Metric[]): Snapshot => ({
  context: null, metrics: new Map(items.map((m) => [m.id, m])), events: [], alerts: [], status: null, connected: true, recording: "recording", seq: 1,
});

describe("live comparison grid", () => {
  it("states which car is faster and ahead", () => {
    render(<CompareGrid cars={[0, 1]} s={snapshot([
      metric("car.driver_name", 0, "LECLERC"), metric("car.driver_name", 1, "HAMILTON"),
      metric("car.speed_mps", 0, 100), metric("car.speed_mps", 1, 50),
      metric("timing.car_position", 0, 1), metric("timing.car_position", 1, 3),
    ])} />);
    expect(screen.getByText(/LECLERC higher by 180 km\/h/)).toBeInTheDocument();
    expect(screen.getByText(/LECLERC lower by P2/)).toBeInTheDocument();
  });

  it("suppresses the delta when a value is restricted", () => {
    render(<CompareGrid cars={[0, 1]} s={snapshot([
      metric("car.driver_name", 0, "LECLERC"), metric("car.driver_name", 1, "HAMILTON"),
      metric("tyre.fl.wear_pct", 0, 20), metric("tyre.fl.wear_pct", 1, null, "restricted"),
    ])} />);
    // Every row without both values keeps the explicit gap marker.
    expect(screen.getAllByText("needs both cars").length).toBeGreaterThan(0);
    expect(screen.queryByText(/higher by/)).not.toBeInTheDocument();
  });

  it("shows no number for a stale feed", () => {
    render(<CompareGrid cars={[0, 1]} s={snapshot([
      metric("car.driver_name", 0, "LECLERC"), metric("car.driver_name", 1, "HAMILTON"),
      metric("car.speed_mps", 0, null, "stale"), metric("car.speed_mps", 1, 50),
    ])} />);
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    expect(screen.queryByText(/higher by/)).not.toBeInTheDocument();
  });

  it("takes throttle and brake from the selected-car buffer", () => {
    const live = new Map<number, SamplePoint[]>([
      [0, [[1000, 0.85, 0.2, 6, 1, 2, null, null] as SamplePoint]],
      [1, [[1000, 0.88, 0, 6, 1, 2, null, null] as SamplePoint]],
    ]);
    render(<CompareGrid cars={[0, 1]} s={snapshot([metric("car.driver_name", 0, "LECLERC"), metric("car.driver_name", 1, "HAMILTON")])} live={live} />);
    expect(screen.getByText("85%")).toBeInTheDocument();
    expect(screen.getByText("20%")).toBeInTheDocument();
    expect(screen.getByText("3% apart")).toBeInTheDocument();
  });

  it("shows a single car without a delta column", () => {
    render(<CompareGrid cars={[0]} s={snapshot([metric("car.driver_name", 0, "LECLERC"), metric("car.speed_mps", 0, 50)])} />);
    expect(screen.getByText("LECLERC")).toBeInTheDocument();
    expect(screen.queryByText(/needs both cars/)).not.toBeInTheDocument();
    expect(screen.getByText("180")).toBeInTheDocument();
  });

  it("renders an empty selection with guidance", () => {
    render(<CompareGrid cars={[]} s={snapshot([])} />);
    expect(screen.getByText(/Select a car/)).toBeInTheDocument();
  });
});

