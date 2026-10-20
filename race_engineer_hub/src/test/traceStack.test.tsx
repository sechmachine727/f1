import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TraceStack, type TraceSeries } from "@/components/TraceStack";
import type { SamplePoint } from "@/lib/samples";

const point = (t: number, mode: number | null, ok: boolean | null = null, on: boolean | null = null): SamplePoint =>
  [t, 0.7, 0, 6, 1500000, mode, ok, on];

function series(until = 10000, step = 1000): TraceSeries {
  const points: SamplePoint[] = [];
  for (let t = 0; t <= until; t += step) points.push(point(t, 0));
  return { key: "c0", name: "LECLERC", color: "#e8002d", points };
}

const endReadout = (text: string) => screen.getByText(new RegExp("– " + text.replace(".", "\\.")));

describe("TraceStack", () => {
  it("labels the 2025 deployment band", () => {
    const s = series(4000);
    s.points = [point(0, 0), point(1000, 1), point(2000, 2), point(3000, 3), point(4000, 0)];
    render(<TraceStack series={[s]} season={2025} />);
    expect(screen.getByText("Medium")).toBeInTheDocument();
    expect(screen.getByText("Hotlap")).toBeInTheDocument();
    expect(screen.getByText("Overtake")).toBeInTheDocument();
    expect(screen.queryByText("Boost")).not.toBeInTheDocument();
  });

  it("shows boost and the separate 2026 overtake strip", () => {
    const s = series(2000);
    s.points = [point(0, 3, true, false), point(1000, 3, true, true), point(2000, 0, false, false)];
    render(<TraceStack series={[s]} season={2026} />);
    expect(screen.getByText("Boost")).toBeInTheDocument();
    expect(screen.getByText("Overtake available")).toBeInTheDocument();
    expect(screen.getByText("Overtake active")).toBeInTheDocument();
  });

  it("keeps an unknown season neutral and labelled", () => {
    const s = series(1000);
    s.points = [point(0, 3), point(1000, 3)];
    render(<TraceStack series={[s]} season={null} />);
    expect(screen.getByText("Mode 3 unclassified")).toBeInTheDocument();
    expect(screen.queryByText("Overtake available")).not.toBeInTheDocument();
  });

  it("starts at 100 percent and zooms about the window", () => {
    render(<TraceStack series={[series()]} season={2025} />);
    expect(screen.getByText("100%")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Zoom in"));
    expect(screen.getByText("150%")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Reset"));
    expect(screen.getByText("100%")).toBeInTheDocument();
  });

  it("freezes a panned window and resumes following the latest sample", () => {
    const { rerender } = render(<TraceStack series={[series(1000, 100)]} season={2025} live />);
    expect(endReadout("0:01.000")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Zoom in"));
    // Zooming at the centre leaves the window behind the latest sample.
    const followButton = screen.getByText("Follow latest");
    expect(followButton).not.toBeDisabled();
    rerender(<TraceStack series={[series(5000, 100)]} season={2025} live />);
    expect(endReadout("0:00.833")).toBeInTheDocument();
    fireEvent.click(followButton);
    expect(endReadout("0:05.000")).toBeInTheDocument();
    expect(screen.getByText("Follow latest")).toBeDisabled();
  });

  it("pans with the keyboard", () => {
    render(<TraceStack series={[series()]} season={2025} />);
    fireEvent.click(screen.getByLabelText("Zoom in"));
    const before = screen.getByRole("group", { name: /traces/i }).textContent;
    fireEvent.keyDown(screen.getByRole("group", { name: /traces/i }), { key: "ArrowLeft" });
    const after = screen.getByRole("group", { name: /traces/i }).textContent;
    expect(after).not.toBe(before);
  });

  it("states when a window has no samples", () => {
    render(<TraceStack series={[{ key: "c0", name: "A", color: "#fff", points: [] }]} season={2025} empty="No completed laps yet." />);
    expect(screen.getByText("No completed laps yet.")).toBeInTheDocument();
  });
});

