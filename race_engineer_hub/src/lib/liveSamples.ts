import { useEffect, useRef, useState } from "react";
import { api, type Metric, type StreamMsg } from "@/lib/live";
import type { SamplePoint } from "@/lib/samples";

/** Channels the selected-car trace subscription needs. The main 24-car
 * dashboard subscription stays without throttle and brake. */
export const TRACE_CHANNELS = [
  "car.throttle_fraction", "car.brake_fraction", "car.gear",
  "energy.store_j", "energy.deploy_mode",
  "overtake.eligible", "boost.active",
  "timing.current_lap_num", "timing.current_lap_ms", "timing.lap_ms",
];

/** Points before a time; used to keep only the current lap. */
export function trimBefore(points: SamplePoint[], at: number): SamplePoint[] {
  let i = 0;
  while (i < points.length && points[i][0] < at) i++;
  return i === 0 ? points : points.slice(i);
}

function value(metrics: Map<string, Metric>, channel: string, car: number): number | string | boolean | null {
  const m = metrics.get(channel + "@" + car) ?? metrics.get(channel);
  if (!m || m.measurement.availability !== "available") return null;
  return m.measurement.value;
}

function numberAt(metrics: Map<string, Metric>, channel: string, car: number): number | null {
  const v = value(metrics, channel, car);
  return typeof v === "number" ? v : null;
}

/** Map the streamed deployment label back to the raw enum the band expects. */
const DEPLOY_MODES: Record<string, number> = { none: 0, medium: 1, hotlap: 2, overtake: 3, boost: 3 };

/** Build one tuple from the latest streamed metrics, or null without inputs. */
export function pointAt(metrics: Map<string, Metric>, car: number, anchorMs: number): SamplePoint | null {
  const lapMs = numberAt(metrics, "timing.current_lap_ms", car);
  const throttle = numberAt(metrics, "car.throttle_fraction", car);
  const brake = numberAt(metrics, "car.brake_fraction", car);
  const gear = numberAt(metrics, "car.gear", car);
  if (lapMs == null || throttle == null || brake == null || gear == null) return null;
  const mode = value(metrics, "energy.deploy_mode", car);
  const ok = value(metrics, "overtake.eligible", car);
  const on = value(metrics, "boost.active", car);
  return [
    Math.round(anchorMs + lapMs), throttle, brake, gear,
    numberAt(metrics, "energy.store_j", car),
    typeof mode === "string" ? DEPLOY_MODES[mode.toLowerCase()] ?? null : null,
    typeof ok === "boolean" ? ok : null,
    typeof on === "boolean" ? on : null,
  ];
}

type Stored = { car_index: number; laps: { start_ms: number; lap_ms: number; points: SamplePoint[] }[] };

/** Live current-lap samples for the selected cars, anchored to the stored
 * session clock so they join the recorded laps without a jump. */
export function useLiveSamples(sessionId: string | null, branchId: string | null, cars: number[], enabled: boolean) {
  const [points, setPoints] = useState<Map<number, SamplePoint[]>>(new Map());
  const anchor = useRef(0);
  const stored = useRef<Map<number, SamplePoint[]>>(new Map());
  const [epoch, setEpoch] = useState(0);
  const key = cars.join(",");

  // A flashback keeps the session id but starts a new branch. Drop the old
  // samples before the stored-lap fetch or stream for the new scope resolves.
  useEffect(() => {
    stored.current = new Map();
    anchor.current = 0;
    setPoints(new Map());
  }, [sessionId, branchId, key, enabled]);

  // Completed laps come from the catalogue, on mount and after each lap change.
  useEffect(() => {
    if (!enabled || !sessionId || cars.length === 0) return;
    let alive = true;
    api<{ cars: Stored[] }>(
      "/sessions/" + sessionId + "/telemetry?branch=" + encodeURIComponent(branchId ?? "") + "&cars=" + key,
    ).then((d) => {
      if (!alive) return;
      const map = new Map<number, SamplePoint[]>();
      let latestEnd = 0;
      for (const car of d.cars ?? []) {
        map.set(car.car_index, (car.laps ?? []).flatMap((l) => l.points).sort((a, b) => a[0] - b[0]));
        for (const l of car.laps ?? []) latestEnd = Math.max(latestEnd, l.start_ms + l.lap_ms);
      }
      stored.current = map;
      if (latestEnd > 0) anchor.current = Math.max(anchor.current, latestEnd);
      setPoints(new Map(map));
    }).catch(() => {});
    return () => { alive = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, branchId, key, enabled, epoch]);

  useEffect(() => {
    if (!enabled || cars.length === 0) return;
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(proto + "://" + location.host + "/api/v1/stream");
    const lapSeen = new Map<number, number>();
    const buffer = new Map(stored.current);
    let finishedLap = false;
    ws.onopen = () => ws.send(JSON.stringify({
      type: "subscribe", schema_version: "1.0.0", request_id: "pit-traces", source_id: "live",
      car_indices: cars, channels: TRACE_CHANNELS, max_hz: 20, after_seq: null,
    }));
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data) as StreamMsg;
      if (!msg.metrics || msg.type === "resync_required") return;
      const metrics = new Map<string, Metric>();
      for (const m of msg.metrics) metrics.set(m.id, m);
      let changed = false;
      for (const car of cars) {
        const lap = numberAt(metrics, "timing.current_lap_num", car);
        const previous = lapSeen.get(car);
        if (lap != null) lapSeen.set(car, lap);
        if (lap != null && previous != null && lap > previous) {
          // The lap just ended: anchor the new one where the last sample was
          // and pull the completed lap from the catalogue.
          const last = buffer.get(car)?.at(-1);
          if (last) anchor.current = Math.max(anchor.current, last[0]);
          finishedLap = true;
        }
        const point = pointAt(metrics, car, anchor.current);
        if (!point) continue;
        buffer.set(car, trimBefore([...(buffer.get(car) ?? []), point], anchor.current));
        changed = true;
      }
      if (changed) setPoints(new Map(buffer));
      if (finishedLap) { finishedLap = false; setEpoch((n) => n + 1); }
    };
    ws.onerror = () => ws.close();
    return () => ws.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, branchId, key, enabled]);

  return { anchor: anchor.current, points };
}

