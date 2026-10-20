import { describe, expect, it } from "vitest";
import {
  MIN_SPAN_MS, clampWindow, formatSessionTime, fullWindow, isAtEnd, panBy, span, zoomAt, zoomPercent,
} from "@/lib/chartWindow";

const full = fullWindow(0, 600000);

describe("chart window", () => {
  it("starts at the full range and reports 100 percent", () => {
    expect(full).toEqual({ start: 0, end: 600000 });
    expect(zoomPercent(full, 0, 600000)).toBe(100);
  });

  it("zooms about the pointer and keeps that time in place", () => {
    const zoomed = zoomAt(full, 300000, 2, 0, 600000);
    expect(span(zoomed)).toBe(300000);
    expect(zoomed.start).toBe(150000);
    expect(zoomed.end).toBe(450000);
    expect(zoomPercent(zoomed, 0, 600000)).toBe(200);
  });

  it("clamps at the ends and never leaves the data", () => {
    expect(zoomAt(full, 0, 4, 0, 600000)).toEqual({ start: 0, end: 150000 });
    expect(zoomAt(full, 600000, 4, 0, 600000)).toEqual({ start: 450000, end: 600000 });
    expect(panBy({ start: 0, end: 1000 }, -5000, 0, 600000)).toEqual({ start: 0, end: 1000 });
    expect(panBy({ start: 0, end: 1000 }, 600000, 0, 600000)).toEqual({ start: 599000, end: 600000 });
  });

  it("honours the minimum span", () => {
    const deep = zoomAt({ start: 0, end: MIN_SPAN_MS }, 100, 100, 0, 600000);
    expect(span(deep)).toBe(MIN_SPAN_MS);
  });

  it("handles a zero-length range", () => {
    expect(fullWindow(5, 5)).toEqual({ start: 5, end: 5 });
    expect(zoomAt({ start: 5, end: 5 }, 5, 2, 5, 5)).toEqual({ start: 5, end: 5 });
    expect(zoomPercent({ start: 5, end: 5 }, 5, 5)).toBe(100);
    expect(clampWindow({ start: 1, end: 9 }, 5, 5)).toEqual({ start: 5, end: 5 });
  });

  it("detects when the window sits at the latest sample", () => {
    expect(isAtEnd(full, 600000)).toBe(true);
    expect(isAtEnd({ start: 1000, end: 5000 }, 600000)).toBe(false);
    expect(isAtEnd({ start: 1000, end: 599999 }, 600000)).toBe(true);
  });

  it("formats session time", () => {
    expect(formatSessionTime(0)).toBe("0:00.000");
    expect(formatSessionTime(91234)).toBe("1:31.234");
    expect(formatSessionTime(-5)).toBe("0:00.000");
  });
});

