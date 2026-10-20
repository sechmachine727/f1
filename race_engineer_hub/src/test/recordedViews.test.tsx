import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { act } from "react";
import { afterEach, beforeEach, vi } from "vitest";
import { RecordedPitWall } from "@/components/RecordedPit";
import { RecordedAnalysis } from "@/components/RecordedAnalysis";
import { clearSelection, select } from "@/lib/selection";

const lap = (car: number, n: number) => ({
  car_index: car, lap: n, lap_ms: 90000 + n, sector1_ms: 28000, sector2_ms: 31000, sector3_ms: 31000,
  distance_m: 5400, avg_speed_mps: 60, top_speed_mps: 80, trap_speed_mps: 0,
  position_start: 1, position_end: 1, gap_ahead_start_ms: 0, gap_leader_start_ms: 0,
  compound: "C3", tyre_age_laps: n, fuel_start_kg: 100, fuel_end_kg: 98, ers_start_j: 4000000, ers_end_j: 3000000,
  pit_in: false, pit_out: false, valid: true, track_limits: false, complete: true, corner_speeds: {},
});

const analytics = {
  session_id: "s", session_type: "race", track_id: 3, regulation_season: 2026,
  started_at_ms: 1, ended_at_ms: 2,
  participants: [
    { car_index: 0, name: "LECLERC", team_id: 1, number: 16, ai: false },
    { car_index: 1, name: "HAMILTON", team_id: 0, number: 44, ai: false },
  ],
  laps: [lap(0, 1), lap(0, 2), lap(1, 1), lap(1, 2)],
  stints: [], best_lap_ms: 90001, theoretical_best_ms: 89900, clean_laps: 4, total_laps: 4, analytics_ready: true,
};

const telemetry = {
  session_id: "s", branch_id: "b", regulation_season: 2026, laps_available: 4, sample_method: "lap-samples-v1",
  cars: [0, 1].map((car) => ({
    car_index: car, name: car === 0 ? "LECLERC" : "HAMILTON", team_id: car === 0 ? 1 : 0, restricted: false,
    laps: [1, 2].map((n) => ({
      lap: n, start_ms: n * 90000, lap_ms: 90000,
      points: [[n * 90000, 0.6, 0, 6, 3000000, 2, true, false], [n * 90000 + 1000, 0.7, 0, 7, 2900000, 2, true, true]],
    })),
  })),
};

const calls: string[] = [];
let sockets = 0;

function stub() {
  calls.length = 0;
  sockets = 0;
  const original = globalThis.fetch;
  const originalWs = globalThis.WebSocket;
  globalThis.fetch = vi.fn(async (url: unknown) => {
    const u = String(url);
    calls.push(u);
    const body = u.includes("/analytics") ? analytics : u.includes("/telemetry") ? telemetry : analytics;
    return { ok: true, json: async () => body } as Response;
  }) as unknown as typeof fetch;
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = class {
    constructor() { sockets++; }
    send() {}
    close() {}
  };
  return () => {
    globalThis.fetch = original;
    (globalThis as unknown as { WebSocket: unknown }).WebSocket = originalWs;
  };
}

beforeEach(() => {
  act(() => select({ sessionId: "s", branchId: "b", label: "Race", season: 2026, recorded: true }));
});
afterEach(() => {
  act(() => clearSelection());
});

describe("recorded telemetry", () => {
  it("loads stored samples for the selected cars and lap range", async () => {
    const restore = stub();
    try {
      render(<RecordedPitWall />);
      expect((await screen.findAllByText("HAMILTON")).length).toBeGreaterThan(0);
      await act(async () => { screen.getAllByText("HAMILTON")[0].click(); });
      await act(async () => { screen.getByRole("tab", { name: "Inputs" }).click(); });
      expect(await screen.findByText("Throttle, brake, gear and energy over session time")).toBeInTheDocument();
      await act(async () => { fireEvent.change(screen.getByLabelText("Laps from"), { target: { value: "2" } }); });
      await act(async () => { fireEvent.change(screen.getByLabelText("to"), { target: { value: "2" } }); });
      await waitFor(() => expect(calls.filter((u) => u.includes("telemetry")).length).toBeGreaterThan(0));
      const ranged = calls.filter((u) => u.includes("telemetry")).join(" ");
      expect(ranged).toContain("cars=0%2C1");
      expect(ranged).toContain("from_lap=2");
      expect(ranged).toContain("to_lap=2");
      expect(screen.getByText(/Throttle, brake, gear and energy/)).toBeInTheDocument();
    } finally { restore(); }
  });

  it("reads only the recorded route and never opens the live stream", async () => {
    const restore = stub();
    try {
      render(<RecordedPitWall />);
      expect((await screen.findAllByText("LECLERC")).length).toBeGreaterThan(0);
      render(<RecordedAnalysis />);
      await act(async () => {});
      expect(sockets).toBe(0);
      expect(calls.some((u) => u.includes("/sessions/s/telemetry"))).toBe(false);
      expect(calls.every((u) => u.includes("/api/v1/sessions/"))).toBe(true);
    } finally { restore(); }
  });
});

