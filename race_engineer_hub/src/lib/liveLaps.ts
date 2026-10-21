import { useEffect, useState } from "react";
import { api } from "@/lib/live";

export type LiveLap = {
  id: string;
  car_index: number;
  lap: number;
  lap_time_ms?: { availability: string; value: number | null };
  valid: boolean;
  complete: boolean;
};

export type PaceRow = { lap: number; ms: number | null; prov: number | null };

/** Completed, valid laps for one car. */
export function completedLaps(laps: LiveLap[], car: number): LiveLap[] {
  return laps.filter((l) =>
    l.car_index === car && l.valid && l.complete &&
    l.lap_time_ms?.availability === "available" && typeof l.lap_time_ms.value === "number",
  );
}

/** Completed lap points plus the in-progress lap, keyed by lap number. */
export function paceRows(laps: LiveLap[], car: number, currentLap: number | null, currentMs: number | null, live: boolean): PaceRow[] {
  const rows: PaceRow[] = completedLaps(laps, car).map((l) => ({ lap: l.lap, ms: l.lap_time_ms!.value as number, prov: null }));
  if (live && currentLap != null && currentLap > 0 && currentMs != null && !rows.some((r) => r.lap === currentLap)) {
    rows.push({ lap: currentLap, ms: null, prov: currentMs });
  }
  return rows.sort((a, b) => a.lap - b.lap);
}

/** Poll catalogued laps for the active branch while live. */
export function useLiveLaps(sessionId: string | null, branchId: string | null, enabled: boolean) {
  const [state, setState] = useState<{ sessionId: string | null; laps: LiveLap[] }>({ sessionId: null, laps: [] });
  useEffect(() => {
    if (!enabled || !sessionId) {
      setState({ sessionId: null, laps: [] });
      return;
    }
    // Drop the previous session's rows before the first fetch resolves.
    setState({ sessionId, laps: [] });
    let alive = true;
    const load = () =>
      api<{ items?: LiveLap[] }>("/sessions/" + sessionId + "/laps?branch=" + encodeURIComponent(branchId ?? ""))
        .then((d) => { if (alive) setState({ sessionId, laps: d.items ?? [] }); })
        .catch(() => null);
    load();
    const id = setInterval(load, 2000);
    return () => { alive = false; clearInterval(id); };
  }, [sessionId, branchId, enabled]);
  return { laps: state.laps, sessionId: state.sessionId };
}

