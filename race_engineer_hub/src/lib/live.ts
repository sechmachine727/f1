import { useEffect, useState } from "react";
import { recordRender, syncClock } from "@/lib/latency";
export type Measurement = { value: number | string | boolean | null; availability: string; unit: string; as_of_ms: number | null; basis: string; source_id: string; confidence: string; reason: string | null };
export type Metric = { id: string; car_index: number | null; channel: string; measurement: Measurement };
export type Context = { session_id: string; branch_id: string; track_id: number | null; regulation_season: number | null; mode_family: string; player_car_index: number | null };
export type Event = { id: string; type: string; severity: string; summary: string };
export type Alert = { id: string; severity: string; summary: string; acknowledged: boolean; state: string };
export type Snapshot = { context: Context | null; metrics: Map<string, Metric>; events: Event[]; alerts: Alert[]; status: string | null; connected: boolean; recording: string; seq: number };
const initial: Snapshot = { context: null, metrics: new Map(), events: [], alerts: [], status: null, connected: false, recording: "idle", seq: 0 };
// Only channels the workspaces render. Subscribing to every canonical channel
// pushed throttle/steer/brake samples for all 24 cars into the browser.
export const UI_CHANNELS = [
  "car.speed_mps", "car.gear", "car.rpm", "car.drs", "car.engine_temp_c", "car.team_id", "car.driver_name",
  "car.wing_front_left_damage_pct", "car.wing_front_right_damage_pct", "car.wing_rear_damage_pct",
  "car.floor_damage_pct", "car.diffuser_damage_pct", "car.sidepod_damage_pct", "car.gearbox_damage_pct",
  "car.engine_damage_pct", "car.drs_fault_pct", "car.ers_fault_pct",
  "tyre.compound", "tyre.age_laps", "fuel.mass_kg", "fuel.capacity_kg", "fuel.finish_margin_laps",
  "energy.store_j", "energy.deploy_mode", "energy.harvest_limit_j_per_lap", "overtake.eligible",
  "timing.lap_ms", "timing.current_lap_num", "timing.current_lap_ms", "timing.lap_distance_m",
  "timing.car_position", "timing.pit_stops", "timing.penalties_s", "timing.warnings",
  "timing.gap_ahead_s", "timing.gap_leader_s",
  "session.weather", "session.air_temp_c", "session.track_temp_c", "session.track_length_m",
  "session.total_laps", "session.safety_car_status",
  "session.paused",
  ...["fl", "fr", "rl", "rr"].flatMap((w) => [
    "tyre." + w + ".surface_c", "tyre." + w + ".core_c", "tyre." + w + ".pressure_pa",
    "tyre." + w + ".wear_pct", "tyre." + w + ".damage_pct", "tyre." + w + ".blisters_pct",
    "brake." + w + ".temp_c", "brake." + w + ".damage_pct",
  ]),
];
let state = initial;
let pending = state;
// Last values seen as available. Drive uses these when the feed stops so a
// pause freezes the display instead of blanking it; other views stay strict.
const lastGood = new Map<string, Metric>();
let flushTimer: ReturnType<typeof setTimeout> | null = null;
let pendingObserved = 0;
const listeners = new Set<() => void>();
let socket: WebSocket | null = null;
function emit() { listeners.forEach((l) => l()); }

// Data arrives at 60Hz, but committing React at that rate repaints the whole
// dashboard 60 times a second for numbers a driver reads at a glance. Coalesce
// to 10Hz; the latest applied state is what renders, so no sample is skipped.
function scheduleFlush() {
  if (flushTimer !== null) return;
  flushTimer = setTimeout(() => {
    flushTimer = null;
    state = pending;
    emit();
    recordRender(pendingObserved);
  }, 100);
}

export type StreamMsg = {
  type: string; seq: string; base_seq: string | null; context: Context | null;
  metrics: Metric[]; removed_metric_ids: string[]; events: Event[]; alerts: Alert[] | null; status: string | null; observed_at_ms?: number;
};

// applyMessage is the whole stream contract: snapshot replaces, delta merges by
// metric id, explicit removals delete, and a base_seq mismatch asks for a resync.
export function applyMessage(prev: Snapshot, msg: StreamMsg): { next: Snapshot; resync: boolean } {
  if (msg.type === "resync_required") return { next: prev, resync: true };
  const applied = Number(prev.seq);
  if (msg.type === "delta" && msg.base_seq !== null && Number(msg.base_seq) !== applied) {
    return { next: prev, resync: true };
  }
  // Deltas update the live map in place: the store keeps no old snapshots, and
  // copying ~1000 entries per message was the largest allocation in the browser.
  const sessionChanged = msg.context?.session_id && prev.context?.session_id && msg.context.session_id !== prev.context.session_id;
  if (sessionChanged || msg.type === "snapshot") lastGood.clear();
  const metrics = msg.type === "snapshot" ? new Map<string, Metric>() : prev.metrics;
  for (const m of msg.metrics ?? []) {
    metrics.set(m.id, m);
    if (m.measurement.availability === "available") lastGood.set(m.id, m);
  }
  for (const id of msg.removed_metric_ids ?? []) { metrics.delete(id); lastGood.delete(id); }
  const events = [...prev.events, ...(msg.events ?? [])].slice(-40);
  return {
    next: {
      metrics, events,
      context: msg.context ?? prev.context,
      alerts: msg.alerts && msg.alerts.length ? msg.alerts : prev.alerts,
      status: msg.status ?? prev.status,
      recording: msg.status ?? prev.recording,
      seq: Number(msg.seq),
      connected: prev.connected,
    },
    resync: false,
  };
}

/** Overlay the last available values so a stopped feed freezes Drive at its
 * final live state. Unavailable channels with no history stay unavailable. */
export function frozenSnapshot(s: Snapshot): Snapshot {
  if (lastGood.size === 0) return s;
  let changed = false;
  const metrics = new Map(s.metrics);
  for (const [id, good] of lastGood) {
    const current = metrics.get(id);
    if (!current || current.measurement.availability === "available") continue;
    metrics.set(id, good);
    changed = true;
  }
  return changed ? { ...s, metrics } : s;
}
function set(patch: Partial<Snapshot>) { state = { ...state, ...patch }; pending = state; emit(); }
export function connect() {
  if (socket) return;
  void syncClock();
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(proto + "://" + location.host + "/api/v1/stream");
  socket = ws;
  ws.onopen = () => {
    set({ connected: true });
    ws.send(JSON.stringify({ type: "subscribe", schema_version: "1.0.0", request_id: "ui", source_id: "live", car_indices: Array.from({ length: 24 }, (_, i) => i), channels: UI_CHANNELS, max_hz: 20, after_seq: null }));
  };
  ws.onclose = () => { socket = null; set({ connected: false }); setTimeout(connect, 1500); };
  ws.onerror = () => ws.close();
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data) as StreamMsg;
    const { next, resync } = applyMessage(pending, msg);
    if (resync) {
      ws.send(JSON.stringify({ type: "subscribe", schema_version: "1.0.0", request_id: "resync", source_id: "live", car_indices: Array.from({ length: 24 }, (_, i) => i), channels: UI_CHANNELS, max_hz: 20, after_seq: null }));
      return;
    }
    pending = next;
    pendingObserved = msg.observed_at_ms ?? pendingObserved;
    scheduleFlush();
  };
}
export function useLive(): Snapshot {
  const [, force] = useState(0);
  useEffect(() => { const l = () => force((n) => n + 1); listeners.add(l); connect(); return () => { listeners.delete(l); }; }, []);
  return state;
}
export function metric(s: Snapshot, channel: string, car?: number | null): Measurement | null {
  const idx = car ?? s.context?.player_car_index ?? 0;
  return s.metrics.get(channel + "@" + idx)?.measurement ?? s.metrics.get(channel)?.measurement ?? null;
}
export function value(m: Measurement | null): number | null { return m && m.availability === "available" && typeof m.value === "number" ? m.value : null; }
/** Player speed is the liveness signal; auxiliary status channels may lag. */
export function isFeedStale(s: Snapshot, player: number): boolean {
  return metric(s, "car.speed_mps", player)?.availability === "stale";
}
export function text(m: Measurement | null): string {
  if (!m || m.availability !== "available") return "—";
  return String(m.value);
}
export function availabilityLabel(m: Measurement | null): string {
  if (!m) return "No data";
  switch (m.availability) {
    case "available": return "Live";
    case "stale": return "No update";
    case "restricted": return "Restricted";
    case "not_received": return "No data";
    case "unsupported": return "Unsupported";
    default: return "Invalid";
  }
}

/** Cars with a reported identity, plus the player. Empty slots are excluded. */
export function activeCars(s: Snapshot): number[] {
  const player = s.context?.player_car_index ?? 0;
  const seen: number[] = [];
  for (let i = 0; i < 24; i++) {
    const team = value(metric(s, "car.team_id", i));
    const name = text(metric(s, "car.driver_name", i));
    if (team != null || (name !== "—" && name !== "") || i === player) seen.push(i);
  }
  return seen;
}
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch("/api/v1" + path, { ...init, headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) } });
  } catch {
    throw new Error("Cannot reach the server.");
  }
  if (res.status === 401) {
    window.dispatchEvent(new Event("f1tele:auth-required"));
    throw new Error("Authentication required.");
  }
  if (!res.ok) throw new Error((await res.json().catch(() => ({ message: res.statusText }))).message);
  return res.json();
}
