import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { completedLaps, paceRows, useLiveLaps, type LiveLap } from "@/lib/liveLaps";

const lap = (n: number, ms: number | null, valid = true): LiveLap => ({
  id: "l" + n, car_index: 0, lap: n, valid, complete: true,
  lap_time_ms: { availability: ms == null ? "not_received" : "available", value: ms },
});

describe("live laps", () => {
  it("keeps only completed valid laps for the car", () => {
    const laps = [lap(1, 90000), lap(2, null), lap(3, 91000, false), { ...lap(4, 92000), car_index: 1 }];
    expect(completedLaps(laps, 0).map((l) => l.lap)).toEqual([1]);
  });

  it("adds the in-progress lap and replaces it when catalogued", () => {
    const rows = paceRows([lap(1, 90000)], 0, 2, 45000, true);
    expect(rows).toEqual([{ lap: 1, ms: 90000, prov: null }, { lap: 2, ms: null, prov: 45000 }]);
    const after = paceRows([lap(1, 90000), lap(2, 90500)], 0, 2, 91000, true);
    expect(after).toEqual([{ lap: 1, ms: 90000, prov: null }, { lap: 2, ms: 90500, prov: null }]);
  });

  it("has no provisional point for a recorded view", () => {
    expect(paceRows([lap(1, 90000)], 0, 2, 45000, false)).toEqual([{ lap: 1, ms: 90000, prov: null }]);
  });

  it("polls every two seconds while live and stays idle for recorded sessions", async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn(async () => ({ ok: true, json: async () => ({ items: [lap(1, 90000)] }) }) as Response);
    vi.stubGlobal("fetch", fetchMock);
    try {
      const { result } = renderHook(() => useLiveLaps("s", "b", true));
      await act(async () => { await vi.advanceTimersByTimeAsync(0); });
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(result.current.laps).toHaveLength(1);
      await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
      expect(fetchMock).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
      vi.unstubAllGlobals();
    }

    vi.useFakeTimers();
    const idle = vi.fn();
    vi.stubGlobal("fetch", idle);
    try {
      renderHook(() => useLiveLaps("s", "b", false));
      await act(async () => { await vi.advanceTimersByTimeAsync(4000); });
      expect(idle).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
      vi.unstubAllGlobals();
    }
  });
});

