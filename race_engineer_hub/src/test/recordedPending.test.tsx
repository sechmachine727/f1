import { describe, expect, it, vi, afterEach } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { RecordedPitWall } from "@/components/RecordedPit";
import { clearSelection, select } from "@/lib/selection";

const analytics = {
  session_id: "s", session_type: "race", track_id: 3, regulation_season: 2026, started_at_ms: 1, ended_at_ms: 2,
  participants: [{ car_index: 0, name: "LECLERC", team_id: 1, number: 16, ai: false }],
  laps: [{
    car_index: 0, lap: 1, lap_ms: 90000, sector1_ms: 28000, sector2_ms: 31000, sector3_ms: 31000, distance_m: 5400,
    avg_speed_mps: 60, top_speed_mps: 80, trap_speed_mps: 0, position_start: 1, position_end: 1,
    gap_ahead_start_ms: 0, gap_leader_start_ms: 0, compound: "C3", tyre_age_laps: 1, fuel_start_kg: 100,
    fuel_end_kg: 98, ers_start_j: 1, ers_end_j: 1, pit_in: false, pit_out: false, valid: true, track_limits: false, complete: true,
  }],
  stints: [], best_lap_ms: 90000, theoretical_best_ms: 90000, clean_laps: 1, total_laps: 1, analytics_ready: true,
};

afterEach(() => { act(() => clearSelection()); vi.unstubAllGlobals(); });

describe("recorded traces without samples", () => {
  it("states that samples are being derived instead of drawing an empty axis", async () => {
    const original = globalThis.fetch;
    globalThis.fetch = vi.fn(async (url: unknown) => {
      const body = String(url).includes("/telemetry")
        ? { session_id: "s", branch_id: "b", regulation_season: 2026, laps_available: 0, sample_method: "lap-samples-v1",
            cars: [{ car_index: 0, name: "LECLERC", team_id: 1, restricted: false, laps: [] }] }
        : analytics;
      return { ok: true, json: async () => body } as Response;
    }) as unknown as typeof fetch;
    act(() => select({ sessionId: "s", branchId: "b", label: "Race", season: 2026, recorded: true }));
    try {
      render(<RecordedPitWall />);
      const tab = await screen.findByRole("tab", { name: "Inputs" });
      await act(async () => { tab.click(); });
      expect(await screen.findByText(/Samples are being derived for this recording/)).toBeInTheDocument();
    } finally {
      globalThis.fetch = original;
    }
  });
});

