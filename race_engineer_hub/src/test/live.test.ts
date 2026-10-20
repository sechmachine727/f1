import { describe, expect, it } from "vitest";
import { activeCars, applyMessage, availabilityLabel, connect, frozenSnapshot, metric, text, type Metric, type Snapshot } from "@/lib/live";

const m = (id: string, channel: string, value: number | string | null, availability = "available"): Metric => ({
  id, channel, car_index: 0,
  measurement: { value, availability, unit: "m/s", as_of_ms: 1, basis: "observed", source_id: "t", confidence: "high", reason: null },
});

const base: Snapshot = { context: null, metrics: new Map(), events: [], alerts: [], status: null, connected: true, recording: "idle", seq: 4 };

describe("stream store", () => {

  it("opens exactly one socket no matter how many panels subscribe", () => {
    const sockets: unknown[] = [];
    class FakeSocket {
      onopen: (() => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((ev: { data: string }) => void) | null = null;
      constructor(public url: string) { sockets.push(this); }
      send() {}
      close() {}
    }
    const original = globalThis.WebSocket;
    // jsdom has no WebSocket implementation; the store only constructs it.
    (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeSocket;
    try {
      connect();
      connect();
      connect();
      expect(sockets).toHaveLength(1);
      expect((sockets[0] as FakeSocket).url).toContain("/api/v1/stream");
    } finally {
      (globalThis as unknown as { WebSocket: unknown }).WebSocket = original;
    }
  });

  it("snapshot replaces the whole metric set", () => {
    const prev = { ...base, metrics: new Map([["a", m("a", "car.speed_mps", 1)]]) };
    const { next, resync } = applyMessage(prev, {
      type: "snapshot", seq: "5", base_seq: null, context: null,
      metrics: [m("b", "car.throttle_fraction", 0.5)], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(resync).toBe(false);
    expect([...next.metrics.keys()]).toEqual(["b"]);
    expect(next.seq).toBe(5);
  });

  it("delta merges by id and keeps untouched metrics", () => {
    const prev = { ...base, metrics: new Map([["a", m("a", "car.speed_mps", 1)], ["b", m("b", "car.gear", 7)]]) };
    const { next } = applyMessage(prev, {
      type: "delta", seq: "5", base_seq: "4", context: null,
      metrics: [m("a", "car.speed_mps", 2)], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(next.metrics.get("a")?.measurement.value).toBe(2);
    expect(next.metrics.get("b")?.measurement.value).toBe(7);
  });

  it("explicit removals delete metrics", () => {
    const prev = { ...base, metrics: new Map([["a", m("a", "car.speed_mps", 1)]]) };
    const { next } = applyMessage(prev, {
      type: "delta", seq: "5", base_seq: "4", context: null,
      metrics: [], removed_metric_ids: ["a"], events: [], alerts: [], status: null,
    });
    expect(next.metrics.size).toBe(0);
  });

  it("sequence mismatch requests a resync instead of merging", () => {
    const prev = { ...base, metrics: new Map([["a", m("a", "car.speed_mps", 1)]]) };
    const { next, resync } = applyMessage(prev, {
      type: "delta", seq: "9", base_seq: "7", context: null,
      metrics: [m("a", "car.speed_mps", 99)], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(resync).toBe(true);
    expect(next.metrics.get("a")?.measurement.value).toBe(1);
  });

  it("resync_required never mutates state", () => {
    const { next, resync } = applyMessage(base, {
      type: "resync_required", seq: "1", base_seq: null, context: null,
      metrics: [], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(resync).toBe(true);
    expect(next).toBe(base);
  });

  it("stale values render as a dash with a state word, never the raw number", () => {
    const stale = m("a", "car.speed_mps", null, "stale").measurement;
    expect(text(stale)).toBe("—");
    expect(availabilityLabel(stale)).toBe("No update");
    expect(text(m("a", "car.speed_mps", 0).measurement)).toBe("0");
    expect(metric(base, "car.speed_mps")).toBeNull();
  });

  it("lists only cars that reported an identity, plus the player", () => {
    const snap: Snapshot = {
      ...base,
      context: { session_id: "s", branch_id: "b", track_id: 3, regulation_season: 2026, mode_family: "time_trial", player_car_index: 0 },
      metrics: new Map([
        ["car.team_id@0", m("car.team_id@0", "car.team_id", 1)],
        ["car.team_id@1", m("car.team_id@1", "car.team_id", null, "not_received")],
        ["car.driver_name@1", m("car.driver_name@1", "car.driver_name", null, "not_received")],
        ["car.driver_name@2", m("car.driver_name@2", "car.driver_name", "HAMILTON")],
      ]),
    };
    expect(activeCars(snap)).toEqual([0, 2]);
  });

  it("freezes the last available value when the feed stops", () => {
    const ctx = { session_id: "freeze", branch_id: "b", track_id: 3, regulation_season: 2026, mode_family: "time_trial", player_car_index: 0 };
    const { next: live } = applyMessage(base, {
      type: "snapshot", seq: "5", base_seq: null, context: ctx,
      metrics: [m("car.speed_mps@0", "car.speed_mps", 55)], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    const { next: stopped } = applyMessage(live, {
      type: "delta", seq: "6", base_seq: "5", context: ctx,
      metrics: [m("car.speed_mps@0", "car.speed_mps", null, "stale")], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(frozenSnapshot(stopped).metrics.get("car.speed_mps@0")?.measurement.value).toBe(55);
    // A new session must not inherit the previous freeze.
    const { next: other } = applyMessage(stopped, {
      type: "snapshot", seq: "7", base_seq: null, context: { ...ctx, session_id: "other" },
      metrics: [m("car.speed_mps@0", "car.speed_mps", null, "stale")], removed_metric_ids: [], events: [], alerts: [], status: null,
    });
    expect(frozenSnapshot(other).metrics.get("car.speed_mps@0")?.measurement.availability).toBe("stale");
  });
});
