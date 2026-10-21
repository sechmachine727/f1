import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { DriveGraph } from "@/components/DriveGraph";
import { WearRing } from "@/components/Drive";
import { specOf, type Sample } from "@/lib/driveHistory";
import { wearColor } from "@/lib/wear";

const now = 1_000_000;
const series: Sample[] = [
  { t: now - 90_000, v: 3.9 },
  { t: now - 60_000, v: 3.7 },
  { t: now - 30_000, v: null },
  { t: now - 10_000, v: 3.4 },
];

describe("DriveGraph", () => {
  it("labels both axes and shows the latest value", () => {
    render(<DriveGraph index={0} spec={specOf("ers")} series={series} rangeMs={120000} color="#fff" onMetric={() => {}} onRange={() => {}} />);
    expect(screen.getAllByText("MJ").length).toBeGreaterThan(0);
    expect(screen.getByText("time")).toBeInTheDocument();
    expect(screen.getByText("-2:00")).toBeInTheDocument();
    expect(screen.getByText("-1:00")).toBeInTheDocument();
    expect(screen.getByText("now")).toBeInTheDocument();
    expect(screen.getByText("3.40")).toBeInTheDocument();
  });

  it("changes metric and range through labelled selectors", () => {
    const onMetric = vi.fn();
    const onRange = vi.fn();
    render(<DriveGraph index={1} spec={specOf("ers")} series={series} rangeMs={120000} color="#fff" onMetric={onMetric} onRange={onRange} />);
    fireEvent.change(screen.getByLabelText("Graph 2 metric"), { target: { value: "fuel" } });
    fireEvent.change(screen.getByLabelText("Graph 2 range"), { target: { value: "30000" } });
    expect(onMetric).toHaveBeenCalledWith("fuel");
    expect(onRange).toHaveBeenCalledWith(30000);
  });

  it("breaks the line at a gap", () => {
    const { container } = render(<DriveGraph index={0} spec={specOf("ers")} series={series} rangeMs={120000} color="#fff" onMetric={() => {}} onRange={() => {}} />);
    expect(container.querySelectorAll("polyline").length).toBe(2);
  });

  it("states when no samples are available", () => {
    render(<DriveGraph index={0} spec={specOf("fuel")} series={[]} rangeMs={120000} color="#fff" onMetric={() => {}} onRange={() => {}} />);
    expect(screen.getByText("Collecting…")).toBeInTheDocument();
  });

  it("colors tyre wear from green to red across the graph", () => {
    const { container } = render(<DriveGraph index={1} spec={specOf("tyre_wear")} series={[
      { t: now - 90_000, v: 0 }, { t: now - 60_000, v: 50 }, { t: now - 30_000, v: 100 },
    ]} rangeMs={120000} color="#fff" onMetric={() => {}} onRange={() => {}} />);
    const stops = [...container.querySelectorAll("stop")].map((s) => s.getAttribute("stop-color"));
    expect(stops).toEqual(["hsl(140.0 68% 52%)", "hsl(70.0 68% 52%)", "hsl(0.0 68% 52%)"]);
    expect(container.querySelector("polyline")?.getAttribute("stroke")).toContain("url(#wear-gradient-1)");
  });

  it("repaints the wear ring when the percentage changes", () => {
    const { container, rerender } = render(<WearRing pct={10} />);
    const ring = () => container.querySelectorAll("circle")[1];
    expect(ring()?.getAttribute("stroke")).toBe(wearColor(10));
    rerender(<WearRing pct={90} />);
    expect(ring()?.getAttribute("stroke")).toBe(wearColor(90));
  });
});

