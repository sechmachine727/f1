import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { DriveGraph } from "@/components/DriveGraph";
import { specOf, type Sample } from "@/lib/driveHistory";

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
});

