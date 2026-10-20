import { useEffect, useState } from "react";
import { api } from "@/lib/live";
import { getCircuitInfo } from "@/data/circuitData";
import type { SessionTelemetry } from "@/lib/samples";

export type RecordedLap = {
  car_index: number; lap: number; lap_ms: number; sector1_ms: number; sector2_ms: number; sector3_ms: number;
  distance_m: number; avg_speed_mps: number; top_speed_mps: number; trap_speed_mps: number;
  position_start: number; position_end: number; gap_ahead_start_ms: number; gap_leader_start_ms: number;
  compound: string; tyre_age_laps: number; fuel_start_kg: number; fuel_end_kg: number; ers_start_j: number; ers_end_j: number;
  pit_in: boolean; pit_out: boolean; valid: boolean; track_limits: boolean; complete: boolean;
  extra_json?: string; corner_speeds?: Record<string, { availability: string; value: number | null }>;
};

export type RecordedParticipant = { car_index: number; name: string; team_id: number; number: number; ai: boolean };
export type RecordedStint = { car_index: number; kind: string; index: number; start_lap: number; end_lap: number; compound: string; laps: number; best_ms: number; median_ms: number; degradation_ms_per_lap: number; fuel_start_kg: number; fuel_end_kg: number; ers_start_j: number; ers_end_j: number };
export type SessionAnalytics = {
  session_id: string; session_type: string; session_type_code: number | null; weekend_link_id?: string | null; track_id: number | null; regulation_season: number | null;
  started_at_ms: number; ended_at_ms: number | null; participants: RecordedParticipant[]; laps: RecordedLap[]; stints: RecordedStint[];
  best_lap_ms: number; theoretical_best_ms: number; clean_laps: number; total_laps: number; analytics_ready: boolean;
};

const cache = new Map<string, SessionAnalytics>();

export function useSessionAnalytics(sessionId: string | null) {
  const [data, setData] = useState<SessionAnalytics | null>(sessionId ? cache.get(sessionId) ?? null : null);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!sessionId) { setData(null); return; }
    let alive = true;
    api<SessionAnalytics>(`/sessions/${sessionId}/analytics`)
      .then((d) => { if (!alive) return; cache.set(sessionId, d); setData(d); setError(""); })
      .catch((e) => { if (alive) setError(String(e)); });
    return () => { alive = false; };
  }, [sessionId]);
  return { data, error };
}

export const driversOf = (s: SessionAnalytics | null, car: number) =>
  s?.laps.filter((l) => l.car_index === car && l.complete && l.lap_ms > 0) ?? [];

export const sessionLabel: Record<string, string> = {
  race: "Race", sprint: "Sprint", practice: "Practice", time_trial: "Time trial",
  qualifying: "Qualifying", qualifying_q1: "Qualifying Q1", qualifying_q2: "Qualifying Q2", qualifying_q3: "Qualifying Q3", unknown: "Unknown session",
};

export const fmtLap = (ms: number | null | undefined) => ms && ms > 0 ? (ms / 1000).toFixed(3) + " s" : "—";
export const fmtGap = (ms: number) => ms === 0 ? "—" : (ms > 0 ? "+" : "") + (ms / 1000).toFixed(3);

/** Circuit name, date and duration for a History row. */
export function sessionMeta(s: { track_id: number | null; started_at_ms: number; ended_at_ms?: number | null }): {
  circuit: string;
  date: string;
  duration: string;
} {
  const circuit = s.track_id == null
    ? "Track unknown"
    : getCircuitInfo(s.track_id)?.name ?? "Track " + s.track_id + " (name unknown)";
  const date = s.started_at_ms > 0
    ? new Date(s.started_at_ms).toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" })
    : "date unknown";
  const duration = s.ended_at_ms && s.ended_at_ms > s.started_at_ms
    ? Math.max(1, Math.round((s.ended_at_ms - s.started_at_ms) / 60000)) + " min"
    : "in progress";
  return { circuit, date, duration };
}

/** Stored per-lap input samples for the selected cars and lap range. */
export function useSessionTelemetry(sessionId: string | null, branchId: string | null, cars: number[], fromLap?: number, toLap?: number) {
  const [data, setData] = useState<SessionTelemetry | null>(null);
  const [error, setError] = useState("");
  const key = cars.join(",");
  useEffect(() => {
    if (!sessionId || cars.length === 0) { setData(null); setError(""); return; }
    let alive = true;
    const q = new URLSearchParams({ branch: branchId ?? "", cars: key });
    if (fromLap) q.set("from_lap", String(fromLap));
    if (toLap) q.set("to_lap", String(toLap));
    api<SessionTelemetry>("/sessions/" + sessionId + "/telemetry?" + q)
      .then((d) => { if (alive) { setData(d); setError(""); } })
      .catch((e) => { if (alive) { setData(null); setError(String(e)); } });
    return () => { alive = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, branchId, key, fromLap, toLap]);
  return { data, error };
}
